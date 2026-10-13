package ingest

import (
	"context"
	"fmt"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// CurrentDBSizeMB returns pg_database_size() for the connected database, in
// megabytes (floored). It is the D6 disk guardrail's source of truth —
// checked against Postgres itself rather than container free space, since
// ingestd cannot see the postgres volume from inside its own container.
func CurrentDBSizeMB(ctx context.Context, pool *db.Pool) (int64, error) {
	var bytes int64
	err := pool.QueryRow(ctx,
		`SELECT pg_database_size(current_database())`,
		nil,
		func(row pgx.Row) error { return row.Scan(&bytes) },
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: read pg_database_size: %w", err)
	}
	return bytes / (1024 * 1024), nil
}
