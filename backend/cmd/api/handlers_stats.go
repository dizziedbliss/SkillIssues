package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// statsCache keeps the last GET /stats answer in memory for its public
// lifetime. The route is exempt from the rate limiter (the landing page polls
// it), so this is what bounds its cost: one database read per lifetime,
// however often and by however many it is asked.
type statsCache struct {
	mu   sync.Mutex
	data []byte
	at   time.Time
}

// minStatsLife keeps the cache on when api.stats.max_age_seconds is 0 (no
// caching at the edge): the database still sees at most one read a second.
const minStatsLife = time.Second

func (c *statsCache) get(life time.Duration, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil || now.Sub(c.at) >= max(life, minStatsLife) {
		return nil, false
	}
	return c.data, true
}

func (c *statsCache) put(data []byte, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data, c.at = data, now
}

// getStats serves the live catalog counts (issue.Stats): the landing page
// polls it, so it is public, short-lived at the edge, and kept in memory
// (statsCache). Concurrent misses share one read.
func (h *handlers) getStats(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	maxAge := cfg.Int(settings.StatsMaxAgeSeconds)
	life := time.Duration(maxAge) * time.Second
	data, ok := h.statsCache.get(life, time.Now())
	if !ok {
		ctx, cancel := context.WithTimeout(r.Context(), cfg.Millis(settings.IssuesQueryTimeoutMs))
		defer cancel()
		v, err, _ := h.issuesFlight.Do("stats", func() (any, error) {
			stats, err := h.issueSvc.Stats(ctx, cfg.Int(settings.IngestStatsActivityRows))
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(stats)
			if err != nil {
				return nil, err
			}
			h.statsCache.put(b, time.Now())
			return b, nil
		})
		if err != nil {
			respondIssueQueryError(w, r, err)
			return
		}
		data = v.([]byte)
	}
	// Public counts with no credentials involved: any site may read them (the
	// flow visualizer is on another origin), and one header suits every
	// cached copy.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Del("Access-Control-Allow-Credentials")
	respondPublic(w, r, data, maxAge, maxAge)
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
