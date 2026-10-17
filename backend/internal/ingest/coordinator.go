package ingest

import (
	"context"
	"errors"
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

// CoordinatorConfig configures the ingestion coordinator.
type CoordinatorConfig struct {
	// Token returns the current server crawling credential. A func, not a
	// string, so a refreshed GitHub App installation token takes effect
	// without restarting this process.
	Token func() string
	// Settings supplies live operational values.
	Settings func() *settings.Snapshot
}

// Coordinator owns the Initial/Freshness, Deep Diver and Discovery worker
// pools and the periodic claim loop that feeds them. No priority queue, no activity
// score — FIFO on next_retry_at via ClaimJobs (plan §7, D1/D9).
type Coordinator struct {
	pool   *db.Pool
	worker *Worker
	live   func() *settings.Snapshot

	initialPool   *worker.Pool
	deepPool      *worker.Pool
	discoveryPool *worker.Pool

	owner string

	// lastEligibilityCheck throttles the whole-table eligibility recompute,
	// which must not run on every 5s claim cycle.
	lastEligibilityCheck time.Time
	lastMetricsRefresh   time.Time
	lastLeaseReclaim     time.Time
	lastClosedPrune      time.Time
	lastExcludedPrune    time.Time

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
	discoveryCount := cfg.Settings().Int(settings.IngestDiscoveryWorkers)

	return &Coordinator{
		pool:        pool,
		worker:      NewWorker(pool, ghClient, cfg.Token, cfg.Settings),
		live:        cfg.Settings,
		initialPool: worker.NewPool("ingest-initial", initialCount, initialCount*8),
		deepPool:    worker.NewPool("ingest-deep", deepCount, deepCount*8),
		// One search page per claim; a deep queue would only hold leases on
		// windows nobody is fetching yet.
		discoveryPool: worker.NewPool("ingest-discovery", discoveryCount, discoveryCount*2),
		owner:         "ingestd-" + uuid.NewString()[:8],
		ctx:           ctx,
		cancel:        cancel,
	}
}

// ApplySettings pushes current concurrency limits into the pools. Safe to
// call on every settings reload.
func (c *Coordinator) ApplySettings(_ *settings.Snapshot) {
	s := c.live()
	c.initialPool.SetConcurrency(s.Int(settings.IngestRoutineWorkers))
	c.deepPool.SetConcurrency(s.Int(settings.IngestPriorityWorkers))
	c.discoveryPool.SetConcurrency(s.Int(settings.IngestDiscoveryWorkers))
}

// Start boots the claim loop in a background goroutine.
func (c *Coordinator) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		slog.Info("ingestion coordinator started", "owner", c.owner)

		interval := c.live().Seconds(settings.IngestClaimIntervalSeconds)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				slog.Info("ingestion coordinator stopped")
				return
			case <-ticker.C:
				c.cycle()
				// Pick up a changed ingest.claim_interval_seconds.
				if next := c.live().Seconds(settings.IngestClaimIntervalSeconds); next != interval {
					interval = next
					ticker.Reset(interval)
				}
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
	c.reclaimLeases(snap)
	c.refreshMetrics(snap)
	c.checkTrackedIssues(snap)
	c.pruneClosedIssues(snap)
	c.pruneExcludedIssues(snap)
	publishPoolStats(c.initialPool)
	publishPoolStats(c.deepPool)
	publishPoolStats(c.discoveryPool)
	c.completeSweeps()

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

	lease := snap.Seconds(settings.IngestLeaseSeconds)
	c.claimAndSubmit(JobInitial, c.initialPool, lease)
	c.claimAndSubmit(JobDeep, c.deepPool, lease)

	// 4. Discovery: start a sweep when one is due, then feed its windows.
	if snap.Bool(settings.IngestDiscoveryEnabled) {
		c.scheduleDiscovery(snap)
		c.claimDiscoveryWindows(lease)
	}
}

// completeSweeps closes sweeps whose windows have all finished. It runs even
// while discovery is disabled or the disk guard pauses new work: it only
// records what already happened.
func (c *Coordinator) completeSweeps() {
	done, err := CompleteSweeps(c.ctx, c.pool)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("discovery_complete").Inc()
		slog.Warn("failed to complete discovery sweeps", "error", err)
		return
	}
	for _, s := range done {
		if s.Status == "failed" {
			slog.Warn("discovery sweep ended with failed windows; the discovery watermark does not advance",
				"sweep_id", s.ID, "kind", s.Kind)
			continue
		}
		slog.Info("discovery sweep completed", "sweep_id", s.ID, "kind", s.Kind)
	}
}

// scheduleDiscovery starts a full or incremental sweep when one is due.
func (c *Coordinator) scheduleDiscovery(snap *settings.Snapshot) {
	kind, due, err := DueSweep(c.ctx, c.pool,
		snap.Seconds(settings.IngestDiscoveryFullIntervalSeconds),
		snap.Seconds(settings.IngestDiscoveryIncrementalIntervalSeconds))
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("discovery_schedule").Inc()
		slog.Warn("failed to read the discovery schedule", "error", err)
		return
	}
	if due {
		// Failures are counted and logged inside; the next cycle retries.
		_, _, _ = c.StartSweep(kind)
	}
}

// StartSweep starts a discovery sweep now, unless one is running
// (ErrSweepRunning). The scheduler and ingestd's admin trigger both call it.
func (c *Coordinator) StartSweep(kind SweepKind) (int64, SweepKind, error) {
	id, started, err := StartSweep(c.ctx, c.pool, kind, sweepParams(c.live()))
	if errors.Is(err, ErrSweepRunning) {
		return 0, kind, err
	}
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("discovery_start").Inc()
		slog.Warn("failed to start a discovery sweep", "kind", kind, "error", err)
		return 0, kind, err
	}
	slog.Info("discovery sweep started", "sweep_id", id, "kind", started)
	return id, started, nil
}

func (c *Coordinator) claimDiscoveryWindows(lease time.Duration) {
	stats := c.discoveryPool.Stats()
	capacity := stats.QueueCapacity - stats.QueueLength
	if capacity <= 0 {
		return
	}
	windows, err := ClaimDiscoveryWindows(c.ctx, c.pool, capacity, c.owner, lease)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("discovery_claim").Inc()
		slog.Warn("failed to claim discovery windows", "error", err)
		return
	}
	for _, win := range windows {
		wn := win
		if err := c.discoveryPool.Submit(func(workerCtx context.Context) error {
			if err := c.worker.ProcessDiscoveryWindow(workerCtx, wn); err != nil {
				slog.Warn("discovery window processing failed", "window_id", wn.ID, "error", err)
			}
			return nil
		}); err != nil {
			if err := ReleaseDiscoveryWindow(c.ctx, c.pool, wn, 5*time.Second); err != nil {
				metrics.IngestCycleErrors.WithLabelValues("discovery_release").Inc()
				slog.Warn("failed to release unsubmitted discovery window; its lease will expire", "window_id", wn.ID, "error", err)
			}
		}
	}
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

// checkTrackedIssues re-checks, on GitHub, open issues users have saved or
// are working on once they are older than ingest.tracked_issue_check_seconds,
// one batch per cycle. It runs before the disk guard: it adds no rows, and
// a user's tracked issue closing matters even while new crawling is paused.
// It shares the crawler lane's budget and floor.
func (c *Coordinator) checkTrackedIssues(snap *settings.Snapshot) {
	due, err := DueTrackedIssues(c.ctx, c.pool,
		snap.Seconds(settings.IngestTrackedIssueCheckSeconds), snap.Int(settings.IngestTrackedIssueBatchSize))
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("tracked_select").Inc()
		slog.Warn("failed to select tracked issues to check", "error", err)
		return
	}
	if len(due) == 0 {
		return
	}
	token := c.worker.token()
	if token == "" {
		metrics.IngestCycleErrors.WithLabelValues("tracked_no_token").Inc()
		slog.Warn("tracked issues due for a check, but no github token is configured", "due", len(due))
		return
	}

	proceed, err := CanProceed(c.ctx, c.pool, LaneCrawler, installationIdentity, snap.Int(settings.IngestRateLimitFloor))
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(LaneCrawler)).Inc()
		slog.Warn("budget check failed, checking tracked issues anyway", "error", err)
	} else if !proceed {
		return
	}

	refs, err := trackedRefs(due)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("tracked_select").Inc()
		slog.Warn("cannot check tracked issues", "error", err)
		return
	}
	states, rl, err := c.worker.github.FetchIssueStates(c.ctx, token, refs)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("tracked_fetch").Inc()
		slog.Warn("failed to fetch tracked issue states", "count", len(refs), "error", err)
		return
	}
	if err := UpdateBudget(c.ctx, c.pool, LaneCrawler, installationIdentity, rl); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
		slog.Warn("failed to persist budget", "error", err)
	}

	closed, err := ApplyTrackedStates(c.ctx, c.pool, due, states)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("tracked_apply").Inc()
		slog.Warn("failed to persist tracked issue states", "count", len(due), "error", err)
		return
	}
	metrics.IngestIssuesClosed.WithLabelValues("tracked").Add(float64(closed))
	slog.Info("checked tracked issues", "checked", len(due), "closed", closed)
}

// pruneClosedIssues deletes issues closed longer ago than
// ingest.closed_issue_retention_days that nobody saved or worked on, at most
// once per ingest.closed_issue_prune_interval_seconds. Each run deletes
// batches until one comes back short, so a backlog clears in one run while
// no single statement holds locks for long. Retention 0 disables it.
func (c *Coordinator) pruneClosedIssues(snap *settings.Snapshot) {
	days := snap.Int(settings.IngestClosedIssueRetentionDays)
	if days == 0 || time.Since(c.lastClosedPrune) < snap.Seconds(settings.IngestClosedIssuePruneIntervalSeconds) {
		return
	}
	c.lastClosedPrune = time.Now()

	retention := time.Duration(days) * 24 * time.Hour
	batch := snap.Int(settings.IngestClosedIssuePruneBatchSize)
	total := 0
	for c.ctx.Err() == nil {
		n, err := PruneClosedIssues(c.ctx, c.pool, retention, batch)
		if err != nil {
			metrics.IngestCycleErrors.WithLabelValues("prune_closed").Inc()
			slog.Warn("failed to prune closed issues", "deleted_before_error", total, "error", err)
			break
		}
		total += n
		metrics.IngestIssuesPruned.Add(float64(n))
		if n < batch {
			break
		}
	}
	if total > 0 {
		slog.Info("pruned closed issues", "deleted", total, "retention_days", days)
	}
}

// pruneExcludedIssues deletes the stored issues of excluded and archived
// repositories (not ones a user saved), on the same interval as the closed-
// issue prune but independent of its retention: turning that off does not
// keep an excluded repository's issues around.
func (c *Coordinator) pruneExcludedIssues(snap *settings.Snapshot) {
	if time.Since(c.lastExcludedPrune) < snap.Seconds(settings.IngestClosedIssuePruneIntervalSeconds) {
		return
	}
	c.lastExcludedPrune = time.Now()

	batch := snap.Int(settings.IngestClosedIssuePruneBatchSize)
	total := 0
	for c.ctx.Err() == nil {
		n, err := PruneExcludedIssues(c.ctx, c.pool, batch)
		if err != nil {
			metrics.IngestCycleErrors.WithLabelValues("prune_excluded").Inc()
			slog.Warn("failed to prune excluded repositories' issues", "deleted_before_error", total, "error", err)
			break
		}
		total += n
		metrics.IngestIssuesPruned.Add(float64(n))
		if n < batch {
			break
		}
	}
	if total > 0 {
		slog.Info("pruned issues of excluded and archived repositories", "deleted", total)
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

// reclaimLeases returns jobs whose lease expired (a crashed or stalled
// worker) to pending, no more often than ingest.lease_reclaim_interval_seconds.
// The first cycle always runs it, so leases a previous process left behind
// are freed before anything new is claimed. Safe while workers are running:
// every job write is fenced on lease_owner, so a stalled worker that wakes
// up after its job was reclaimed gets ErrLeaseLost instead of a write.
func (c *Coordinator) reclaimLeases(snap *settings.Snapshot) {
	if time.Since(c.lastLeaseReclaim) < snap.Seconds(settings.IngestLeaseReclaimIntervalSeconds) {
		return
	}
	c.lastLeaseReclaim = time.Now()
	n, err := ReclaimExpiredLeases(c.ctx, c.pool)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("reclaim_leases").Inc()
		slog.Warn("failed to reclaim expired leases", "error", err)
	} else if n > 0 {
		metrics.IngestLeasesReclaimed.Add(float64(n))
		slog.Warn("reclaimed expired job leases", "count", n)
	}

	n, err = ReclaimExpiredDiscoveryLeases(c.ctx, c.pool)
	if err != nil {
		metrics.IngestCycleErrors.WithLabelValues("reclaim_discovery_leases").Inc()
		slog.Warn("failed to reclaim expired discovery window leases", "error", err)
	} else if n > 0 {
		metrics.IngestLeasesReclaimed.Add(float64(n))
		slog.Warn("reclaimed expired discovery window leases", "count", n)
	}
}

// checkDiskGuard returns true if the coordinator should proceed with
// enqueuing/claiming this cycle, false if ingest.max_db_size_mb is exceeded
// and new work must pause. Logs loudly on each transition, not every cycle,
// so a sustained pause doesn't spam the log every cycle.
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

func (c *Coordinator) claimAndSubmit(jobType JobType, pool *worker.Pool, lease time.Duration) {
	stats := pool.Stats()
	capacity := stats.QueueCapacity - stats.QueueLength
	if capacity <= 0 {
		return
	}
	jobs, err := ClaimJobs(c.ctx, c.pool, jobType, capacity, c.owner, lease)
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
		"discovery_pool": c.discoveryPool.Stats(),
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
		// A pool that does not drain in time leaves its jobs running; their
		// leases expire and the next process reclaims them.
		if err := c.initialPool.Shutdown(timeout); err != nil {
			slog.Warn("initial pool did not drain before shutdown", "error", err)
		}
		if err := c.deepPool.Shutdown(timeout); err != nil {
			slog.Warn("deep pool did not drain before shutdown", "error", err)
		}
		if err := c.discoveryPool.Shutdown(timeout); err != nil {
			slog.Warn("discovery pool did not drain before shutdown", "error", err)
		}
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
