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
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// handlers bundles all service dependencies for the handler functions.
type handlers struct {
	cfg        *config.Config
	db         *db.Pool
	cache      *cache.Client
	authSvc    *auth.Service
	ghOAuth    *auth.GitHubOAuth
	profileSvc *profile.Service
	issueSvc   *issue.Service
	contribSvc *contribution.Service
	ghClient   *github.Client
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
			"error":         "GITHUB_CLIENT_ID is not configured",
		})
		return
	}

	authorizeURL, _, err := h.ghOAuth.BeginOAuth(r.Context(), h.cfg.GitHubRedirectURI)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin OAuth flow: "+err.Error())
		return
	}

	// If browser directly navigated or explicitly requested redirect
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

	redirectWithError := func(code string, detail string) {
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

	// Redirect to frontend with token in query param for robust multi-origin support
	sep := "?"
	if strings.Contains(frontendURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, fmt.Sprintf("%s%stoken=%s", frontendURL, sep, url.QueryEscape(rawToken)), http.StatusFound)
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

	// Fetch user's token or fallback
	var token string
	_ = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(github_access_token, '') FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&token) },
	)
	if token == "" {
		token = h.cfg.GitHubToken
	}
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

func (h *handlers) syncIssues(w http.ResponseWriter, r *http.Request) {
	h.proxyToPython(w, r, "/issues/sync")
}

func (h *handlers) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 20
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	f := issue.Filters{
		Difficulty: q.Get("difficulty"),
		Language:   q.Get("language"),
		Search:     q.Get("search"),
		Limit:      limit,
	}

	isDefaultHot := f.Difficulty == "" && f.Language == "" && f.Search == "" && f.Limit == 50
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
		searchKey = cache.CacheSearchKey(f.Difficulty, f.Language, f.Search, f.Limit)
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

	// Populate cache
	if userID == 0 && h.cache != nil {
		if data, err := json.Marshal(issues); err == nil {
			if isDefaultHot {
				h.cache.SetHotIssues(r.Context(), data, 2*time.Minute)
			} else if searchKey != "" {
				h.cache.SetCachedSearch(r.Context(), searchKey, data, 1*time.Minute)
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

	if limit == 3 && h.cache != nil {
		if data, err := json.Marshal(scored); err == nil {
			h.cache.SetRecommendations(r.Context(), userID, data, 5*time.Minute)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Branch == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "branch name is required")
		return
	}

	var (
		issueID, issueNum int
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
		token = h.cfg.GitHubToken
	}
	if token == "" && username != "alex-dev" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub before submitting a pull request.")
		return
	}

	owner, name, _ := strings.Cut(repoFull, "/")
	prTitle := title
	prBody := issueBody
	if prBody == "" {
		prBody = fmt.Sprintf("SkillIssues contribution for issue #%d", issueNum)
	}

	var prURL string
	var prNum int
	var pullRequest map[string]any

	if token != "" && username != "alex-dev" {
		head := body.Branch
		if username != "" {
			head = fmt.Sprintf("%s:%s", username, body.Branch)
		}
		res, err := h.ghClient.CreatePullRequest(r.Context(), token, owner, name, prTitle, head, "main", prBody)
		if err == nil {
			prURL = res.HTMLURL
			prNum = res.Number
			pullRequest = map[string]any{"html_url": prURL, "number": prNum}
		}
	}

	if prURL == "" {
		if username != "" {
			prURL = fmt.Sprintf("https://github.com/%s/%s/compare/main...%s:%s?expand=1", owner, name, username, body.Branch)
		} else {
			prURL = fmt.Sprintf("https://github.com/%s/%s", owner, name)
		}
		prNum = issueNum
		pullRequest = map[string]any{
			"html_url":       prURL,
			"number":         prNum,
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
	} else if !h.cfg.DemoMode {
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
	h.proxyToPython(w, r, "/admin/sync/repository/"+r.PathValue("id"))
}

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
	isSecure := strings.HasPrefix(h.cfg.FrontendURL, "https://")
	sameSite := http.SameSiteLaxMode
	if isSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "skillissues_session",
		Value:    rawToken,
		MaxAge:   int(auth.SessionDuration.Seconds()),
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
