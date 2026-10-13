package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// authGitHubToken handles POST /auth/github/token (Classic PAT login only).
// Both clients only expose OAuth now; this stays registered but 404s unless
// auth.pat_login_enabled is explicitly turned on.
// Body: {"token": "ghp_..."}
func (h *handlers) authGitHubToken(w http.ResponseWriter, r *http.Request) {
	if h.live == nil || !h.live.Current().Bool(settings.AuthPATLoginEnabled) {
		middleware.RespondError(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(strings.TrimSpace(body.Token)) < 5 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "token is required")
		return
	}

	cleanTok := strings.TrimSpace(body.Token)
	if !strings.HasPrefix(cleanTok, "ghp_") {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "Only GitHub Classic Personal Access Tokens (starting with 'ghp_') or OAuth are supported.")
		return
	}

	ghUser, err := h.ghOAuth.ValidatePAT(r.Context(), cleanTok)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "Invalid GitHub token: please ensure it has 'repo' and 'read:user' scopes.")
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, cleanTok)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to create user record")
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_pat")
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.setSessionCookie(w, rawToken)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user": map[string]any{
			"id":         userID,
			"username":   ghUser.Username,
			"avatar_url": ghUser.AvatarURL,
		},
		"session_token": rawToken,
	})
}
