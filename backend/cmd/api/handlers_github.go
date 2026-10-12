package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	token, _ := h.authSvc.GetAccessToken(r.Context(), userID)
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Please click 'Connect GitHub' in top bar to authorize repository forking.")
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
	token, _ := h.authSvc.GetAccessToken(r.Context(), userID)
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub before pushing changes.")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}
