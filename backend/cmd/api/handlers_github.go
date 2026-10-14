package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/middleware"
)

// forkRepository handles POST /github/forks
func (h *handlers) forkRepository(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		Repository string `json:"repository"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Repository == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "valid repository is required")
		return
	}

	owner, repo, ok := strings.Cut(body.Repository, "/")
	if !ok || owner == "" || repo == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "repository must be in owner/name format")
		return
	}

	// Forking attributes the fork to whoever's token performs it, so this must
	// always be the user's own GitHub authorization — never the server's.
	token, ok := h.userGitHubToken(w, r, userID, "Please click 'Connect GitHub' in top bar to authorize repository forking.")
	if !ok {
		return
	}

	fork, err := h.ghClient.CreateFork(r.Context(), token, owner, repo)
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, fmt.Sprintf("failed to fork repository: %v", err))
		return
	}
	middleware.RespondJSON(w, http.StatusOK, fork)
}

// getGitToken handles GET /github/git-token
func (h *handlers) getGitToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// This token drives `git push`, so it must be the user's own GitHub
	// authorization — a server-token fallback would attribute every push
	// made this way to the server's identity, not the contributor's.
	token, ok := h.userGitHubToken(w, r, userID, "Connect GitHub before pushing changes.")
	if !ok {
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// userGitHubToken returns the signed-in user's own GitHub token. When there
// is no usable token it writes the response and returns ok=false: 403 with
// notConnected if the user has to connect (or reconnect) GitHub, 502 if
// loading or refreshing the token failed in a way a retry may fix. It never
// falls back to the server credential.
func (h *handlers) userGitHubToken(w http.ResponseWriter, r *http.Request, userID int, notConnected string) (string, bool) {
	token, err := h.authSvc.GetAccessToken(r.Context(), userID)
	switch {
	case errors.Is(err, auth.ErrGitHubReauthRequired):
		slog.Warn("user GitHub authorization unusable; asking them to reconnect", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusForbidden, notConnected)
		return "", false
	case err != nil:
		slog.Error("failed to load user GitHub token", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusBadGateway, "Could not load your GitHub authorization. Please try again.")
		return "", false
	case token == "":
		middleware.RespondError(w, http.StatusForbidden, notConnected)
		return "", false
	}
	return token, true
}
