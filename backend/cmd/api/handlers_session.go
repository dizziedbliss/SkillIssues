package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/jackc/pgx/v5"
)

// getSession handles GET /auth/session
func (h *handlers) getSession(w http.ResponseWriter, r *http.Request) {
	// Attempt to resolve session without hard-failing
	userID, err := h.resolveSession(r)
	if err != nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}

	user, err := h.getUserBasic(r.Context(), userID)
	if err != nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          user,
	})
}

// authLogout handles POST /auth/logout. A failed delete is reported: a
// client told it is signed out while its session still works is worse than a
// visible error.
func (h *handlers) authLogout(w http.ResponseWriter, r *http.Request) {
	rawToken := h.extractToken(r)
	if rawToken != "" {
		if err := h.authSvc.DeleteSession(r.Context(), rawToken); err != nil {
			slog.Error("logout: failed to delete session", "error", err)
			middleware.RespondError(w, http.StatusInternalServerError, "failed to sign out")
			return
		}
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

// authLogoutAll handles POST /auth/logout-all: ends every session of the
// signed-in user, on every device.
func (h *handlers) authLogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, err := h.resolveSession(r)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := h.authSvc.DeleteAllSessions(r.Context(), userID); err != nil {
		slog.Error("logout-all: failed to delete sessions", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to sign out everywhere")
		return
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

// githubAuthorizedAppsURL is where a user revokes the Login app's grant.
// Revoking there does not end a SkillIssues session: the server holds no
// GitHub token to invalidate, so a session ends only on logout, expiry,
// "log out everywhere" or purge.
const githubAuthorizedAppsURL = "https://github.com/settings/applications"

// authPurge handles POST /auth/purge. auth_sessions.user_id cascades on
// users.id (00001_initial_schema.sql), so deleting the user row alone is
// atomic and complete. The body must be {"confirm": true}: it makes the
// request a JSON POST a cross-site form cannot produce, and an explicit
// statement that the caller means to delete the account. A failed delete is
// reported: a client that believes the account is gone while it still exists
// server-side is worse than a visible error (CLAUDE.md).
func (h *handlers) authPurge(w http.ResponseWriter, r *http.Request) {
	userID, err := h.resolveSession(r)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var body struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
		middleware.RespondError(w, http.StatusUnprocessableEntity, `body must be {"confirm": true}`)
		return
	}
	if err := h.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
		slog.Error("purge: failed to delete account", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"purged":              true,
		"authorized_apps_url": githubAuthorizedAppsURL,
	})
}

// ---------------------------------------------------------------------------
// Session cookie helpers
// ---------------------------------------------------------------------------

// sessionCookieSameSite is Lax in every deployment. SameSite compares
// sites (registrable domains), not origins, so Lax covers the same-origin
// production deploy and a frontend/API subdomain split (SESSION_COOKIE_DOMAIN)
// alike, and still sends the cookie on the top-level GET of an OAuth
// callback. None would only be needed across registrable domains, and would
// expose the session to cross-site requests for no benefit. The desktop app
// authenticates with a bearer token, not this cookie.
const sessionCookieSameSite = http.SameSiteLaxMode

// sessionCookieName is the cookie a browser session travels in.
const sessionCookieName = "skillissues_session"

func (h *handlers) setSessionCookie(w http.ResponseWriter, rawToken string) {
	isSecure := strings.HasPrefix(h.cfg.FrontendURLPrimary, "https://")
	cookie := &http.Cookie{
		Name:  sessionCookieName,
		Value: rawToken,
		// Must track the session TTL, or the cookie and the stored session
		// disagree about when the session ends.
		MaxAge:   int(h.sessionTTL().Seconds()),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sessionCookieSameSite,
		Path:     "/",
	}
	if h.cfg.SessionCookieDomain != "" {
		cookie.Domain = h.cfg.SessionCookieDomain
	}
	http.SetCookie(w, cookie)
}

func (h *handlers) clearSessionCookie(w http.ResponseWriter) {
	isSecure := strings.HasPrefix(h.cfg.FrontendURLPrimary, "https://")
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sessionCookieSameSite,
		Path:     "/",
	}
	if h.cfg.SessionCookieDomain != "" {
		cookie.Domain = h.cfg.SessionCookieDomain
	}
	http.SetCookie(w, cookie)
}

func (h *handlers) extractToken(r *http.Request) string {
	if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimPrefix(bearer, "Bearer ")
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		return c.Value
	}
	return ""
}

func (h *handlers) resolveSession(r *http.Request) (int, error) {
	rawToken := h.extractToken(r)
	if rawToken == "" {
		return 0, fmt.Errorf("no token")
	}
	return h.authSvc.ValidateSession(r.Context(), rawToken)
}

// getUserBasic returns a minimal user record for session responses.
func (h *handlers) getUserBasic(ctx context.Context, userID int) (map[string]any, error) {
	var username, avatarURL string
	err := h.db.QueryRow(ctx,
		`SELECT COALESCE(username, ''), COALESCE(avatar_url, '') FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(&username, &avatarURL)
		},
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":         userID,
		"username":   username,
		"avatar_url": avatarURL,
	}, nil
}
