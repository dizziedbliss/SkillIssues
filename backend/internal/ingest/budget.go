// Package ingest implements the broad-coverage issue ingestion pipeline:
// an Initial/Freshness worker that fetches one page per eligible
// repository, a Deep Diver that owns subsequent pagination, a shared
// Postgres-backed GitHub rate-limit budget, and a leased job queue so any
// number of worker processes can claim work without double-processing a
// repository's cursor.
//
// See docs/ingestion-architecture.md for the design this
// package implements, and plan §7 for the safety invariants: atomic job
// claims, one active pagination job per repository, atomic progress (issue
// upserts and cursor advancement commit together or not at all), and no
// repository priority/ranking in the crawl path.
package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
)

// Lane separates the crawler's budget reservation from interactive,
// on-demand reads (plan §3.1), so a saturated crawl can't block issue
// browsing and heavy browsing can't stall the crawl.
type Lane string

const (
	LaneCrawler     Lane = "crawler"
	LaneInteractive Lane = "interactive"
)

// BudgetState is a point-in-time read of github_budget for one lane.
type BudgetState struct {
	Remaining      int
	Limit          int
	ResetAt        time.Time
	SecondaryUntil *time.Time
}

// UpdateBudget records the rate-limit telemetry from a GraphQL response.
// identity distinguishes which credential this reservation belongs to
// (e.g. "installation" or "user:<id>"); crawling always uses "installation".
func UpdateBudget(ctx context.Context, pool *db.Pool, lane Lane, identity string, rl github.GraphQLRateLimit) error {
	var resetAt *time.Time
	if !rl.ResetAt.IsZero() {
		resetAt = &rl.ResetAt
	}
	return pool.Exec(ctx,
		`INSERT INTO github_budget (identity, lane, rate_limit, remaining, reset_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now())
		 ON CONFLICT (identity, lane) DO UPDATE SET
			rate_limit = EXCLUDED.rate_limit,
			remaining  = EXCLUDED.remaining,
			reset_at   = EXCLUDED.reset_at,
			updated_at = now()`,
		identity, string(lane), rl.Limit, rl.Remaining, resetAt,
	)
}

// MarkSecondaryLimit records that GitHub's secondary rate limit fired,
// pausing scheduling on the lane until untilAt regardless of the primary
// point budget.
func MarkSecondaryLimit(ctx context.Context, pool *db.Pool, lane Lane, identity string, untilAt time.Time) error {
	return pool.Exec(ctx,
		`INSERT INTO github_budget (identity, lane, secondary_until, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (identity, lane) DO UPDATE SET
			secondary_until = EXCLUDED.secondary_until,
			updated_at = now()`,
		identity, string(lane), untilAt,
	)
}

// CanProceed reports whether the lane's shared budget allows another
// request: the primary point budget must be above floor (or already past
// its reset time), and any secondary-limit pause must have elapsed.
func CanProceed(ctx context.Context, pool *db.Pool, lane Lane, identity string, floor int) (bool, error) {
	var remaining, limit *int
	var resetAt, secondaryUntil *time.Time
	err := pool.QueryRow(ctx,
		`SELECT remaining, rate_limit, reset_at, secondary_until FROM github_budget WHERE identity = $1 AND lane = $2`,
		[]any{identity, string(lane)},
		func(row pgx.Row) error { return row.Scan(&remaining, &limit, &resetAt, &secondaryUntil) },
	)
	if err != nil {
		if err == db.ErrNotFound {
			// No telemetry yet: allow the first attempt so budget state can
			// actually be established.
			return true, nil
		}
		return false, fmt.Errorf("ingest: read budget: %w", err)
	}

	now := time.Now()
	if secondaryUntil != nil && now.Before(*secondaryUntil) {
		return false, nil
	}
	if remaining != nil && *remaining < floor && (resetAt == nil || now.Before(*resetAt)) {
		return false, nil
	}
	return true, nil
}
