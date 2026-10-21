package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dizziedbliss/skillissues/internal/claim"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func claimIssueID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return 0, false
	}
	return id, true
}

// getIssueClaim says whether an issue is claimed, and the caller's claim on it.
func (h *handlers) getIssueClaim(w http.ResponseWriter, r *http.Request) {
	id, ok := claimIssueID(w, r)
	if !ok {
		return
	}
	st, err := h.claimSvc.Of(r.Context(), middleware.GetUserID(r.Context()), id)
	if err != nil {
		slog.Error("read claim", "issue_id", id, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to read the claim")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	middleware.RespondJSON(w, http.StatusOK, st)
}

// claimIssue claims an issue for the caller (or returns their claim on it).
func (h *handlers) claimIssue(w http.ResponseWriter, r *http.Request) {
	id, ok := claimIssueID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())
	cfg := h.live.Current()
	c, err := h.claimSvc.Claim(r.Context(), userID, id, cfg.Int(settings.ClaimsDays), cfg.Int(settings.ClaimsMaxActive))
	switch {
	case errors.Is(err, claim.ErrNotFound):
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
	case errors.Is(err, claim.ErrClosed):
		middleware.RespondError(w, http.StatusConflict, "this issue is closed")
	case errors.Is(err, claim.ErrTaken):
		middleware.RespondError(w, http.StatusConflict, "someone else on SkillIssues has claimed this issue")
	case errors.Is(err, claim.ErrTooMany):
		middleware.RespondError(w, http.StatusUnprocessableEntity,
			"you already hold "+strconv.Itoa(cfg.Int(settings.ClaimsMaxActive))+" claims: finish or release one first")
	case err != nil:
		slog.Error("claim issue", "issue_id", id, "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to claim the issue")
	default:
		h.claimed.set(id, true)
		middleware.RespondJSON(w, http.StatusCreated, c)
	}
}

// releaseIssue gives up the caller's claim on an issue.
func (h *handlers) releaseIssue(w http.ResponseWriter, r *http.Request) {
	id, ok := claimIssueID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())
	err := h.claimSvc.Release(r.Context(), userID, id)
	switch {
	case errors.Is(err, claim.ErrNotYours):
		middleware.RespondError(w, http.StatusNotFound, "you hold no claim on this issue")
	case err != nil:
		slog.Error("release claim", "issue_id", id, "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to release the claim")
	default:
		h.claimed.set(id, false)
		w.WriteHeader(http.StatusNoContent)
	}
}

// getMyClaims lists the caller's active claims with their issues.
func (h *handlers) getMyClaims(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	claims, err := h.claimSvc.Mine(r.Context(), userID)
	if err != nil {
		slog.Error("list claims", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list your claims")
		return
	}
	type withIssue struct {
		claim.Claim
		Issue any `json:"issue"`
	}
	out := make([]withIssue, 0, len(claims))
	for _, c := range claims {
		it, err := h.issueSvc.GetIssue(r.Context(), c.IssueID)
		if err != nil {
			slog.Error("list claims: read issue", "issue_id", c.IssueID, "error", err)
			middleware.RespondError(w, http.StatusInternalServerError, "failed to list your claims")
			return
		}
		out = append(out, withIssue{Claim: c, Issue: it})
	}
	w.Header().Set("Cache-Control", "private, no-store")
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"claims": out})
}
