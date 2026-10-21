package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/middleware"
)

// getGitHubDeviceConfig handles GET /auth/github/device: what the desktop app
// needs to start GitHub's device flow itself. A client id is public (it is in
// every authorize URL); serving it keeps it in one place, this server's config,
// rather than built into each desktop release.
func (h *handlers) getGitHubDeviceConfig(w http.ResponseWriter, r *http.Request) {
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"enabled":   h.cfg.GitHubLoginClientID != "",
		"client_id": h.cfg.GitHubLoginClientID,
	})
}

// authGitHubDevice handles POST /auth/github/device — desktop sign-in.
//
// The desktop app runs GitHub's device flow against the Login app, asking for
// no scopes, and posts the resulting token here once. The server checks with
// GitHub that the token was issued to the Login app, reads the identity it
// belongs to, and drops it. The response is a bearer session; no GitHub token
// is kept. Pushing needs a second device flow, for public_repo, whose token
// stays on the user's machine and never comes here.
// Body: {"access_token": "..."}
func (h *handlers) authGitHubDevice(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubLoginClientID == "" {
		middleware.RespondError(w, http.StatusServiceUnavailable, "sign-in is not configured")
		return
	}

	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccessToken == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "access_token is required")
		return
	}

	ghUser, err := h.ghOAuth.VerifyDeviceToken(r.Context(), body.AccessToken)
	switch {
	case errors.Is(err, auth.ErrInvalidDeviceToken), errors.Is(err, auth.ErrInvalidGitHubIdentity):
		middleware.RespondError(w, http.StatusUnauthorized, "invalid GitHub token")
		return
	case err != nil:
		slog.Error("device sign-in: GitHub token check failed", "error", err)
		middleware.RespondError(w, http.StatusBadGateway, "could not verify the GitHub token")
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.Name, ghUser.AvatarURL)
	if err != nil {
		slog.Error("device sign-in: failed to upsert user", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}
	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_device")
	if err != nil {
		slog.Error("device sign-in: failed to create session", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}

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
