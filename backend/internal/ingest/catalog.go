package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// Periodic catalog jobs, recorded in catalog_refresh so a restart does not run
// them early:
//
//	shuffle  re-randomise repositories.shuffle_key, the shuffled feed's order
//	facets   recompute catalog_facets, the filter dropdowns' values and counts

const (
	catalogShuffle = "shuffle"
	catalogFacets  = "facets"
)

// CatalogJobDue reports whether the named job last ran longer than every ago,
// or never has.
func CatalogJobDue(ctx context.Context, pool *db.Pool, name string, every time.Duration) (bool, error) {
	var at time.Time
	err := pool.QueryRow(ctx, `SELECT refreshed_at FROM catalog_refresh WHERE name = $1`, []any{name},
		func(row pgx.Row) error { return row.Scan(&at) })
	if errors.Is(err, db.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("ingest: read %s refresh time: %w", name, err)
	}
	return time.Since(at) >= every, nil
}

func markCatalogJobTx(ctx context.Context, tx pgx.Tx, name string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO catalog_refresh (name, refreshed_at) VALUES ($1, now())
		 ON CONFLICT (name) DO UPDATE SET refreshed_at = now()`, name)
	return err
}

// Reshuffle draws a new random order for the shuffled feed.
func Reshuffle(ctx context.Context, pool *db.Pool) (int, error) {
	var n int
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE repositories SET shuffle_key = floor(random() * 2147483647)::integer`)
		if err != nil {
			return err
		}
		n = int(tag.RowsAffected())
		return markCatalogJobTx(ctx, tx, catalogShuffle)
	})
	if err != nil {
		return 0, fmt.Errorf("ingest: reshuffle repositories: %w", err)
	}
	return n, nil
}

// RefreshFacets recomputes the filter dropdowns' values: per kind, the
// maxValues values with the most open issues. Languages, topics and tech are
// counted through each eligible repository's open issues (tech from every
// language GitHub reports in the repository, shown in its commonest
// spelling); labels across open issues, grouped case-insensitively — the
// labels filter matches that way — and shown in their commonest spelling.
// Readers see the old values until the new ones commit.
func RefreshFacets(ctx context.Context, pool *db.Pool, maxValues int) error {
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM catalog_facets`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			WITH open AS MATERIALIZED (
			  SELECT repository_id, count(*) AS n FROM issues WHERE state = 'OPEN' GROUP BY 1
			), repo AS MATERIALIZED (
			  SELECT r.language, r.topics, r.languages, open.n
			  FROM open JOIN repositories r ON r.id = open.repository_id
			  WHERE r.is_eligible
			), languages AS (
			  INSERT INTO catalog_facets (kind, value, issues)
			  SELECT 'language', language, sum(n)::int FROM repo
			  WHERE language IS NOT NULL AND language <> ''
			  GROUP BY language ORDER BY 3 DESC LIMIT $1
			  RETURNING 1
			), topics AS (
			  INSERT INTO catalog_facets (kind, value, issues)
			  SELECT 'topic', t, sum(n)::int FROM repo, unnest(topics) t
			  WHERE t <> ''
			  GROUP BY t ORDER BY 3 DESC LIMIT $1
			  RETURNING 1
			)
			INSERT INTO catalog_facets (kind, value, issues)
			SELECT 'tech', mode() WITHIN GROUP (ORDER BY e->>'name'), sum(n)::int
			FROM repo, jsonb_array_elements(CASE WHEN jsonb_typeof(languages) = 'array' THEN languages ELSE '[]'::jsonb END) e
			WHERE e->>'name' <> ''
			GROUP BY lower(e->>'name') ORDER BY 3 DESC LIMIT $1`, maxValues); err != nil {
			return fmt.Errorf("repository facets: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO catalog_facets (kind, value, issues)
			SELECT 'label', mode() WITHIN GROUP (ORDER BY l), count(*)::int
			FROM issues i, unnest(i.labels) l
			WHERE i.state = 'OPEN' AND l <> ''
			GROUP BY lower(l) ORDER BY 3 DESC LIMIT $1`, maxValues); err != nil {
			return fmt.Errorf("label facets: %w", err)
		}
		return markCatalogJobTx(ctx, tx, catalogFacets)
	})
	if err != nil {
		return fmt.Errorf("ingest: refresh facets: %w", err)
	}
	return nil
}
