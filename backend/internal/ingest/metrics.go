package ingest

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/worker"
)

// Mirrors the CHECK constraints on ingest_jobs, so a combination that drops
// to zero rows is exported as 0 instead of keeping its last value.
var (
	jobTypes    = []string{"initial", "freshness", "deep", "discovery"}
	jobStatuses = []string{"pending", "running", "done", "failed"}
)

// RefreshQueueMetrics re-reads the Postgres-backed gauges. Each query is
// independent; the first failure is returned after the rest have run.
func RefreshQueueMetrics(ctx context.Context, pool *db.Pool) error {
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	note(refreshJobCounts(ctx, pool))
	note(refreshStuckLeases(ctx, pool))
	note(refreshRepositoryCounts(ctx, pool))
	note(refreshBudget(ctx, pool))
	return firstErr
}

func refreshJobCounts(ctx context.Context, pool *db.Pool) error {
	counts := map[[2]string]float64{}
	err := pool.QueryRows(ctx,
		`SELECT job_type, status, count(*) FROM ingest_jobs GROUP BY 1, 2`, nil,
		func(rows pgx.Rows) error {
			var jt, st string
			var n int64
			if err := rows.Scan(&jt, &st, &n); err != nil {
				return err
			}
			counts[[2]string{jt, st}] = float64(n)
			return nil
		})
	if err != nil {
		return fmt.Errorf("ingest: count jobs: %w", err)
	}
	for _, jt := range jobTypes {
		for _, st := range jobStatuses {
			metrics.IngestJobs.WithLabelValues(jt, st).Set(counts[[2]string{jt, st}])
		}
	}
	return nil
}

func refreshStuckLeases(ctx context.Context, pool *db.Pool) error {
	var n int64
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM ingest_jobs WHERE status = 'running' AND lease_expires_at < now()`, nil,
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return fmt.Errorf("ingest: count stuck leases: %w", err)
	}
	metrics.IngestStuckLeases.Set(float64(n))
	return nil
}

func refreshRepositoryCounts(ctx context.Context, pool *db.Pool) error {
	counts := map[[2]bool]float64{}
	err := pool.QueryRows(ctx,
		`SELECT is_eligible, last_synced_at IS NOT NULL, count(*) FROM repositories GROUP BY 1, 2`, nil,
		func(rows pgx.Rows) error {
			var eligible, synced bool
			var n int64
			if err := rows.Scan(&eligible, &synced, &n); err != nil {
				return err
			}
			counts[[2]bool{eligible, synced}] = float64(n)
			return nil
		})
	if err != nil {
		return fmt.Errorf("ingest: count repositories: %w", err)
	}
	for _, e := range []bool{true, false} {
		for _, s := range []bool{true, false} {
			metrics.RepositoriesEligible.WithLabelValues(strconv.FormatBool(e), strconv.FormatBool(s)).Set(counts[[2]bool{e, s}])
		}
	}
	return nil
}

// refreshBudget exports only the server installation's rows: per-user
// identities would make one series per user.
func refreshBudget(ctx context.Context, pool *db.Pool) error {
	err := pool.QueryRows(ctx,
		`SELECT lane, rate_limit, remaining, reset_at, secondary_until
		   FROM github_budget WHERE identity = $1`,
		[]any{installationIdentity},
		func(rows pgx.Rows) error {
			var lane string
			var limit, remaining *int
			var resetAt, secondaryUntil *time.Time
			if err := rows.Scan(&lane, &limit, &remaining, &resetAt, &secondaryUntil); err != nil {
				return err
			}
			if remaining != nil {
				metrics.GitHubBudgetRemaining.WithLabelValues(lane).Set(float64(*remaining))
			}
			if limit != nil {
				metrics.GitHubBudgetLimit.WithLabelValues(lane).Set(float64(*limit))
			}
			if resetAt != nil {
				metrics.GitHubBudgetResetSeconds.WithLabelValues(lane).Set(float64(resetAt.Unix()))
			}
			paused := 0.0
			if secondaryUntil != nil && time.Now().Before(*secondaryUntil) {
				paused = 1
			}
			metrics.GitHubSecondaryPaused.WithLabelValues(lane).Set(paused)
			return nil
		})
	if err != nil {
		return fmt.Errorf("ingest: read github_budget: %w", err)
	}
	return nil
}

func publishPoolStats(p *worker.Pool) {
	s := p.Stats()
	metrics.WorkerPool.WithLabelValues(s.Name, "concurrency").Set(float64(s.Concurrency))
	metrics.WorkerPool.WithLabelValues(s.Name, "active").Set(float64(s.ActiveWorkers))
	metrics.WorkerPool.WithLabelValues(s.Name, "queue_length").Set(float64(s.QueueLength))
	metrics.WorkerPool.WithLabelValues(s.Name, "queue_capacity").Set(float64(s.QueueCapacity))
}
