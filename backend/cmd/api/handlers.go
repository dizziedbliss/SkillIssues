// handlers.go — HTTP handler implementations for the SkillIssues API.
//
// Response shapes are consumed by both the web frontend and the Electron
// desktop app, so changing a payload shape requires updating both clients.
// Sync and ingestion requests are forwarded to the ingestion service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/contribution"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/profile"
	"github.com/dizziedbliss/skillissues/internal/recommendation"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ticketTTL bounds how long a desktop OAuth ticket stays claimable. Without
// an expiry, a ticket that's never polled (or polled only once and left
// unconsumed) would hand out a live session to whoever holds it, forever.
const ticketTTL = 10 * time.Minute

// ticketEntry pairs stored ticket data with when it was created, so expired
// entries can be rejected and swept instead of living in memory forever.
type ticketEntry struct {
	data      any
	createdAt time.Time
}

func (t ticketEntry) expired() bool { return time.Since(t.createdAt) > ticketTTL }

// handlers bundles all service dependencies for the handler functions.
type handlers struct {
	cfg            *config.Config
	live           *settings.Live
	db             *db.Pool
	cache          *cache.Client
	authSvc        *auth.Service
	ghOAuth        *auth.GitHubOAuth
	profileSvc     *profile.Service
	issueSvc       *issue.Service
	contribSvc     *contribution.Service
	ghClient       *github.Client
	desktopTickets sync.Map // ticket -> ticketEntry{data: session data}
	stateTickets   sync.Map // state -> ticketEntry{data: ticket string}
}

// sweepExpiredTickets periodically drops stale entries from desktopTickets
// and stateTickets so abandoned OAuth attempts don't accumulate forever.
func (h *handlers) sweepExpiredTickets(ctx context.Context) {
	ticker := time.NewTicker(ticketTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, m := range []*sync.Map{&h.desktopTickets, &h.stateTickets} {
				m.Range(func(key, value any) bool {
					if entry, ok := value.(ticketEntry); ok && entry.expired() {
						m.Delete(key)
					}
					return true
				})
			}
		}
	}
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
		"status":        "ok",
		"service":       "api",
		"repositories":  repoCount,
		"issues":        issueCount,
		"contributions": contribCount,
		"cache":         cacheStatus,
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
	_ = h.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, userID)
	h.clearSessionCookie(w)
	middleware.RespondJSON(w, http.StatusOK, map[string]bool{"purged": true})
}

// ---------------------------------------------------------------------------
// Auth: GitHub PAT
// ---------------------------------------------------------------------------

// authGitHubToken handles POST /auth/github/token (PAT login)
// authGitHubToken handles POST /auth/github/token (Classic PAT login only)
// Body: {"token": "ghp_..."}
func (h *handlers) authGitHubToken(w http.ResponseWriter, r *http.Request) {
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
			"error":         "GITHUB_CLIENT_ID is not configured",
		})
		return
	}

	authorizeURL, state, err := h.ghOAuth.BeginOAuth(r.Context(), h.cfg.GitHubRedirectURI)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin OAuth flow: "+err.Error())
		return
	}

	isDesktop := r.URL.Query().Get("source") == "desktop"
	var ticket string
	if isDesktop {
		ticket = uuid.NewString()
		h.stateTickets.Store(state, ticketEntry{data: ticket, createdAt: time.Now()})
	}

	// If browser directly navigated or explicitly requested redirect
	if r.URL.Query().Get("redirect") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, authorizeURL, http.StatusFound)
		return
	}

	resp := map[string]any{
		"enabled":       true,
		"authorize_url": authorizeURL,
		"redirect_uri":  h.cfg.GitHubRedirectURI,
	}
	if ticket != "" {
		resp["ticket"] = ticket
	}
	middleware.RespondJSON(w, http.StatusOK, resp)
}

// authGitHubCallback handles GET /auth/github/callback
func (h *handlers) authGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	frontendURL := h.cfg.FrontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	var ticket string
	if stateVal, ok := h.stateTickets.LoadAndDelete(state); ok {
		if entry, ok := stateVal.(ticketEntry); ok && !entry.expired() {
			ticket = entry.data.(string)
		}
	}

	redirectWithError := func(code string, detail string) {
		if ticket != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SkillIssues — Authentication Error</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-app: #f7f6fb;
      --text-primary: #1e1338;
      --text-muted: #6b5f88;
      --border-card: #e5e0f2;
      --accent-primary: #241442;
      --accent-primary-hover: #371f63;
    }
    * { box-sizing: border-box; }
    body {
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg-app);
      color: var(--text-primary);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 24px;
      -webkit-font-smoothing: antialiased;
    }
    .card {
      background: #ffffff;
      border: 1px solid var(--border-card);
      border-radius: 24px;
      padding: 44px 36px;
      text-align: center;
      max-width: 440px;
      width: 100%%;
      box-shadow: 0 20px 40px -12px rgba(36, 20, 66, 0.08), 0 2px 6px rgba(36, 20, 66, 0.04);
    }
    .badge-icon {
      width: 60px;
      height: 60px;
      border-radius: 18px;
      background: #fef2f2;
      color: #dc2626;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 20px;
    }
    h2 {
      font-size: 22px;
      font-weight: 800;
      color: var(--text-primary);
      margin: 0 0 12px;
      letter-spacing: -0.02em;
    }
    .err-pill {
      display: inline-block;
      background: #fef2f2;
      color: #b91c1c;
      border: 1px solid #fecaca;
      padding: 8px 16px;
      border-radius: 10px;
      font-size: 13.5px;
      font-weight: 600;
      margin-bottom: 20px;
      word-break: break-word;
      max-width: 100%%;
    }
    p {
      font-size: 14px;
      color: var(--text-muted);
      line-height: 1.6;
      margin: 0;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge-icon">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
      </svg>
    </div>
    <h2>Authentication Failed</h2>
    <div class="err-pill">%s: %s</div>
    <p>You can close this tab and return to the application to try again.</p>
  </div>
</body>
</html>`, html.EscapeString(code), html.EscapeString(detail))
			return
		}
		sep := "?"
		if strings.Contains(frontendURL, "?") {
			sep = "&"
		}
		target := fmt.Sprintf("%s%serror=%s", frontendURL, sep, url.QueryEscape(code))
		if detail != "" {
			target += fmt.Sprintf("&error_description=%s", url.QueryEscape(detail))
		}
		http.Redirect(w, r, target, http.StatusFound)
	}

	if errParam != "" {
		redirectWithError(errParam, errDesc)
		return
	}

	if code == "" || state == "" {
		redirectWithError("missing_code_or_state", "Authorization code or state parameter missing from callback")
		return
	}

	ghToken, ghUser, err := h.ghOAuth.ExchangeOAuthCode(r.Context(), code, state)
	if err != nil {
		slog.Error("failed to exchange oauth code", "error", err)
		redirectWithError("oauth_failed", err.Error())
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, ghToken)
	if err != nil {
		slog.Error("failed to upsert user after oauth", "error", err)
		redirectWithError("user_creation_failed", err.Error())
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_oauth")
	if err != nil {
		slog.Error("failed to create session after oauth", "error", err)
		redirectWithError("session_failed", err.Error())
		return
	}

	h.setSessionCookie(w, rawToken)

	// If this flow originated from Electron Desktop, record ticket & present auto-redirect landing page
	if ticket != "" {
		h.desktopTickets.Store(ticket, ticketEntry{
			createdAt: time.Now(),
			data: map[string]any{
				"session_token": rawToken,
				"user": map[string]any{
					"id":         userID,
					"username":   ghUser.Username,
					"avatar_url": ghUser.AvatarURL,
				},
			},
		})

		deepLink := fmt.Sprintf("skillissues://auth?token=%s&ticket=%s", url.QueryEscape(rawToken), url.QueryEscape(ticket))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SkillIssues — Authorized</title>
  <meta http-equiv="refresh" content="0;url=%s" />
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=Noto+Sans:wght@700;800&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-app: #f7f6fb;
      --text-primary: #1e1338;
      --text-muted: #6b5f88;
      --border-card: #e5e0f2;
      --accent-primary: #241442;
      --accent-primary-hover: #371f63;
      --accent-blue: #7c3aed;
    }
    * { box-sizing: border-box; }
    body {
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg-app);
      color: var(--text-primary);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 24px;
      -webkit-font-smoothing: antialiased;
    }
    .card {
      background: #ffffff;
      border: 1px solid var(--border-card);
      border-radius: 24px;
      padding: 44px 36px;
      text-align: center;
      max-width: 440px;
      width: 100%%;
      box-shadow: 0 20px 40px -12px rgba(36, 20, 66, 0.08), 0 2px 6px rgba(36, 20, 66, 0.04);
    }
    .badge-icon {
      width: 60px;
      height: 60px;
      border-radius: 18px;
      background: #ede9f6;
      color: var(--accent-blue);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 20px;
    }
    h1 {
      font-size: 22px;
      font-weight: 800;
      color: var(--text-primary);
      margin: 0 0 10px;
      letter-spacing: -0.02em;
    }
    p {
      font-size: 14px;
      color: var(--text-muted);
      line-height: 1.6;
      margin: 0 0 28px;
    }
    a.btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      background: var(--accent-primary);
      color: #ffffff;
      text-decoration: none;
      padding: 12px 28px;
      border-radius: 12px;
      font-weight: 600;
      font-size: 14px;
      box-shadow: 0 4px 14px rgba(36, 20, 66, 0.15);
      transition: all 0.15s ease;
    }
    a.btn:hover {
      background: var(--accent-primary-hover);
      transform: translateY(-1px);
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge-icon">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M20 6 9 17l-5-5"/>
      </svg>
    </div>
    <h1>Authorization Successful!</h1>
    <p>You have connected your GitHub account. Redirecting you back to SkillIssues Desktop...</p>
    <a class="btn" href="%s">Open SkillIssues Desktop <span>↗</span></a>
  </div>
  <script>
    setTimeout(function() { window.location.href = %q; }, 100);
  </script>
</body>
</html>`, deepLink, deepLink, deepLink)
		return
	}

	// Web flow: the session cookie was already set above; redirect back to the
	// frontend without putting the token in the URL (it would leak via browser
	// history and the Referer header).
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

// authGitHubPoll handles GET /auth/github/poll?ticket=...
// Used by Electron desktop to verify when browser-based OAuth authorization completes.
func (h *handlers) authGitHubPoll(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "ticket is required")
		return
	}
	val, ok := h.desktopTickets.Load(ticket)
	if !ok {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	entry, ok := val.(ticketEntry)
	if !ok || entry.expired() {
		h.desktopTickets.Delete(ticket)
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	// Consume the ticket: once the desktop app has retrieved the session, the
	// ticket must not be claimable again by anyone who later learns it.
	h.desktopTickets.Delete(ticket)
	data := entry.data.(map[string]any)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"session_token": data["session_token"],
		"user":          data["user"],
	})
}

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
	var token string
	_ = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(github_access_token, '') FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&token) },
	)
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
	var token string
	_ = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(github_access_token, '') FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&token) },
	)
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub before pushing changes.")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// ---------------------------------------------------------------------------
// Issues, saved, recommendations, profile, contributions, webhook, admin
// ---------------------------------------------------------------------------

// proxyToSyncd forwards a POST to the ingestion service and pipes the response back.
func (h *handlers) proxyToSyncd(w http.ResponseWriter, r *http.Request, path string) {
	base := strings.TrimRight(h.cfg.SyncdURL, "/")
	if base == "" {
		middleware.RespondError(w, http.StatusInternalServerError, "ingestion service not configured")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, base+path, nil)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to create sync request")
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, "ingestion service unavailable")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// syncPath builds the ingestion endpoint for an optional repository id.
// The id is parsed as an integer so it can never inject extra path segments.
func syncPath(rawRepoID string) (string, error) {
	if rawRepoID == "" {
		return "/sync/issues", nil
	}
	id, err := strconv.Atoi(rawRepoID)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("invalid repository id")
	}
	return fmt.Sprintf("/sync/priority/%d", id), nil
}

func (h *handlers) syncIssues(w http.ResponseWriter, r *http.Request) {
	path, err := syncPath(r.URL.Query().Get("repo_id"))
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.proxyToSyncd(w, r, path)
}

func (h *handlers) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cfg := h.live.Current()

	// A search gets a larger page than a browse, since results are grouped into
	// categories and each needs enough room to be useful.
	defaultLimit := cfg.Int(settings.IssuesDefaultPageSize)
	if q.Get("search") != "" {
		defaultLimit = cfg.Int(settings.IssuesSearchPageSize)
	}

	limit := defaultLimit
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	page := 1
	if pStr := q.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	f := issue.Filters{
		Difficulty: q.Get("difficulty"),
		Language:   q.Get("language"),
		Search:     q.Get("search"),
		Limit:      limit,
		Page:       page,
	}

	// The hot path is the unfiltered first page at the default size. This
	// previously compared against a literal 50 while the default was 20, so the
	// cache was unreachable; comparing against the configured default fixes it.
	isDefaultHot := f.Difficulty == "" && f.Language == "" && f.Search == "" &&
		f.Limit == cfg.Int(settings.IssuesDefaultPageSize) && f.Page == 1
	userID := middleware.GetUserID(r.Context())

	// Fast path: cached hot issues for unauthenticated or default views
	if isDefaultHot && userID == 0 && h.cache != nil {
		if cached := h.cache.GetHotIssues(r.Context()); cached != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}

	searchKey := ""
	if !isDefaultHot && userID == 0 && h.cache != nil {
		searchKey = cache.CacheSearchKey(f.Difficulty, f.Language, f.Search, f.Limit, f.Page)
		if cached := h.cache.GetCachedSearch(r.Context(), searchKey); cached != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}

	var prefLangs []string
	var userName string
	if userID != 0 {
		if p, err := h.profileSvc.GetFullProfile(r.Context(), userID); err == nil {
			prefLangs = p.PreferredLanguages
			userName = p.Username
		}
	}

	issues, err := h.issueSvc.SearchIssues(r.Context(), f, prefLangs, userName)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to search issues")
		return
	}
	if issues == nil {
		issues = []issue.Issue{}
	}

	// Populate cache. A TTL of zero means caching is switched off for that path.
	if userID == 0 && h.cache != nil {
		hotTTL := cfg.Seconds(settings.CacheHotIssuesTTLSeconds)
		searchTTL := cfg.Seconds(settings.CacheSearchTTLSeconds)

		if data, err := json.Marshal(issues); err == nil {
			if isDefaultHot && hotTTL > 0 {
				h.cache.SetHotIssues(r.Context(), data, hotTTL)
			} else if searchKey != "" && searchTTL > 0 {
				h.cache.SetCachedSearch(r.Context(), searchKey, data, searchTTL)
			}
		}
	}

	middleware.RespondJSON(w, http.StatusOK, issues)
}

func (h *handlers) getIssue(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	it, err := h.issueSvc.GetIssue(r.Context(), id)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, it)
}

func (h *handlers) getSaved(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	saved, err := h.issueSvc.GetSavedIssues(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch saved issues")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, saved)
}

func (h *handlers) saveIssue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		IssueID int `json:"issue_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IssueID <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "valid issue_id is required")
		return
	}
	if err := h.issueSvc.SaveIssue(r.Context(), userID, body.IssueID); err != nil {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"saved":    true,
		"issue_id": body.IssueID,
	})
}

func (h *handlers) unsaveIssue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	_ = h.issueSvc.UnsaveIssue(r.Context(), userID, id)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"saved":    false,
		"issue_id": id,
	})
}

func (h *handlers) getRecommendations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	limit := 3
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Redis Cache lookup for default limit
	if limit == 3 && h.cache != nil {
		if cached := h.cache.GetRecommendations(r.Context(), userID); cached != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}

	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load developer profile")
		return
	}

	candidates, err := h.issueSvc.GetRecommendationCandidates(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load candidate issues")
		return
	}

	scored := recommendation.ScoreCandidates(p, candidates, limit)
	if scored == nil {
		scored = []issue.Issue{}
	}

	if recTTL := h.live.Current().Seconds(settings.CacheRecommendationsTTLSecond); limit == 3 && recTTL > 0 && h.cache != nil {
		if data, err := json.Marshal(scored); err == nil {
			h.cache.SetRecommendations(r.Context(), userID, data, recTTL)
		}
	}

	middleware.RespondJSON(w, http.StatusOK, scored)
}

func (h *handlers) getMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "profile not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, p)
}

func (h *handlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body profile.ProfileUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "invalid profile update body")
		return
	}

	p, err := h.profileSvc.UpdateProfile(r.Context(), userID, body)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	if h.cache != nil {
		h.cache.InvalidateRecommendations(r.Context(), userID)
	}
	middleware.RespondJSON(w, http.StatusOK, p)
}

func (h *handlers) getPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "profile not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"interests":           p.Interests,
		"preferred_languages": p.PreferredLanguages,
	})
}

func (h *handlers) updatePreferences(w http.ResponseWriter, r *http.Request) {
	h.updateProfile(w, r)
}

func (h *handlers) getPreferenceOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.profileSvc.GetPreferenceOptions(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch preference options")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"options": options})
}

func (h *handlers) getProgress(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, "profile not found")
		return
	}

	var contribs []map[string]any
	_ = h.db.QueryRows(r.Context(),
		`SELECT c.id, c.user_id, c.issue_id, COALESCE(c.pr_url, ''), COALESCE(c.pr_number, 0),
		        c.state, c.created_at::text, c.updated_at::text, COALESCE(c.merged_at::text, ''),
		        i.title, i.number, i.difficulty, i.url AS issue_url,
		        COALESCE(i.required_skills, '{}'), COALESCE(i.technologies, '{}'),
		        i.updated_at::text AS issue_updated_at, r.full_name AS repository, COALESCE(r.language, '')
		 FROM contributions c
		 JOIN issues i ON i.id = c.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE c.user_id = $1 ORDER BY c.updated_at DESC`,
		[]any{userID},
		func(rows pgx.Rows) error {
			var id, uID, issueID, prNum, issueNum int
			var prURL, state, createdAt, updatedAt, mergedAt, title, diff, issueURL, issueUpdatedAt, repo, lang string
			var skills, techs []string
			if err := rows.Scan(&id, &uID, &issueID, &prURL, &prNum, &state, &createdAt, &updatedAt, &mergedAt,
				&title, &issueNum, &diff, &issueURL, &skills, &techs, &issueUpdatedAt, &repo, &lang); err != nil {
				return err
			}
			contribs = append(contribs, map[string]any{
				"id":               id,
				"user_id":          uID,
				"issue_id":         issueID,
				"pr_url":           prURL,
				"pr_number":        prNum,
				"state":            state,
				"created_at":       createdAt,
				"updated_at":       updatedAt,
				"merged_at":        mergedAt,
				"title":            title,
				"number":           issueNum,
				"difficulty":       diff,
				"issue_url":        issueURL,
				"required_skills":  skills,
				"technologies":     techs,
				"issue_updated_at": issueUpdatedAt,
				"repository":       repo,
				"language":         lang,
			})
			return nil
		},
	)
	if contribs == nil {
		contribs = []map[string]any{}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"progress_score":      p.ProgressScore,
		"completed_count":     p.CompletedCount,
		"demonstrated_skills": p.DemonstratedSkills,
		"contributions":       contribs,
	})
}
func (h *handlers) startContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		IssueID int `json:"issue_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IssueID <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "valid issue_id is required")
		return
	}

	c, err := h.contribSvc.StartContribution(r.Context(), userID, body.IssueID)
	if err != nil {
		if err == db.ErrNotFound {
			middleware.RespondError(w, http.StatusNotFound, "issue not found")
			return
		}
		middleware.RespondError(w, http.StatusInternalServerError, "failed to start contribution")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) listContributions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	list, err := h.contribSvc.ListContributions(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list contributions")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, list)
}

func (h *handlers) getWorking(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	working, err := h.contribSvc.GetWorking(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list working contributions")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, working)
}

func (h *handlers) getContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	c, err := h.contribSvc.GetContribution(r.Context(), userID, id)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "contribution not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) updateContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	var body struct {
		PRURL    string `json:"pr_url"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PRURL == "" || body.PRNumber <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "pr_url and pr_number are required")
		return
	}

	c, err := h.contribSvc.AttachPR(r.Context(), userID, id, body.PRURL, body.PRNumber, h.cfg.GitHubToken)
	if err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) submitContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	var body struct {
		Branch string `json:"branch"`
		Base   string `json:"base"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Branch == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "branch name is required")
		return
	}

	var (
		issueID, issueNum                               int
		username, userToken, title, issueBody, repoFull string
	)
	err = h.db.QueryRow(r.Context(),
		`SELECT c.issue_id, u.username, COALESCE(u.github_access_token, ''),
		        i.title, COALESCE(i.body, ''), i.number, r.full_name
		 FROM contributions c
		 JOIN users u ON u.id = c.user_id
		 JOIN issues i ON i.id = c.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE c.id = $1 AND c.user_id = $2`,
		[]any{id, userID},
		func(row pgx.Row) error {
			return row.Scan(&issueID, &username, &userToken, &title, &issueBody, &issueNum, &repoFull)
		},
	)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "contribution not found")
		return
	}

	token := userToken
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub before submitting a pull request.")
		return
	}

	owner, name, _ := strings.Cut(repoFull, "/")
	prTitle := title
	prBody := issueBody
	if prBody == "" {
		prBody = fmt.Sprintf("SkillIssues contribution for issue #%d", issueNum)
	}

	// Dynamically determine target repository default branch (e.g. 'main', 'master', 'develop')
	baseBranch := strings.TrimSpace(body.Base)
	if baseBranch == "" {
		if defBranch, err := h.ghClient.GetDefaultBranch(r.Context(), token, owner, name); err == nil && defBranch != "" {
			baseBranch = defBranch
		} else {
			baseBranch = "main"
		}
	}

	var prURL string
	var prNum int
	var pullRequest map[string]any

	{
		head := body.Branch
		if username != "" {
			head = fmt.Sprintf("%s:%s", username, body.Branch)
		}
		res, err := h.ghClient.CreatePullRequest(r.Context(), token, owner, name, prTitle, head, baseBranch, prBody)
		if err == nil {
			prURL = res.HTMLURL
			prNum = res.Number
			pullRequest = map[string]any{"html_url": prURL, "number": prNum, "base": baseBranch}
		}
	}

	if prURL == "" {
		if username != "" {
			prURL = fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s:%s?expand=1", owner, name, baseBranch, username, body.Branch)
		} else {
			prURL = fmt.Sprintf("https://github.com/%s/%s", owner, name)
		}
		prNum = issueNum
		pullRequest = map[string]any{
			"html_url":       prURL,
			"number":         prNum,
			"base":           baseBranch,
			"manual_compare": true,
		}
	}

	var updated contribution.Contribution
	_ = h.db.QueryRow(r.Context(),
		`UPDATE contributions SET pr_url = $1, pr_number = $2, state = 'PR_OPEN', updated_at = now()
		 WHERE id = $3
		 RETURNING id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at`,
		[]any{prURL, prNum, id},
		func(row pgx.Row) error {
			return row.Scan(&updated.ID, &updated.UserID, &updated.IssueID, &updated.PRURL, &updated.PRNumber, &updated.State, &updated.XPAwarded, &updated.CreatedAt, &updated.UpdatedAt, &updated.MergedAt)
		},
	)

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"contribution": updated,
		"pull_request": pullRequest,
		"web_url":      prURL,
	})
}

func (h *handlers) verifyContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}

	res, err := h.contribSvc.VerifyContribution(r.Context(), userID, id, h.cfg.GitHubToken)
	if err != nil {
		if err == db.ErrNotFound {
			middleware.RespondError(w, http.StatusNotFound, "contribution not found")
			return
		}
		middleware.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if res.Verified && res.EarnedXP > 0 && h.cache != nil {
		h.cache.InvalidateRecommendations(r.Context(), userID)
	}

	middleware.RespondJSON(w, http.StatusOK, res)
}

func (h *handlers) webhookGitHub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	delivery := r.Header.Get("X-GitHub-Delivery")
	if delivery == "" {
		delivery = uuid.New().String()
	}
	eventName := r.Header.Get("X-GitHub-Event")
	if eventName == "" {
		eventName = "unknown"
	}

	if h.cfg.GitHubWebhookSecret != "" {
		if !auth.VerifyWebhookSignature([]byte(h.cfg.GitHubWebhookSecret), body, signature) {
			middleware.RespondError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}
	} else {
		middleware.RespondError(w, http.StatusServiceUnavailable, "webhook verification is not configured")
		return
	}

	var insertedDelivery string
	_ = h.db.QueryRow(r.Context(),
		`INSERT INTO webhook_events (delivery_id, event_name) VALUES ($1, $2)
		 ON CONFLICT (delivery_id) DO NOTHING RETURNING delivery_id`,
		[]any{delivery, eventName},
		func(row pgx.Row) error { return row.Scan(&insertedDelivery) },
	)

	if insertedDelivery != "" && eventName == "pull_request" {
		var payload struct {
			PullRequest struct {
				Merged  bool   `json:"merged"`
				HTMLURL string `json:"html_url"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.PullRequest.Merged && payload.PullRequest.HTMLURL != "" {
				_, _ = h.contribSvc.ApplyMergedPRByUrl(r.Context(), payload.PullRequest.HTMLURL, h.cfg.GitHubToken)
			}
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"accepted":    true,
		"delivery_id": delivery,
		"event":       eventName,
	})
}

func (h *handlers) adminSyncRepository(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")
	if rawID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "repository id is required")
		return
	}
	path, err := syncPath(rawID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.proxyToSyncd(w, r, path)
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
		Name:     "skillissues_session",
		Value:    rawToken,
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
