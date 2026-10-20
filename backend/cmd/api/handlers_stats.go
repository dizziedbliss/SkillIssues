package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// getStats serves the live catalog counts (issue.Stats): the landing page
// polls it, so it is public and short-lived at the edge. Concurrent misses
// share one read.
func (h *handlers) getStats(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	ctx, cancel := context.WithTimeout(r.Context(), cfg.Millis(settings.IssuesQueryTimeoutMs))
	defer cancel()
	v, err, _ := h.issuesFlight.Do("stats", func() (any, error) {
		stats, err := h.issueSvc.Stats(ctx, cfg.Int(settings.IngestStatsActivityRows))
		if err != nil {
			return nil, err
		}
		return json.Marshal(stats)
	})
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	// Public counts with no credentials involved: any site may read them (the
	// flow visualizer is on another origin), and one header suits every
	// cached copy.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Del("Access-Control-Allow-Credentials")
	maxAge := cfg.Int(settings.StatsMaxAgeSeconds)
	respondPublic(w, r, v.([]byte), maxAge, maxAge)
}

// lookupRepository answers whether owner/repo (?name=) is indexed, and how
// many open issues it has listed.
func (h *handlers) lookupRepository(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	ctx, cancel := context.WithTimeout(r.Context(), cfg.Millis(settings.IssuesQueryTimeoutMs))
	defer cancel()
	found, err := h.issueSvc.LookupRepository(ctx, r.URL.Query().Get("name"))
	if errors.Is(err, issue.ErrBadRepositoryName) {
		middleware.RespondError(w, http.StatusBadRequest, "name must be owner/repo")
		return
	}
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	data, err := json.Marshal(found)
	if err != nil {
		slog.Error("encoding a repository lookup failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to encode the lookup")
		return
	}
	respondPublic(w, r, data, cfg.Int(settings.CachePublicMaxAgeSeconds), cfg.Int(settings.CachePublicStaleSeconds))
}
