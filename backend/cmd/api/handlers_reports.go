package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/report"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// createReport records {"type", "id", "reason", "note"} from a signed-in
// person: 201, 400 (a reason or type that doesn't exist), 404 (no such
// target, or a theme that isn't public), 409 (already reported) or 429.
func (h *handlers) createReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, `body must be {"type", "id", "reason", "note"}`)
		return
	}
	userID := middleware.GetUserID(r.Context())
	cfg := h.live.Current()
	err := h.reportSvc.Create(r.Context(), userID, report.Target{Type: body.Type, ID: body.ID}, body.Reason, body.Note,
		cfg.Int(settings.ReportsPerUserPerDay))
	switch {
	case err == nil:
		middleware.RespondJSON(w, http.StatusCreated, map[string]bool{"reported": true})
	case errors.Is(err, report.ErrInvalid):
		middleware.RespondError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), report.ErrInvalid.Error()+": "))
	case errors.Is(err, report.ErrNotFound):
		middleware.RespondError(w, http.StatusNotFound, "there is nothing to report there")
	case errors.Is(err, report.ErrDuplicate):
		middleware.RespondError(w, http.StatusConflict, "you have already reported this; it is waiting for review")
	case errors.Is(err, report.ErrTooMany):
		middleware.RespondError(w, http.StatusTooManyRequests, "you have made as many reports as one day allows; try again tomorrow")
	default:
		slog.Error("report failed", "user_id", userID, "type", body.Type, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "the report could not be saved")
	}
}

// listPublicThemes is a page of the community listing: ?sort=popular|newest,
// ?search=, ?page=. Themes reports have hidden are left out.
func (h *handlers) listPublicThemes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort := q.Get("sort")
	if sort != "" && sort != "popular" && sort != "newest" {
		middleware.RespondError(w, http.StatusBadRequest, "sort must be popular or newest")
		return
	}
	page := 1
	if p := q.Get("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			middleware.RespondError(w, http.StatusBadRequest, "page must be a whole number, 1 or more")
			return
		}
		page = n
	}
	size := h.live.Current().Int(settings.ThemesPublicPageSize)
	themes, err := h.themeSvc.ListPublic(r.Context(), middleware.GetUserID(r.Context()), sort, q.Get("search"), size, (page-1)*size,
		h.hidden.get().Themes)
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, map[string]any{"themes": themes, "page": page, "more": len(themes) == size})
}

// adminListReports lists every target with open reports, most reported first.
func (h *handlers) adminListReports(w http.ResponseWriter, r *http.Request) {
	open, err := h.reportSvc.Open(r.Context())
	if err != nil {
		slog.Error("listing reports failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list reports")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"reports": open})
}

// adminResolveReport closes a target's open reports: {"type", "id",
// "action": "uphold" | "dismiss"}. Upholding acts on the target (report.Resolve).
func (h *handlers) adminResolveReport(w http.ResponseWriter, r *http.Request) {
	h.resolveReport(w, r)
}

// moderator answers 404 to anyone who isn't one, so the routes don't say they exist.
func (h *handlers) moderator(w http.ResponseWriter, r *http.Request) bool {
	ok, err := h.profileSvc.IsModerator(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		slog.Error("reading moderator flag failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to check your account")
		return false
	}
	if !ok {
		middleware.RespondError(w, http.StatusNotFound, "not found")
		return false
	}
	return true
}

// moderationReports is GET /admin/reports for a moderator signed in to the app.
func (h *handlers) moderationReports(w http.ResponseWriter, r *http.Request) {
	if h.moderator(w, r) {
		w.Header().Set("Cache-Control", "private, no-store")
		h.adminListReports(w, r)
	}
}

// moderationResolve is POST /admin/reports/resolve for a moderator signed in to the app.
func (h *handlers) moderationResolve(w http.ResponseWriter, r *http.Request) {
	if h.moderator(w, r) {
		slog.Info("moderator resolved reports", "user_id", middleware.GetUserID(r.Context()))
		h.resolveReport(w, r)
	}
}

// resolveReport closes a target's open reports and re-reads the hidden set,
// so the decision shows in lists at once rather than at the next refresh.
func (h *handlers) resolveReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Action != "uphold" && body.Action != "dismiss") {
		middleware.RespondError(w, http.StatusUnprocessableEntity, `body must be {"type", "id", "action": "uphold" | "dismiss"}`)
		return
	}
	err := h.reportSvc.Resolve(r.Context(), report.Target{Type: body.Type, ID: body.ID}, body.Action == "uphold")
	if errors.Is(err, report.ErrNotFound) {
		middleware.RespondError(w, http.StatusNotFound, "no open reports for that target")
		return
	}
	if err != nil {
		slog.Error("resolving reports failed", "type", body.Type, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to resolve the reports")
		return
	}
	// Resolved either way; a failed re-read is logged and counted, and the
	// periodic refresh catches up.
	_ = h.hidden.refresh(r.Context(), h.reportSvc, h.live.Current())
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"resolved": true, "action": body.Action})
}
