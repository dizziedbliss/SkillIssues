package main

import (
	"context"
	"fmt"
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

// authLogout handles POST /auth/logout
func (h *handlers) authLogout(w http.ResponseWriter, r *http.Request) {
	rawToken := h.extractToken(r)
	if rawToken != "" {
		_ = h.authSvc.DeleteSession(r.Context(), rawToken)
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

// authPurge handles POST /auth/purge. auth_sessions.user_id cascades on
// users.id (00001_initial_schema.sql), so deleting the user row alone is
// atomic and complete — no separate DeleteAllSessions call, and no
// transaction needed for a single statement. A failed delete must be
// reported: a client that believes the account is gone while it still
// exists server-side is worse than a visible error (CLAUDE.md).
func (h *handlers) authPurge(w http.ResponseWriter, r *http.Request) {
	userID, err := h.resolveSession(r)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := h.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"purged": true})
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

func (h *handlers) setSessionCookie(w http.ResponseWriter, rawToken string) {
	isSecure := strings.HasPrefix(h.cfg.FrontendURLPrimary, "https://")
	cookie := &http.Cookie{
		Name:  "skillissues_session",
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
		Name:     "skillissues_session",
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
	if c, err := r.Cookie("skillissues_session"); err == nil {
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
		`SELECT username, COALESCE(avatar_url, '') FROM users WHERE id = $1`,
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
