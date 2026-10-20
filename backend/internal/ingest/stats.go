package ingest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// catalogStatsRecount names the exact recount in catalog_refresh.
const catalogStatsRecount = "stats_recount"

// RollupStats folds the deltas the issues triggers wrote (migration 00043)
// into catalog_stats, catalog_stats_daily and catalog_activity, keeping the
// newest activityRows activity rows. With recount it also sets the totals to
// exact counts.
//
// It runs in one REPEATABLE READ transaction, so the deltas it folds and the
// issues it counts are the same snapshot: a change committed after it began
// has its delta left for the next rollup and is not in the count either, so
// nothing is counted twice or missed. It returns how many deltas it folded.
func RollupStats(ctx context.Context, pool *db.Pool, recount bool, activityRows int) (int, error) {
	tx, err := pool.Raw().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return 0, fmt.Errorf("ingest: begin stats rollup: %w", err)
	}
	// A no-op once committed; on an error path the error returned says what failed.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var folded int
	err = tx.QueryRow(ctx, `
		WITH d AS (
		  DELETE FROM catalog_stat_deltas RETURNING at, repository_id, indexed, arrived, opened, closed
		), tot AS (
		  SELECT coalesce(sum(indexed), 0) AS indexed, coalesce(sum(opened), 0) AS opened,
		         coalesce(sum(closed), 0) AS closed
		  FROM d
		), stats AS (
		  UPDATE catalog_stats s SET
		    value = s.value + CASE s.key WHEN 'issues_indexed' THEN tot.indexed
		                                 WHEN 'open_issues'    THEN tot.opened
		                                 ELSE tot.closed END,
		    updated_at = now()
		  FROM tot
		  WHERE s.key IN ('issues_indexed', 'open_issues', 'closed_since')
		), daily AS (
		  INSERT INTO catalog_stats_daily AS c (day, arrived, closed)
		  SELECT (at AT TIME ZONE 'UTC')::date, sum(arrived), sum(closed) FROM d
		  GROUP BY 1 HAVING sum(arrived) > 0 OR sum(closed) > 0
		  ON CONFLICT (day) DO UPDATE SET arrived = c.arrived + EXCLUDED.arrived, closed = c.closed + EXCLUDED.closed
		), activity AS (
		  INSERT INTO catalog_activity (at, repository_id, arrived, closed)
		  SELECT at, repository_id, arrived, closed FROM d
		  WHERE arrived > 0 OR closed > 0
		  ORDER BY at DESC LIMIT $1
		)
		SELECT count(*) FROM d`, activityRows).Scan(&folded)
	if err != nil {
		return 0, fmt.Errorf("ingest: fold stats deltas: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM catalog_activity WHERE id NOT IN (
		  SELECT id FROM catalog_activity ORDER BY at DESC, id DESC LIMIT $1
		)`, activityRows); err != nil {
		return 0, fmt.Errorf("ingest: trim stats activity: %w", err)
	}

	if recount {
		// issues_indexed counts every issue ever inserted, so pruning must not
		// lower it: the recount only ever raises it (and seeds it).
		if _, err := tx.Exec(ctx, `
			WITH counts AS (
			  SELECT (SELECT count(*) FROM issues WHERE state = 'OPEN')        AS open,
			         (SELECT count(*) FROM issues)                             AS total,
			         (SELECT count(*) FROM repositories WHERE is_eligible)     AS repos
			)
			INSERT INTO catalog_stats AS s (key, value)
			SELECT 'open_issues', open FROM counts
			UNION ALL SELECT 'issues_indexed', total FROM counts
			UNION ALL SELECT 'repositories_indexed', repos FROM counts
			ON CONFLICT (key) DO UPDATE SET
			  value = CASE WHEN s.key = 'issues_indexed' THEN greatest(s.value, EXCLUDED.value) ELSE EXCLUDED.value END,
			  updated_at = now()`); err != nil {
			return 0, fmt.Errorf("ingest: recount catalog stats: %w", err)
		}
		if err := markCatalogJobTx(ctx, tx, catalogStatsRecount); err != nil {
			return 0, fmt.Errorf("ingest: mark stats recount: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("ingest: commit stats rollup: %w", err)
	}
	return folded, nil
}
