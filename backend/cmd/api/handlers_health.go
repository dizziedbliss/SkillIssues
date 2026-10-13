package main

import (
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/middleware"
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
// Returns counts of repositories, issues, and contributions.
func (h *handlers) healthDetail(w http.ResponseWriter, r *http.Request) {
	var repoCount, issueCount, contribCount int
	_ = h.db.QueryRow(r.Context(),
		`SELECT
			(SELECT COUNT(*) FROM repositories),
			(SELECT COUNT(*) FROM issues),
			(SELECT COUNT(*) FROM contributions)`,
		nil,
		func(row pgx.Row) error {
			return row.Scan(&repoCount, &issueCount, &contribCount)
		},
	)

	cacheStatus := "unavailable"
	if h.cache != nil {
		if err := h.cache.Ping(r.Context()); err == nil {
			cacheStatus = "ok"
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"service":       "api",
		"repositories":  repoCount,
		"issues":        issueCount,
		"contributions": contribCount,
		"cache":         cacheStatus,
	})
}
