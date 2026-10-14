package ingest

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/worker"
	"github.com/google/uuid"
)

// claimInterval is how often the coordinator polls for newly-due work.
// Short enough that claimed capacity doesn't sit idle, long enough not to
// hammer Postgres with an empty-result poll.
const claimInterval = 5 * time.Second

// CoordinatorConfig configures the ingestion coordinator.
type CoordinatorConfig struct {
	// Token returns the current server crawling credential. A func, not a
	// string, so a refreshed GitHub App installation token takes effect
	// without restarting this process.
	Token func() string
	// Settings supplies live operational values.
	Settings func() *settings.Snapshot
}

// Coordinator owns the Initial/Freshness and Deep Diver worker pools and
// the periodic claim loop that feeds them. No priority queue, no activity
// score — FIFO on next_retry_at via ClaimJobs (plan §7, D1/D9).
type Coordinator struct {
	pool   *db.Pool
	worker *Worker
	live   func() *settings.Snapshot

	initialPool *worker.Pool
	deepPool    *worker.Pool

	owner string

	// lastEligibilityCheck throttles the whole-table eligibility recompute,
	// which must not run on every 5s claim cycle.
	lastEligibilityCheck time.Time
	lastMetricsRefresh   time.Time

	// dbSizePaused and lastDBSizeMB back Status()/the D6 disk guardrail —
	// pg_database_size() is cheap (metadata lookup, not a scan), so it's
	// checked every cycle rather than throttled like eligibility. Atomic
	// because Status() is read from the HTTP goroutine.
	dbSizePaused atomic.Bool
	lastDBSizeMB atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCoordinator builds a Coordinator. ghClient performs the actual GraphQL
// calls; cfg.Token and cfg.Settings are read fresh on every cycle.
func NewCoordinator(pool *db.Pool, ghClient *github.Client, cfg CoordinatorConfig) *Coordinator {
	if cfg.Settings == nil {
		fallback := settings.Defaults()
		cfg.Settings = func() *settings.Snapshot { return fallback }
	}
	ctx, cancel := context.WithCancel(context.Background())

	initialCount := cfg.Settings().Int(settings.IngestRoutineWorkers)
	deepCount := cfg.Settings().Int(settings.IngestPriorityWorkers)

	return &Coordinator{
		pool:        pool,
		worker:      NewWorker(pool, ghClient, cfg.Token, cfg.Settings),
		live:        cfg.Settings,
		initialPool: worker.NewPool("ingest-initial", initialCount, initialCount*8),
		deepPool:    worker.NewPool("ingest-deep", deepCount, deepCount*8),
		owner:       "ingestd-" + uuid.NewString()[:8],
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ApplySettings pushes current concurrency limits into the pools. Safe to
// call on every settings reload.
func (c *Coordinator) ApplySettings(_ *settings.Snapshot) {
	s := c.live()
	c.initialPool.SetConcurrency(s.Int(settings.IngestRoutineWorkers))
	c.deepPool.SetConcurrency(s.Int(settings.IngestPriorityWorkers))
}

// Start boots the claim loop in a background goroutine.
func (c *Coordinator) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		slog.Info("ingestion coordinator started", "owner", c.owner)

		ticker := time.NewTicker(claimInterval)
		defer ticker.Stop()

		// Reclaim leases left behind by a previous crashed process before
		// claiming anything new.
		if n, err := ReclaimExpiredLeases(c.ctx, c.pool); err != nil {
			slog.Warn("failed to reclaim expired leases", "error", err)
		} else if n > 0 {
			slog.Info("reclaimed expired leases", "count", n)
		}

		for {
			select {
			case <-c.ctx.Done():
				slog.Info("ingestion coordinator stopped")
				return
			case <-ticker.C:
				c.cycle()
			}
		}
	}()
}

func (c *Coordinator) cycle() {
	snap := c.live()
	batch := snap.Int(settings.IngestEnqueueBatchSize)

	// 1. Eligibility first, on its own cadence: a threshold change decides
	//    what the next two steps are even allowed to look at.
	c.recheckEligibility(snap)
	c.refreshMetrics(snap)
	publishPoolStats(c.initialPool)
	publishPoolStats(c.deepPool)

	// D6 disk guardrail: once the database itself has grown past the
	// configured ceiling, stop enqueuing and claiming entirely — new work
	// only adds rows, and existing "running" jobs still get to finish their
	// current page rather than being killed mid-write.
	if !c.checkDiskGuard(snap) {
		return
	}

	// 2. Repositories that have never been ingested.
	if n, err := EnqueueInitialJobs(c.ctx, c.pool, batch); err != nil {
		metrics.IngestCycleErrors.WithLabelValues("enqueue_initial").Inc()
		slog.Warn("failed to enqueue initial jobs", "error", err)
	} else if n > 0 {
		slog.Info("enqueued initial jobs for newly eligible repositories", "count", n)
	}

	// 3. Repositories whose first page is stale — the Freshness half of the
	//    cycle. Without this the catalog stops moving once seeded.
	freshnessAfter := snap.Seconds(settings.IngestFreshnessIntervalSeconds)
	if n, err := EnqueueFreshnessJobs(c.ctx, c.pool, batch, freshnessAfter); err != nil {
		metrics.IngestCycleErrors.WithLabelValues("enqueue_freshness").Inc()
		slog.Warn("failed to enqueue freshness jobs", "error", err)
	} else if n > 0 {
		slog.Info("enqueued freshness re-checks", "count", n, "older_than", freshnessAfter)
	}

	c.claimAndSubmit(JobInitial, c.initialPool)
	c.claimAndSubmit(JobDeep, c.deepPool)
}

// recheckEligibility re-tests stored repository metadata against the current
// thresholds, no more often than ingest.eligibility.recheck_seconds. It is a
// whole-table UPDATE, so it deliberately does not run on every 5s cycle.
func (c *Coordinator) recheckEligibility(snap *settings.Snapshot) {
	interval := snap.Seconds(settings.IngestEligibilityRecheckSeconds)
	if time.Since(c.lastEligibilityCheck) < interval {
		return
	}
	c.lastEligibilityCheck = time.Now()

	minStars := snap.Int(settings.IngestEligibilityMinStars)
	pushedWithinDays := snap.Int(settings.IngestEligibilityPushedWithinDays)
	n, err := RecomputeEligibility(c.ctx, c.pool, minStars, pushedWithinDays)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("eligibility").Inc()
		slog.Warn("failed to recompute repository eligibility", "error", err)
		return
	}
	if n > 0 {
		slog.Info("repository eligibility recomputed",
			"changed", n, "min_stars", minStars, "pushed_within_days", pushedWithinDays)
	}
}

// refreshMetrics re-reads the Postgres-backed /metrics gauges no more often
// than ingest.metrics_refresh_seconds.
func (c *Coordinator) refreshMetrics(snap *settings.Snapshot) {
	if time.Since(c.lastMetricsRefresh) < snap.Seconds(settings.IngestMetricsRefreshSeconds) {
		return
	}
	c.lastMetricsRefresh = time.Now()
	if err := RefreshQueueMetrics(c.ctx, c.pool); err != nil {
		metrics.IngestCycleErrors.WithLabelValues("metrics_refresh").Inc()
		slog.Warn("failed to refresh ingest metrics", "error", err)
	}
}

// checkDiskGuard returns true if the coordinator should proceed with
// enqueuing/claiming this cycle, false if ingest.max_db_size_mb is exceeded
// and new work must pause. Logs loudly on each transition, not every cycle,
// so a sustained pause doesn't spam the log at claimInterval.
func (c *Coordinator) checkDiskGuard(snap *settings.Snapshot) bool {
	maxMB := snap.Int(settings.IngestMaxDBSizeMB)

	sizeMB, err := CurrentDBSizeMB(c.ctx, c.pool)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("db_size").Inc()
		slog.Warn("failed to check database size for disk guardrail, proceeding", "error", err)
		return true
	}
	c.lastDBSizeMB.Store(sizeMB)
	metrics.IngestDBSizeBytes.Set(float64(sizeMB) * 1024 * 1024)

	exceeded := maxMB > 0 && sizeMB > int64(maxMB)
	wasPaused := c.dbSizePaused.Load()
	if exceeded && !wasPaused {
		slog.Warn("database size exceeds ingest.max_db_size_mb — pausing new ingestion work",
			"db_size_mb", sizeMB, "max_db_size_mb", maxMB)
	} else if !exceeded && wasPaused {
		slog.Info("database size back under ingest.max_db_size_mb — resuming ingestion work",
			"db_size_mb", sizeMB, "max_db_size_mb", maxMB)
	}
	c.dbSizePaused.Store(exceeded)
	paused := 0.0
	if exceeded {
		paused = 1
	}
	metrics.IngestPaused.Set(paused)
	return !exceeded
}

func (c *Coordinator) claimAndSubmit(jobType JobType, pool *worker.Pool) {
	stats := pool.Stats()
	capacity := stats.QueueCapacity - stats.QueueLength
	if capacity <= 0 {
		return
	}
	jobs, err := ClaimJobs(c.ctx, c.pool, jobType, capacity, c.owner)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("claim").Inc()
		slog.Warn("failed to claim jobs", "job_type", jobType, "error", err)
		return
	}
	for _, job := range jobs {
		j := job
		if err := pool.Submit(func(workerCtx context.Context) error {
			if err := c.worker.ProcessJob(workerCtx, j); err != nil {
				slog.Warn("job processing failed", "repo", j.FullName, "job_type", j.JobType, "error", err)
			}
			return nil
		}); err != nil {
			// Pool is full right after we just checked capacity (a bit of
			// slop from concurrent submissions is fine) — release the job
			// rather than leaving it claimed and unprocessed.
			if err := ReleaseJob(c.ctx, c.pool, j, 5*time.Second); err != nil {
				metrics.IngestCycleErrors.WithLabelValues("release").Inc()
				slog.Warn("failed to release unsubmitted job; its lease will expire", "repo", j.FullName, "error", err)
			}
		}
	}
}

// Status returns coordinator operational telemetry.
func (c *Coordinator) Status() map[string]any {
	return map[string]any{
		"initial_pool":   c.initialPool.Stats(),
		"deep_pool":      c.deepPool.Stats(),
		"db_size_mb":     c.lastDBSizeMB.Load(),
		"db_size_paused": c.dbSizePaused.Load(),
	}
}

// Shutdown stops the claim loop and drains both worker pools.
func (c *Coordinator) Shutdown(timeout time.Duration) error {
	c.cancel()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		_ = c.initialPool.Shutdown(timeout)
		_ = c.deepPool.Shutdown(timeout)
		close(done)
	}()

	select {
	case <-done:
		slog.Info("ingestion coordinator shutdown complete")
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
