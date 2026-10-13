// Package migrate applies the database schema, which is embedded into the
// binary so no SQL files need to be mounted or copied into the image.
//
// The migrations in this package are the single source of truth for the
// schema. Never edit a migration that has been applied anywhere; add a new one.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// lockKey guards against two processes migrating at once. api and ingestd both
// start behind the same migrate step, but a replica restarting mid-deploy can
// still race, and concurrent DDL on the same table deadlocks rather than
// serialising.
const lockKey int64 = 0x5C111

// Run applies every pending migration. It is safe to call concurrently from
// multiple processes: all but one will block until the leader finishes.
func Run(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Advisory locks are scoped to a session, so the lock has to be taken and
	// released on one pinned connection rather than through the pool.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pin connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockKey); err != nil {
			slog.Warn("failed to release migration lock", "error", err)
		}
	}()

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(slogLogger{})
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	before, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	after, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if before == after {
		slog.Info("schema already current", "version", after)
	} else {
		slog.Info("schema migrated", "from_version", before, "to_version", after)
	}
	return nil
}

// slogLogger routes goose's own output through slog so migration logs match
// the structured format the rest of the services emit.
type slogLogger struct{}

func (slogLogger) Printf(format string, v ...any) {
	slog.Info("goose: " + trimNewline(fmt.Sprintf(format, v...)))
}

func (slogLogger) Fatalf(format string, v ...any) {
	slog.Error("goose: " + trimNewline(fmt.Sprintf(format, v...)))
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
