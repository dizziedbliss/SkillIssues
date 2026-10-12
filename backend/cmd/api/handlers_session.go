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

// authPurge handles POST /auth/purge
func (h *handlers) authPurge(w http.ResponseWriter, r *http.Request) {
	userID, err := h.resolveSession(r)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	_ = h.authSvc.DeleteAllSessions(r.Context(), userID)
	_ = h.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, userID)
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"purged": true})
}

// ---------------------------------------------------------------------------
// Session cookie helpers
// ---------------------------------------------------------------------------

func (h *handlers) setSessionCookie(w http.ResponseWriter, rawToken string) {
	isSecure := strings.HasPrefix(h.cfg.FrontendURL, "https://")
	sameSite := http.SameSiteLaxMode
	if isSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:  "skillissues_session",
		Value: rawToken,
		// Must track the session TTL, or the cookie and the stored session
		// disagree about when the session ends.
		MaxAge:   int(h.sessionTTL().Seconds()),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSite,
		Path:     "/",
	})
}

func (h *handlers) clearSessionCookie(w http.ResponseWriter) {
	isSecure := strings.HasPrefix(h.cfg.FrontendURL, "https://")
	sameSite := http.SameSiteLaxMode
	if isSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "skillissues_session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSite,
		Path:     "/",
	})
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
