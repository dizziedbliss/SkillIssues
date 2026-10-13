package main

import (
	"encoding/json"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/profile"
)

func (h *handlers) getMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "profile not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, p)
}

func (h *handlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body profile.ProfileUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "invalid profile update body")
		return
	}

	p, err := h.profileSvc.UpdateProfile(r.Context(), userID, body)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	if h.cache != nil {
		h.cache.InvalidateRecommendations(r.Context(), userID)
	}
	middleware.RespondJSON(w, http.StatusOK, p)
}

func (h *handlers) getPreferenceOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.profileSvc.GetPreferenceOptions(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch preference options")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"options": options})
}
