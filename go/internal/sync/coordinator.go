package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/worker"
	"github.com/jackc/pgx/v5"
)

// RateLimitBudget monitors GitHub GraphQL API quota to avoid hitting secondary rate limits.
type RateLimitBudget struct {
	mu        sync.RWMutex
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Used      int       `json:"used"`
	ResetAt   time.Time `json:"reset_at"`
	Cost      int       `json:"cost"`
}

// Update records new rate limit measurements returned by GitHub GraphQL.
func (b *RateLimitBudget) Update(rl github.GraphQLRateLimit) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Limit = rl.Limit
	b.Remaining = rl.Remaining
	b.Used = rl.Used
	b.ResetAt = rl.ResetAt
	b.Cost = rl.Cost
}

// CanProceed checks whether there is sufficient rate limit capacity remaining.
func (b *RateLimitBudget) CanProceed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.Remaining == 0 && b.Limit == 0 {
		// Budget not initialized yet; allow first attempt
		return true
	}
	if b.Remaining < 100 && time.Now().Before(b.ResetAt) {
		return false
	}
	return true
}

// BudgetSnapshot contains a point-in-time copy of rate limit metrics without lock fields.
type BudgetSnapshot struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
}

// Snapshot returns the current rate limit telemetry.
func (b *RateLimitBudget) Snapshot() BudgetSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BudgetSnapshot{
		Limit:     b.Limit,
		Remaining: b.Remaining,
		ResetAt:   b.ResetAt,
	}
}

// CoordinatorConfig configures the sync coordinator.
type CoordinatorConfig struct {
	WorkerCount         int
	PriorityWorkerCount int
	OnDemandWorkerCount int
	SyncInterval        time.Duration
	GitHubToken         string
}

// Coordinator orchestrates multi-tiered background synchronization for open-source issue ingestion.
type Coordinator struct {
	db           *db.Pool
	cache        *cache.Client
	github       *github.Client
	token        string
	routinePool  *worker.Pool
	priorityPool *worker.Pool
	onDemandPool *worker.Pool
	budget       *RateLimitBudget
	syncInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	syncLock     sync.Mutex
}

// NewCoordinator creates a new Sync Coordinator.
func NewCoordinator(dbPool *db.Pool, cacheClient *cache.Client, ghClient *github.Client, cfg CoordinatorConfig) *Coordinator {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.PriorityWorkerCount <= 0 {
		cfg.PriorityWorkerCount = 2
	}
	if cfg.OnDemandWorkerCount <= 0 {
		cfg.OnDemandWorkerCount = 8
	}
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 15 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Coordinator{
		db:           dbPool,
		cache:        cacheClient,
		github:       ghClient,
		token:        cfg.GitHubToken,
		routinePool:  worker.NewPool("sync-routine", cfg.WorkerCount, cfg.WorkerCount*8),
		priorityPool: worker.NewPool("sync-priority", cfg.PriorityWorkerCount, cfg.PriorityWorkerCount*8),
		onDemandPool: worker.NewPool("sync-ondemand", cfg.OnDemandWorkerCount, cfg.OnDemandWorkerCount*8),
		budget:       &RateLimitBudget{},
		syncInterval: cfg.SyncInterval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start boots the routine scheduler ticker loop in a background goroutine.
func (c *Coordinator) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		slog.Info("sync coordinator scheduler started", "interval", c.syncInterval.String())

		ticker := time.NewTicker(c.syncInterval)
		defer ticker.Stop()

		// Run an initial routine sync on boot
		_, _ = c.TriggerRoutineSync(c.ctx)

		for {
			select {
			case <-c.ctx.Done():
				slog.Info("sync coordinator scheduler stopped")
				return
			case <-ticker.C:
				if _, err := c.TriggerRoutineSync(c.ctx); err != nil {
					slog.Warn("routine sync cycle encountered error", "error", err)
				}
			}
		}
	}()
}

// TriggerRoutineSync enqueues pending repositories into the routine worker pool.
func (c *Coordinator) TriggerRoutineSync(ctx context.Context) (int, error) {
	if !c.budget.CanProceed() {
		slog.Warn("rate limit budget depleted, skipping routine sync cycle", "reset_at", c.budget.Snapshot().ResetAt)
		return 0, nil
	}

	jobs, err := GetPendingRepositories(ctx, c.db, 20)
	if err != nil {
		return 0, fmt.Errorf("fetch pending repositories: %w", err)
	}

	enqueued := 0
	for _, job := range jobs {
		j := job
		err := c.routinePool.Submit(func(workerCtx context.Context) error {
			return c.SyncRepository(workerCtx, j, c.token)
		})
		if err != nil {
			slog.Debug("routine worker pool queue full, skipping repo for now", "repo", j.FullName)
		} else {
			enqueued++
		}
	}

	slog.Info("routine sync enqueued repositories", "count", enqueued, "available", len(jobs))
	return enqueued, nil
}

// TriggerPrioritySync enqueues a priority synchronization for a specific repository.
func (c *Coordinator) TriggerPrioritySync(ctx context.Context, repoID int) error {
	_ = SetRepositoryPriority(ctx, c.db, repoID, "HIGH")

	job := RepositoryJob{ID: repoID}
	err := c.priorityPool.Submit(func(workerCtx context.Context) error {
		var fullName string
		var lang *string
		var stars int
		query := "SELECT full_name, language, stars FROM repositories WHERE id = $1"
		if err := c.db.QueryRow(workerCtx, query, []any{repoID}, func(row pgx.Row) error {
			return row.Scan(&fullName, &lang, &stars)
		}); err != nil {
			return err
		}
		job.FullName = fullName
		job.Language = lang
		job.Stars = stars
		return c.SyncRepository(workerCtx, job, c.token)
	})

	return err
}

// SyncRepository performs the full GitHub GraphQL sync for a single repository.
func (c *Coordinator) SyncRepository(ctx context.Context, repo RepositoryJob, token string) error {
	if token == "" {
		slog.Debug("github token not configured; skipping sync for repository", "repo", repo.FullName)
		return nil
	}

	if !c.budget.CanProceed() {
		slog.Warn("rate limit low, aborting repository sync", "repo", repo.FullName)
		return errors.New("github rate limit low")
	}

	parts := strings.Split(repo.FullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository full_name: %s", repo.FullName)
	}
	owner, name := parts[0], parts[1]

	_ = MarkSyncStarted(ctx, c.db, repo.ID)

	var cursor *string
	var since *time.Time
	if repo.LastIssueSyncAt != nil && !repo.LastIssueSyncAt.IsZero() {
		since = repo.LastIssueSyncAt
	}

	totalSynced := 0
	var lastRateLimit github.GraphQLRateLimit
	repoLang := ""
	if repo.Language != nil {
		repoLang = *repo.Language
	}

	for {
		var page *github.RepositoryIssuesPage
		var fetchErr error

		// Retry up to 3 times with exponential backoff
		for attempt := 0; attempt < 3; attempt++ {
			page, fetchErr = c.github.FetchRepositoryIssues(ctx, token, owner, name, cursor, since)
			if fetchErr == nil {
				break
			}
			if strings.Contains(fetchErr.Error(), "Could not resolve to a Repository") {
				slog.Info("repository is private or does not exist on GitHub, skipping", "repo", repo.FullName)
				_ = MarkSyncComplete(ctx, c.db, repo.ID, github.GraphQLRateLimit{})
				return nil
			}
			if strings.Contains(fetchErr.Error(), "HTTP 401") || strings.Contains(fetchErr.Error(), "Bad credentials") {
				_ = MarkSyncFailed(ctx, c.db, repo.ID)
				return fetchErr
			}
			if strings.Contains(fetchErr.Error(), "rate limit exceeded") {
				_ = MarkSyncFailed(ctx, c.db, repo.ID)
				return fetchErr
			}
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		if fetchErr != nil {
			_ = MarkSyncFailed(ctx, c.db, repo.ID)
			return fmt.Errorf("fetch issues for %s: %w", repo.FullName, fetchErr)
		}

		lastRateLimit = page.RateLimit
		c.budget.Update(page.RateLimit)

		for _, issue := range page.Issues {
			if strings.ToUpper(issue.State) != "OPEN" {
				continue
			}
			if len(issue.Labels) == 0 {
				continue
			}

			score, difficulty, hasValid, skills, tech, tags := ExtractMetadata(issue.Labels, issue.Body, repoLang)
			if err := UpsertIssue(ctx, c.db, repo.ID, issue, score, difficulty, hasValid, skills, tech, tags); err != nil {
				slog.Warn("failed to upsert issue", "repo", repo.FullName, "issue", issue.Number, "error", err)
			} else {
				totalSynced++
			}
		}

		if page.RateLimit.Remaining < 100 {
			slog.Warn("rate limit remaining low (<100), stopping pagination", "repo", repo.FullName, "remaining", page.RateLimit.Remaining)
			break
		}

		if !page.HasNextPage || page.EndCursor == "" {
			break
		}
		cursor = &page.EndCursor
	}

	if err := MarkSyncComplete(ctx, c.db, repo.ID, lastRateLimit); err != nil {
		slog.Warn("failed to mark sync complete", "repo", repo.FullName, "error", err)
	}

	slog.Info("synced repository issues", "repo", repo.FullName, "issues_synced", totalSynced)
	return nil
}

// Status returns coordinator operational telemetry.
func (c *Coordinator) Status() map[string]any {
	return map[string]any{
		"rate_limit":      c.budget.Snapshot(),
		"routine_pool":   c.routinePool.Stats(),
		"priority_pool":  c.priorityPool.Stats(),
		"on_demand_pool": c.onDemandPool.Stats(),
	}
}

// Shutdown gracefully shuts down worker pools and the scheduler loop.
func (c *Coordinator) Shutdown(timeout time.Duration) error {
	c.cancel()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		_ = c.routinePool.Shutdown(timeout)
		_ = c.priorityPool.Shutdown(timeout)
		_ = c.onDemandPool.Shutdown(timeout)
		close(done)
	}()

	select {
	case <-done:
		slog.Info("sync coordinator shutdown complete")
		return nil
	case <-time.After(timeout):
		return errors.New("sync coordinator shutdown timed out")
	}
}
