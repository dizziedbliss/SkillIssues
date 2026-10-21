package main

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/middleware"
)

// oauthStateCookie binds an OAuth round trip to the browser that began it.
// The __Host- prefix makes browsers refuse it unless it is Secure, Path=/ and
// has no Domain, so a sibling subdomain cannot plant one.
const oauthStateCookie = "__Host-oauth_state"

// oauthStateMaxAge matches the lifetime of the oauth_states row.
const oauthStateMaxAge = 10 * 60

// authGitHubBegin handles GET /auth/github. It sends the browser to the Login
// app's authorize page (or returns the URL as JSON) and sets the state cookie
// the callback will require.
func (h *handlers) authGitHubBegin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubLoginClientID == "" {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{
			"enabled":       false,
			"authorize_url": "",
			"redirect_uri":  h.cfg.GitHubRedirectURI,
			"error":         "GITHUB_LOGIN_CLIENT_ID is not configured",
		})
		return
	}

	authorizeURL, state, err := h.ghOAuth.BeginOAuth(r.Context(), h.cfg.GitHubRedirectURI)
	if err != nil {
		slog.Error("failed to begin OAuth flow", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin sign-in")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    auth.StateBinding(state),
		MaxAge:   oauthStateMaxAge,
		HttpOnly: true,
		// __Host- requires Secure. Browsers treat localhost as a secure
		// context for cookies, so this also works in local development.
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	if r.URL.Query().Get("redirect") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, authorizeURL, http.StatusFound)
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"enabled":       true,
		"authorize_url": authorizeURL,
		"redirect_uri":  h.cfg.GitHubRedirectURI,
	})
}

// authGitHubCallback handles GET /auth/github/callback.
//
// Every failure redirects to the frontend with a short error code from a
// fixed set. Text GitHub or an internal error supplies is never put in the
// redirect: it would be attacker-influenced content reflected into a page.
func (h *handlers) authGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	frontendURL := h.cfg.FrontendURLPrimary

	// The state cookie is single-use whatever happens next.
	cookie, cookieErr := r.Cookie(oauthStateCookie)
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: "", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Path: "/",
	})

	fail := func(code string) {
		sep := "?"
		if strings.Contains(frontendURL, "?") {
			sep = "&"
		}
		http.Redirect(w, r, frontendURL+sep+"error="+url.QueryEscape(code), http.StatusFound)
	}

	if r.URL.Query().Get("error") != "" {
		// The user declined, or GitHub refused. Which one is not worth
		// reflecting; the client shows one message for both.
		fail("access_denied")
		return
	}
	if code == "" || state == "" {
		fail("invalid_callback")
		return
	}
	// A callback with no matching cookie did not start in this browser: a
	// replayed or planted URL. Refused before the state row is consumed so
	// the real browser's attempt is unaffected.
	if cookieErr != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(auth.StateBinding(state))) != 1 {
		fail("invalid_callback")
		return
	}

	ghUser, err := h.ghOAuth.ExchangeOAuthCode(r.Context(), code, state)
	if err != nil {
		slog.Error("failed to exchange oauth code", "error", err)
		fail("oauth_failed")
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.Name, ghUser.AvatarURL)
	if err != nil {
		slog.Error("failed to upsert user after oauth", "error", err)
		fail("sign_in_failed")
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_oauth")
	if err != nil {
		slog.Error("failed to create session after oauth", "error", err)
		fail("sign_in_failed")
		return
	}

	// The session cookie is set; redirect without putting the token in the
	// URL (it would leak via history and Referer). ?auth=ok tells the SPA a
	// round trip just finished so it can report a real error instead of
	// silently rendering logged-out if the cookie didn't take.
	h.setSessionCookie(w, rawToken)
	sep := "?"
	if strings.Contains(frontendURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, frontendURL+sep+"auth=ok", http.StatusFound)
}
