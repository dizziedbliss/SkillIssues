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

// Lane separates the crawler's budget reservation from interactive work —
// checks made because a user is waiting on them — so a saturated crawl
// cannot block a user and user work cannot stall the crawl.
//
// Both lanes spend one GitHub budget: GitHub counts points per credential,
// not per lane. Each lane records the telemetry its own calls return, and
// CanProceed reads them together as the one counter they are. What separates
// the lanes is how far each may spend it: the crawler lane stops while a
// reserve of ingest.interactive_reserve_percent of the limit is still left,
// and only the interactive lane spends into it.
type Lane string

const (
	LaneCrawler     Lane = "crawler"
	LaneInteractive Lane = "interactive"
)

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
// point budget. GitHub applies a secondary limit to the credential, so
// CanProceed pauses every lane of the identity, not just this one.
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

// CanProceed reports whether the identity's budget allows another request on
// the lane: no secondary-limit pause may be in force on any lane, and the
// primary point budget must be above the lane's floor (or already past its
// reset time).
//
// floor is the caller's own floor (ingest.rate_limit_floor, or discovery's
// higher one). On the crawler lane it is measured above the interactive
// reserve, reservePercent of the limit: with a 12,500-point limit and 30%, a
// floor of 100 stops the crawl at 3,850 remaining, and discovery's 1,000 at
// 4,750, so discovery still yields first. The interactive lane ignores the
// reserve and stops at floor alone.
//
// Remaining is read across every lane's row: the counter is shared and, within
// one reset window, only ever falls, so the lowest value among the rows of the
// newest window is the freshest one, whichever lane saw it. A row from an
// earlier window describes a budget that has since been refilled. (Telemetry
// without a reset time, which GitHub should never send, forms a window of its
// own and still counts against the floor.)
func CanProceed(ctx context.Context, pool *db.Pool, lane Lane, identity string, floor, reservePercent int) (bool, error) {
	var remaining, limit *int
	var resetAt, secondaryUntil *time.Time
	err := pool.QueryRow(ctx,
		`WITH budget AS (SELECT * FROM github_budget WHERE identity = $1),
		      latest AS (SELECT max(reset_at) AS reset_at FROM budget WHERE remaining IS NOT NULL)
		 SELECT (SELECT min(b.remaining)  FROM budget b, latest w WHERE b.reset_at IS NOT DISTINCT FROM w.reset_at),
		        (SELECT max(b.rate_limit) FROM budget b, latest w WHERE b.reset_at IS NOT DISTINCT FROM w.reset_at),
		        (SELECT reset_at FROM latest),
		        (SELECT max(secondary_until) FROM budget)`,
		[]any{identity},
		func(row pgx.Row) error { return row.Scan(&remaining, &limit, &resetAt, &secondaryUntil) },
	)
	if err != nil {
		return false, fmt.Errorf("ingest: read budget: %w", err)
	}

	now := time.Now()
	if secondaryUntil != nil && now.Before(*secondaryUntil) {
		return false, nil
	}
	if remaining == nil {
		// No telemetry yet: allow the first attempt so budget state can
		// actually be established.
		return true, nil
	}
	if resetAt != nil && !now.Before(*resetAt) {
		// The window this telemetry describes has reset.
		return true, nil
	}
	if lane == LaneCrawler && limit != nil {
		floor += *limit * reservePercent / 100
	}
	return *remaining >= floor, nil
}
