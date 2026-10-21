package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/progress"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// getPublicProfile is someone's public profile: their level, badges and
// merges, only if they made it public.
func (h *handlers) getPublicProfile(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	p, err := h.progressSvc.PublicProfile(r.Context(), r.PathValue("login"),
		cfg.Int(settings.XPLevelStep), cfg.Int(settings.ProfilesPublicPulls), time.Now())
	switch {
	case errors.Is(err, progress.ErrNotPublic):
		middleware.RespondError(w, http.StatusNotFound, "no public profile by that name")
	case err != nil:
		slog.Error("read public profile", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to read the profile")
	default:
		// Never cached by others: turning the profile off must take effect.
		w.Header().Set("Cache-Control", "no-store")
		middleware.RespondJSON(w, http.StatusOK, p)
	}
}

// getLeaderboard ranks people with public profiles by XP:
// ?period=month|all, ?language=Go. Off until api.leaderboards.enabled.
func (h *handlers) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	if !cfg.Bool(settings.LeaderboardsEnabled) {
		middleware.RespondError(w, http.StatusNotFound, "the leaderboard isn't open yet")
		return
	}
	var since *time.Time
	switch r.URL.Query().Get("period") {
	case "", "month":
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		since = &start
	case "all":
	default:
		middleware.RespondError(w, http.StatusBadRequest, "period must be month or all")
		return
	}
	language := r.URL.Query().Get("language")
	entries, err := h.progressSvc.Leaderboard(r.Context(), since, language, cfg.Int(settings.XPLevelStep), cfg.Int(settings.LeaderboardsSize))
	if err != nil {
		slog.Error("read leaderboard", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to read the leaderboard")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"entries": entries, "since": since})
}
