package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/progress"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/xp"
)

// getMyXP is the caller's level, streak, badges and languages, their latest
// XP, and where tracking of their pull requests stands.
func (h *handlers) getMyXP(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	cfg := h.live.Current()
	events, err := h.progressSvc.Events(r.Context(), userID)
	if err != nil {
		slog.Error("read xp ledger", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load your XP")
		return
	}
	recent, err := h.progressSvc.Recent(r.Context(), userID, cfg.Int(settings.XPRecentLimit))
	if err != nil {
		slog.Error("read recent xp", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load your XP")
		return
	}
	sync, err := h.progressSvc.SyncOf(r.Context(), userID)
	if err != nil {
		slog.Error("read pr sync", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load your XP")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"summary":    xp.Summarise(events, cfg.Int(settings.XPLevelStep), time.Now()),
		"recent":     recent,
		"sync":       sync,
		"badge_keys": xp.BadgeKeys(),
		"quests":     xp.Quests(events),
	})
}

// getMyPulls is the caller's tracked pull requests; ?state=open for open ones.
func (h *handlers) getMyPulls(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	openOnly := false
	switch r.URL.Query().Get("state") {
	case "", "all":
	case "open":
		openOnly = true
	default:
		middleware.RespondError(w, http.StatusBadRequest, "state must be open or all")
		return
	}
	pulls, err := h.progressSvc.Pulls(r.Context(), userID, openOnly, h.live.Current().Int(settings.PRsPageSize))
	if err != nil {
		slog.Error("read pull requests", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load your pull requests")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"pull_requests": pulls})
}

// refreshMyPulls asks ingestd to read the caller's pull requests on its next
// cycle, at most once per api.prs.refresh_min_seconds.
func (h *handlers) refreshMyPulls(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	gap := h.live.Current().Seconds(settings.PRsRefreshMinSeconds)
	next, err := h.progressSvc.RequestCheck(r.Context(), userID, gap)
	if errors.Is(err, progress.ErrTooSoon) {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(time.Until(next).Seconds()))))
		middleware.RespondJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "your pull requests were checked moments ago", "next_allowed_at": next,
		})
		return
	}
	if err != nil {
		slog.Error("request pr check", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to ask for a check")
		return
	}
	middleware.RespondJSON(w, http.StatusAccepted, map[string]any{"next_allowed_at": next})
}
