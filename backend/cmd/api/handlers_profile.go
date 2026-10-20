package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/profile"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func (h *handlers) getMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		// The session was valid (requireAuth), so a failure here is ours, not the caller's.
		slog.Error("get profile failed", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load the profile")
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

	cfg := h.live.Current()
	p, err := h.profileSvc.UpdateProfile(r.Context(), userID, body, profile.Limits{
		BioMaxChars:  cfg.Int(settings.ProfileBioMaxChars),
		MaxLanguages: cfg.Int(settings.ProfileMaxLanguages),
		MaxInterests: cfg.Int(settings.ProfileMaxInterests),
	})
	if errors.Is(err, profile.ErrInvalid) {
		middleware.RespondError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), profile.ErrInvalid.Error()+": "))
		return
	}
	if err != nil {
		slog.Error("update profile failed", "user_id", userID, "error", err)
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
		slog.Error("preference options failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch preference options")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"options": options})
}
