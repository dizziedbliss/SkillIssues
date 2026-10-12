package ingest

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
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
	if n, err := EnqueueInitialJobs(c.ctx, c.pool, 500); err != nil {
		slog.Warn("failed to enqueue initial jobs", "error", err)
	} else if n > 0 {
		slog.Info("enqueued initial jobs for newly eligible repositories", "count", n)
	}

	c.claimAndSubmit(JobInitial, c.initialPool)
	c.claimAndSubmit(JobDeep, c.deepPool)
}

func (c *Coordinator) claimAndSubmit(jobType JobType, pool *worker.Pool) {
	stats := pool.Stats()
	capacity := stats.QueueCapacity - stats.QueueLength
	if capacity <= 0 {
		return
	}
	jobs, err := ClaimJobs(c.ctx, c.pool, jobType, capacity, c.owner)
	if err != nil {
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
			_ = ReleaseJob(c.ctx, c.pool, j, 5*time.Second)
		}
	}
}

// Status returns coordinator operational telemetry.
func (c *Coordinator) Status() map[string]any {
	return map[string]any{
		"initial_pool": c.initialPool.Stats(),
		"deep_pool":    c.deepPool.Stats(),
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
