package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/middleware"
)

// authGitHubAppBegin handles GET /auth/github-app — starts the GitHub App's
// user-authorization flow (D5), which mints a user-to-server token for
// fork/push/PR instead of the OAuth App's classic, non-expiring token.
func (h *handlers) authGitHubAppBegin(w http.ResponseWriter, r *http.Request) {
	if h.ghAppLogin == nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"error":   "GitHub App is not configured",
		})
		return
	}

	authorizeURL, _, err := h.ghAppLogin.BeginAuth(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin GitHub App auth: "+err.Error())
		return
	}

	if r.URL.Query().Get("redirect") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, authorizeURL, http.StatusFound)
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"enabled":       true,
		"authorize_url": authorizeURL,
	})
}

// authGitHubAppCallback handles GET /auth/github-app/callback.
func (h *handlers) authGitHubAppCallback(w http.ResponseWriter, r *http.Request) {
	if h.ghAppLogin == nil {
		middleware.RespondError(w, http.StatusNotFound, "GitHub App is not configured")
		return
	}

	frontendURL := h.cfg.FrontendURLPrimary

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		target := frontendURL + "?error=" + url.QueryEscape(errParam)
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	if code == "" || state == "" {
		middleware.RespondError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	userID, _, err := h.ghAppLogin.ExchangeCode(r.Context(), code, state)
	if err != nil {
		slog.Error("github app auth exchange failed", "error", err)
		target := frontendURL + "?error=" + url.QueryEscape("github_app_auth_failed")
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_app")
	if err != nil {
		slog.Error("failed to create session after github app auth", "error", err)
		target := frontendURL + "?error=" + url.QueryEscape("session_failed")
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	h.setSessionCookie(w, rawToken)
	http.Redirect(w, r, frontendURL+"?auth=ok", http.StatusFound)
}
