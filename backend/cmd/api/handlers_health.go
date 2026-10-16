package main

import (
	"log/slog"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/ingest"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

// health handles GET /health
// Response: {"status":"ok","service":"api"}
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	// Run a basic DB check matching the Python implementation
	if err := h.db.Ping(r.Context()); err != nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "api",
	})
}

// healthDetail handles GET /health/detail
// Returns counts of repositories and issues, plus the D6
// disk guardrail state so the database-size ceiling is visible before it's
// hit, not discovered after ingestion has already stopped.
func (h *handlers) healthDetail(w http.ResponseWriter, r *http.Request) {
	var repoCount, issueCount int
	status := "ok"
	if err := h.db.QueryRow(r.Context(),
		`SELECT
			(SELECT COUNT(*) FROM repositories),
			(SELECT COUNT(*) FROM issues)`,
		nil,
		func(row pgx.Row) error {
			return row.Scan(&repoCount, &issueCount)
		},
	); err != nil {
		// A component that is down must not be reported as connected: the
		// counts below would otherwise silently read as zero instead of
		// "unknown", which looks identical to an empty catalog.
		slog.Warn("health/detail: failed to read catalog counts", "error", err)
		status = "degraded"
	}

	cacheStatus := "unavailable"
	if h.cache != nil {
		if err := h.cache.Ping(r.Context()); err == nil {
			cacheStatus = "ok"
		}
	}

	dbSizeMB, err := ingest.CurrentDBSizeMB(r.Context(), h.db)
	if err != nil {
		slog.Warn("health/detail: failed to read database size", "error", err)
		status = "degraded"
	}
	maxDBSizeMB := h.live.Current().Int(settings.IngestMaxDBSizeMB)

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"service":          "api",
		"repositories":     repoCount,
		"issues":           issueCount,
		"cache":            cacheStatus,
		"db_size_mb":       dbSizeMB,
		"max_db_size_mb":   maxDBSizeMB,
		"ingestion_paused": maxDBSizeMB > 0 && dbSizeMB > int64(maxDBSizeMB),
	})
}
