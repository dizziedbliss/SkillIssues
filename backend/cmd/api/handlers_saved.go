package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/middleware"
)

func (h *handlers) getSaved(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	saved, err := h.issueSvc.GetSavedIssues(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch saved issues")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, saved)
}

func (h *handlers) saveIssue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		IssueID int `json:"issue_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IssueID <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "valid issue_id is required")
		return
	}
	if err := h.issueSvc.SaveIssue(r.Context(), userID, body.IssueID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			middleware.RespondError(w, http.StatusNotFound, "issue not found")
			return
		}
		slog.Error("save issue failed", "user_id", userID, "issue_id", body.IssueID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "could not save the issue")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"saved":    true,
		"issue_id": body.IssueID,
	})
}

func (h *handlers) unsaveIssue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	if err := h.issueSvc.UnsaveIssue(r.Context(), userID, id); err != nil {
		slog.Error("unsave issue failed", "user_id", userID, "issue_id", id, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "could not remove the saved issue")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"saved":    false,
		"issue_id": id,
	})
}
