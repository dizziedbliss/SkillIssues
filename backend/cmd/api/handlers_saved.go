package main

import (
	"encoding/json"
	"net/http"
	"strconv"

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
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
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
	_ = h.issueSvc.UnsaveIssue(r.Context(), userID, id)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"saved":    false,
		"issue_id": id,
	})
}
