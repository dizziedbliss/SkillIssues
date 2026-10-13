// Command migrate applies the embedded database schema and exits.
//
// It runs as a one-shot container that api and ingestd depend on via
// service_completed_successfully, so a fresh `docker compose up` produces a
// correct schema with no manual SQL.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dizziedbliss/skillissues/internal/migrate"
)

// connectAttempts covers the window where Postgres reports healthy but is not
// yet accepting connections.
const (
	connectAttempts = 10
	connectBackoff  = 2 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Read the DSN directly rather than through internal/config: this binary
	// needs nothing else, and config.Load additionally requires webhook and
	// admin secrets under ENV=production that a migration run has no use for.
	databaseURL := strings.Trim(strings.TrimSpace(os.Getenv("DATABASE_URL")), `"' `)
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		if err = migrate.Run(ctx, databaseURL); err == nil {
			slog.Info("migrations complete")
			return
		}
		if ctx.Err() != nil {
			slog.Error("interrupted before migrations completed", "error", err)
			os.Exit(1)
		}
		slog.Warn("migration attempt failed; retrying",
			"attempt", attempt, "of", connectAttempts, "error", err)

		select {
		case <-ctx.Done():
			os.Exit(1)
		case <-time.After(connectBackoff):
		}
	}

	slog.Error("migrations failed", "attempts", connectAttempts, "error", err)
	os.Exit(1)
}
