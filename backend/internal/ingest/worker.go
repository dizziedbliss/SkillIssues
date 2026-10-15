package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// installationIdentity is the budget row key for the server's own crawling
// credential — every worker process shares one row, since they all crawl
// with the same installation token (or GITHUB_TOKEN while the GitHub App
// isn't configured yet).
const installationIdentity = "installation"

// Worker fetches and persists one page for a claimed job.
type Worker struct {
	pool   *db.Pool
	github *github.Client
	token  func() string
	live   func() *settings.Snapshot
}

// NewWorker builds a Worker. token is read per call (not captured once) so
// a refreshed installation token takes effect without restarting.
func NewWorker(pool *db.Pool, ghClient *github.Client, token func() string, live func() *settings.Snapshot) *Worker {
	return &Worker{pool: pool, github: ghClient, token: token, live: live}
}

// ProcessJob fetches exactly one page for job and persists it atomically.
// Both "initial" and "deep" jobs go through the same path — they differ
// only in where their cursor starts and what ClaimJobs fed in.
func (w *Worker) ProcessJob(ctx context.Context, job Job) error {
	outcome, err := w.processJob(ctx, job)
	if errors.Is(err, ErrLeaseLost) {
		// The lease expired and the job was reclaimed while this worker
		// held it; nothing it wrote was applied. Raise ingest.lease_seconds
		// if this is frequent.
		outcome = "lease_lost"
	}
	metrics.IngestJobOutcomes.WithLabelValues(string(job.JobType), outcome).Inc()
	return err
}

func (w *Worker) processJob(ctx context.Context, job Job) (string, error) {
	token := w.token()
	if token == "" {
		return "no_token", fmt.Errorf("no github token configured")
	}

	owner, name, ok := strings.Cut(job.FullName, "/")
	if !ok {
		if err := MarkUnavailable(ctx, w.pool, job, "invalid full_name"); err != nil {
			return "persist_error", fmt.Errorf("mark %q unavailable: %w", job.FullName, err)
		}
		return "invalid_repo", fmt.Errorf("invalid repository full_name: %q", job.FullName)
	}

	snap := w.live()
	floor := snap.Int(settings.IngestRateLimitFloor)
	proceed, err := CanProceed(ctx, w.pool, LaneCrawler, installationIdentity, floor)
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(LaneCrawler)).Inc()
		slog.Warn("budget check failed, proceeding cautiously", "error", err)
	} else if !proceed {
		// Not the job's fault — release it rather than counting an attempt.
		if err := ReleaseJob(ctx, w.pool, job, snap.Seconds(settings.IngestRateLimitPauseSeconds)); err != nil {
			return "persist_error", fmt.Errorf("release job: %w", err)
		}
		return "budget_deferred", nil
	}

	// A pass pages through two connections in lockstep: open issues (all of
	// them on a full crawl, or those updated since the watermark), and on
	// an incremental pass the issues closed since the watermark. Each stops
	// being requested once it is exhausted.
	query := github.IssuesQuery{
		FetchOpen:      !job.OpenDone,
		OpenCursor:     job.Cursor,
		PageSize:       snap.Int(settings.IngestGraphQLPageSize),
		Since:          job.Since,
		FetchClosed:    job.Since != nil && !job.ClosedDone,
		ClosedCursor:   job.ClosedCursor,
		ClosedPageSize: snap.Int(settings.IngestClosedPageSize),
		FetchStats:     job.PagesDone == 0,
	}
	page, err := w.fetchWithBatchLadder(ctx, token, owner, name, query)
	if err != nil {
		return w.handleFetchError(ctx, job, err)
	}

	if err := UpdateBudget(ctx, w.pool, LaneCrawler, installationIdentity, page.RateLimit); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
		slog.Warn("failed to persist budget", "error", err)
	}

	repoLang := ""
	if job.Language != nil {
		repoLang = *job.Language
	}

	tx, err := w.pool.Raw().Begin(ctx)
	if err != nil {
		return "persist_error", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	upserted := 0
	for _, issue := range page.Issues {
		if err := UpsertIssueTx(ctx, tx, job.RepositoryID, issue, repoLang); err != nil {
			return "persist_error", fmt.Errorf("upsert issue #%d: %w", issue.Number, err)
		}
		upserted++
	}

	closed, err := MarkIssuesClosedTx(ctx, tx, job.RepositoryID, page.ClosedIssues)
	if err != nil {
		return "persist_error", err
	}
	if page.Stats != nil {
		if err := UpdateRepositoryStatsTx(ctx, tx, job.RepositoryID, *page.Stats); err != nil {
			return "persist_error", err
		}
	}

	progress := PageProgress{
		OpenCursor:   derefOr(job.Cursor, ""),
		OpenDone:     job.OpenDone,
		ClosedCursor: derefOr(job.ClosedCursor, ""),
		ClosedDone:   job.ClosedDone || !query.FetchClosed,
		PagesDone:    job.PagesDone + 1,
	}
	if query.FetchOpen {
		progress.OpenCursor = page.EndCursor
		progress.OpenDone = !page.HasNextPage
	}
	if query.FetchClosed {
		progress.ClosedCursor = page.ClosedEndCursor
		progress.ClosedDone = !page.ClosedHasNextPage
	}
	maxPages := snap.Int(settings.IngestMaxPagesPerRepo)
	if maxPages > 0 && progress.PagesDone >= maxPages && !(progress.OpenDone && progress.ClosedDone) {
		slog.Warn("repository hit the per-repository page cap, stopping pagination here",
			"repo", job.FullName, "pages_done", progress.PagesDone, "max_pages_per_repo", maxPages)
	}
	if err := CompleteOrRequeue(ctx, tx, job, progress, maxPages); err != nil {
		return "persist_error", fmt.Errorf("complete/requeue job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "persist_error", fmt.Errorf("commit page: %w", err)
	}
	metrics.IngestIssuesUpserted.Add(float64(upserted))
	metrics.IngestIssuesClosed.WithLabelValues("crawl").Add(float64(closed))

	slog.Info("page ingested",
		"repo", job.FullName, "job_type", job.JobType, "incremental", job.Since != nil,
		"issues", upserted, "closed", closed,
		"has_next_page", !(progress.OpenDone && progress.ClosedDone), "remaining_budget", page.RateLimit.Remaining,
	)
	return "page_ingested", nil
}

// fetchWithBatchLadder implements plan §7's batch ladder, applied to page
// size (this worker fetches one repository per call rather than GraphQL
// alias-batching several — see the ingestion doc's throughput-vs-safety
// tradeoff notes). On RESOURCE_LIMITS_EXCEEDED or a timeout-shaped error,
// it halves the page size and retries, bottoming out at 1, then restores
// the configured size for the *next* call.
func (w *Worker) fetchWithBatchLadder(ctx context.Context, token, owner, name string, q github.IssuesQuery) (*github.RepositoryIssuesPage, error) {
	for {
		page, err := w.github.FetchRepositoryIssues(ctx, token, owner, name, q)
		if err == nil {
			return page, nil
		}
		if !isResourceLimitError(err) || (q.PageSize <= 1 && q.ClosedPageSize <= 1) {
			return nil, err
		}
		q.PageSize = max(q.PageSize/2, 1)
		q.ClosedPageSize = max(q.ClosedPageSize/2, 1)
		slog.Warn("resource limit hit, shrinking page size and retrying",
			"repo", owner+"/"+name, "new_page_size", q.PageSize, "new_closed_page_size", q.ClosedPageSize, "error", err)
	}
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

func isResourceLimitError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "RESOURCE_LIMITS_EXCEEDED") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "Timeout")
}

// handleFetchError classifies a fetch failure: NOT_FOUND isolates the
// repository permanently; secondary/primary rate limits pause the lane and
// retry later; anything else goes through the normal backoff/retry path.
// The returned outcome labels ingest_job_outcomes_total.
func (w *Worker) handleFetchError(ctx context.Context, job Job, err error) (string, error) {
	msg := err.Error()
	snap := w.live()

	if strings.Contains(msg, "Could not resolve to a Repository") || strings.Contains(msg, "NOT_FOUND") {
		slog.Info("repository not found on GitHub, marking unavailable", "repo", job.FullName)
		if err := MarkUnavailable(ctx, w.pool, job, "not_found"); err != nil {
			return "persist_error", fmt.Errorf("mark %q unavailable: %w", job.FullName, err)
		}
		return "not_found", nil
	}

	if strings.Contains(msg, "rate limit exceeded") || strings.Contains(msg, "HTTP 403") || strings.Contains(msg, "HTTP 429") {
		until := time.Now().Add(snap.Seconds(settings.IngestRateLimitPauseSeconds))
		if err := MarkSecondaryLimit(ctx, w.pool, LaneCrawler, installationIdentity, until); err != nil {
			metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
			slog.Warn("failed to record secondary rate limit pause", "error", err)
		}
		if err := FailJob(ctx, w.pool, job, "rate_limit",
			msg, snap.Seconds(settings.IngestBackoffBaseSeconds), snap.Int(settings.IngestMaxAttempts)); err != nil {
			return "persist_error", fmt.Errorf("fail job: %w", err)
		}
		return "rate_limited", nil
	}

	if err := FailJob(ctx, w.pool, job, "fetch_error", msg,
		snap.Seconds(settings.IngestBackoffBaseSeconds), snap.Int(settings.IngestMaxAttempts)); err != nil {
		return "persist_error", fmt.Errorf("fail job: %w", err)
	}
	return "fetch_error", nil
}
