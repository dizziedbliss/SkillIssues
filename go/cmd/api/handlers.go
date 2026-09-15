// handlers.go — HTTP handler implementations for the SkillIssues Go API.
//
// Each handler function corresponds 1:1 with a Python FastAPI endpoint.
// Response shapes are identical to the Python implementation to preserve
// client compatibility (web frontend + Electron).
//
// Phase 1 provides: health, auth (all variants), and stub implementations
// for remaining endpoints that proxy to the Python services during transition.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/jackc/pgx/v5"
)

// handlers bundles all service dependencies for the handler functions.
type handlers struct {
	cfg     *config.Config
	db      *db.Pool
	cache   *cache.Client
	authSvc *auth.Service
	ghOAuth *auth.GitHubOAuth
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// health handles GET /health
// Response: {"status":"ok","service":"api"}
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	// Run a basic DB check matching the Python implementation
	if err := h.db.Ping(r.Context()); err != nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "api",
	})
}

// healthDetail handles GET /health/detail
// Returns counts of repositories, issues, and contributions.
func (h *handlers) healthDetail(w http.ResponseWriter, r *http.Request) {
	var repoCount, issueCount, contribCount int
	_ = h.db.QueryRow(r.Context(),
		`SELECT
			(SELECT COUNT(*) FROM repositories),
			(SELECT COUNT(*) FROM issues),
			(SELECT COUNT(*) FROM contributions)`,
		nil,
		func(row pgx.Row) error {
			return row.Scan(&repoCount, &issueCount, &contribCount)
		},
	)

	cacheStatus := "unavailable"
	if h.cache != nil {
		if err := h.cache.Ping(r.Context()); err == nil {
			cacheStatus = "ok"
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"service":      "api",
		"repositories": repoCount,
		"issues":       issueCount,
		"contributions": contribCount,
		"cache":        cacheStatus,
	})
}

// ---------------------------------------------------------------------------
// Auth: Session
// ---------------------------------------------------------------------------

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
		if h.cache != nil {
			h.cache.DeleteSession(r.Context(), auth.HashedToken(rawToken))
		}
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
	// Also delete the user record unless it's the demo user
	var githubID string
	_ = h.db.QueryRow(r.Context(), `SELECT github_id FROM users WHERE id = $1`, []any{userID},
		func(row pgx.Row) error { return row.Scan(&githubID) })
	if githubID != "demo-1" {
		_ = h.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, userID)
	}
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"purged": true})
}

// ---------------------------------------------------------------------------
// Auth: GitHub PAT
// ---------------------------------------------------------------------------

// authGitHubToken handles POST /auth/github/token (PAT login)
// Body: {"token": "ghp_..."}
func (h *handlers) authGitHubToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Token) < 5 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "token is required (min 5 characters)")
		return
	}

	ghUser, err := h.ghOAuth.ValidatePAT(r.Context(), body.Token)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "invalid GitHub token")
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, body.Token)
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

// ---------------------------------------------------------------------------
// Auth: GitHub OAuth PKCE
// ---------------------------------------------------------------------------

// authGitHubBegin handles GET|POST /auth/github
func (h *handlers) authGitHubBegin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubClientID == "" {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{
			"enabled":       false,
			"authorize_url": "",
			"redirect_uri":  h.cfg.GitHubRedirectURI,
		})
		return
	}

	authorizeURL, _, err := h.ghOAuth.BeginOAuth(r.Context(), h.cfg.GitHubRedirectURI)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin OAuth flow")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"enabled":       true,
		"authorize_url": authorizeURL,
		"redirect_uri":  h.cfg.GitHubRedirectURI,
	})
}

// authGitHubCallback handles GET /auth/github/callback
func (h *handlers) authGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		http.Redirect(w, r, h.cfg.FrontendURL+"?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	ghToken, ghUser, err := h.ghOAuth.ExchangeOAuthCode(r.Context(), code, state)
	if err != nil {
		http.Redirect(w, r, h.cfg.FrontendURL+"?error=oauth_failed", http.StatusFound)
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, ghToken)
	if err != nil {
		http.Redirect(w, r, h.cfg.FrontendURL+"?error=user_creation_failed", http.StatusFound)
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_oauth")
	if err != nil {
		http.Redirect(w, r, h.cfg.FrontendURL+"?error=session_failed", http.StatusFound)
		return
	}

	h.setSessionCookie(w, rawToken)
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

// ---------------------------------------------------------------------------
// Auth: Demo mode
// ---------------------------------------------------------------------------

// authDemo handles POST /auth/demo (only registered when DemoMode=true)
func (h *handlers) authDemo(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.UpsertUser(r.Context(), "demo-1", "alex-dev", "https://github.com/identicons/alex-dev.png", "")
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "demo login failed")
		return
	}
	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "demo")
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "session creation failed")
		return
	}
	h.setSessionCookie(w, rawToken)

	user, _ := h.getUserBasic(r.Context(), userID)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"user": user})
}

// ---------------------------------------------------------------------------
// GitHub integration: forks and git-token
// ---------------------------------------------------------------------------

// forkRepository handles POST /github/forks
// Proxied to Python during migration. Full Go port in Phase 2.
func (h *handlers) forkRepository(w http.ResponseWriter, r *http.Request) {
	h.proxyToPython(w, r, "/github/forks")
}

// getGitToken handles GET /github/git-token
func (h *handlers) getGitToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Fetch the user's stored GitHub token
	var token string
	_ = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(github_access_token, '') FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&token) },
	)

	// Fall back to server GITHUB_TOKEN if user has no stored token
	if token == "" {
		token = h.cfg.GitHubToken
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]string{"token": token})
}

// ---------------------------------------------------------------------------
// Issues, saved, recommendations, profile, contributions, webhook, admin
// All proxied to Python API during Phase 1 migration.
// Full Go implementations land in Phases 2–4.
// ---------------------------------------------------------------------------

func (h *handlers) syncIssues(w http.ResponseWriter, r *http.Request)         { h.proxyToPython(w, r, "/issues/sync") }
func (h *handlers) listIssues(w http.ResponseWriter, r *http.Request)          { h.proxyToPython(w, r, "/issues"+queryString(r)) }
func (h *handlers) getIssue(w http.ResponseWriter, r *http.Request)            { h.proxyToPython(w, r, "/issues/"+r.PathValue("id")) }
func (h *handlers) getSaved(w http.ResponseWriter, r *http.Request)            { h.proxyToPython(w, r, "/saved") }
func (h *handlers) saveIssue(w http.ResponseWriter, r *http.Request)           { h.proxyToPython(w, r, "/saved") }
func (h *handlers) unsaveIssue(w http.ResponseWriter, r *http.Request)         { h.proxyToPython(w, r, "/saved/"+r.PathValue("id")) }
func (h *handlers) getRecommendations(w http.ResponseWriter, r *http.Request)  { h.proxyToPython(w, r, "/recommendations"+queryString(r)) }
func (h *handlers) getMe(w http.ResponseWriter, r *http.Request)               { h.proxyToPython(w, r, "/me") }
func (h *handlers) updateProfile(w http.ResponseWriter, r *http.Request)       { h.proxyToPython(w, r, "/profile") }
func (h *handlers) getPreferences(w http.ResponseWriter, r *http.Request)      { h.proxyToPython(w, r, "/preferences") }
func (h *handlers) updatePreferences(w http.ResponseWriter, r *http.Request)   { h.proxyToPython(w, r, "/preferences") }
func (h *handlers) getPreferenceOptions(w http.ResponseWriter, r *http.Request){ h.proxyToPython(w, r, "/preferences/options") }
func (h *handlers) getProgress(w http.ResponseWriter, r *http.Request)         { h.proxyToPython(w, r, "/progress") }
func (h *handlers) startContribution(w http.ResponseWriter, r *http.Request)   { h.proxyToPython(w, r, "/contributions") }
func (h *handlers) listContributions(w http.ResponseWriter, r *http.Request)   { h.proxyToPython(w, r, "/contributions") }
func (h *handlers) getWorking(w http.ResponseWriter, r *http.Request)          { h.proxyToPython(w, r, "/working") }
func (h *handlers) getContribution(w http.ResponseWriter, r *http.Request)     { h.proxyToPython(w, r, "/contributions/"+r.PathValue("id")) }
func (h *handlers) updateContribution(w http.ResponseWriter, r *http.Request)  { h.proxyToPython(w, r, "/contributions/"+r.PathValue("id")) }
func (h *handlers) submitContribution(w http.ResponseWriter, r *http.Request)  { h.proxyToPython(w, r, "/contributions/"+r.PathValue("id")+"/submit") }
func (h *handlers) verifyContribution(w http.ResponseWriter, r *http.Request)  { h.proxyToPython(w, r, "/contributions/"+r.PathValue("id")+"/verify") }
func (h *handlers) webhookGitHub(w http.ResponseWriter, r *http.Request)       { h.proxyToPython(w, r, "/webhooks/github") }
func (h *handlers) adminSyncRepository(w http.ResponseWriter, r *http.Request) { h.proxyToPython(w, r, "/admin/sync/repository/"+r.PathValue("id")) }

// ---------------------------------------------------------------------------
// Python proxy (transition layer)
// ---------------------------------------------------------------------------

// pythonAPIBase is the upstream Python API URL.
// Points to the Python api service during the migration period.
func (h *handlers) pythonAPIBase() string {
	base := os.Getenv("PYTHON_API_URL")
	if base == "" {
		base = "http://api:8000" // default: Docker service name
	}
	return strings.TrimRight(base, "/")
}

// proxyToPython forwards the request to the Python API and pipes the response back.
// Auth headers/cookies are forwarded transparently so session validation works.
func (h *handlers) proxyToPython(w http.ResponseWriter, r *http.Request, path string) {
	target := h.pythonAPIBase() + path

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, "proxy error")
		return
	}

	// Forward relevant headers
	for _, hdr := range []string{"Content-Type", "Authorization", "Cookie", "X-Request-ID"} {
		if v := r.Header.Get(hdr); v != "" {
			proxyReq.Header.Set(hdr, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, fmt.Sprintf("upstream unavailable: %v", err))
		return
	}
	defer resp.Body.Close()

	// Forward response headers (Set-Cookie important for OAuth flow)
	for key, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// ---------------------------------------------------------------------------
// Session cookie helpers
// ---------------------------------------------------------------------------

func (h *handlers) setSessionCookie(w http.ResponseWriter, rawToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "skillissues_session",
		Value:    rawToken,
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (h *handlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "skillissues_session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
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
	if h.cache != nil {
		hashed := auth.HashedToken(rawToken)
		if uid := h.cache.GetSession(r.Context(), hashed); uid != 0 {
			return uid, nil
		}
	}
	return h.authSvc.ValidateSession(r.Context(), rawToken)
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

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

// queryString returns the raw query string portion of the URL, with leading "?".
func queryString(r *http.Request) string {
	if r.URL.RawQuery != "" {
		return "?" + r.URL.RawQuery
	}
	return ""
}

// ensure time import is used (in setSessionCookie)
var _ = strconv.Itoa
var _ = time.Second
