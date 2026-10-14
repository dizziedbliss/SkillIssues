package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

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

// syncPath builds the ingestion endpoint that forces an immediate sync of
// one repository. The id is parsed as an integer so it can never inject
// extra path segments. Not a priority mechanism (D9) — it's an explicit
// admin nudge onto the same FIFO queue everything else uses.
func syncPath(rawRepoID string) (string, error) {
	id, err := strconv.Atoi(rawRepoID)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("invalid repository id")
	}
	return fmt.Sprintf("/sync/priority/%d", id), nil
}

// enqueueResync fires the same priority sync `adminSyncRepository` triggers,
// but fire-and-forget: the caller (a freshness check — D16) doesn't need the
// ingestion service's response, just to know the nudge was sent.
func (h *handlers) enqueueResync(ctx context.Context, repoID int) error {
	base := strings.TrimRight(h.cfg.SyncdURL, "/")
	if base == "" {
		return fmt.Errorf("ingestion service not configured")
	}
	path, err := syncPath(strconv.Itoa(repoID))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, nil)
	if err != nil {
		return fmt.Errorf("create sync request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ingestion service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ingestion service returned HTTP %d", resp.StatusCode)
	}
	return nil
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

// getIssueBody handles GET /issues/{id}/body — the on-demand read path
// (plan §3.1). Issue bodies are never stored unbounded (D17); this fetches
// through the Postgres cache / single-flight GraphQL path instead.
func (h *handlers) getIssueBody(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	if h.bodyFetcher == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "body fetching is not configured")
		return
	}

	userID := middleware.GetUserID(r.Context())
	var userToken string
	if userID != 0 {
		// Optional here: a read can use the server credential, so a user
		// whose token is unusable still gets the body — but say why.
		userToken, err = h.authSvc.GetAccessToken(r.Context(), userID)
		if err != nil {
			slog.Warn("issue body: user GitHub token unavailable, using server credential", "user_id", userID, "error", err)
			userToken = ""
		}
	}

	body, err := h.bodyFetcher.GetBody(r.Context(), id, userID, userToken)
	if err != nil {
		if errors.Is(err, issue.ErrBodyUnavailable) {
			reason := "fetching"
			switch {
			case errors.Is(err, issue.ErrBodyBudget):
				reason = "budget"
			case errors.Is(err, issue.ErrBodyNoCredential):
				reason = "no-credential"
			}
			middleware.RespondJSON(w, http.StatusAccepted, map[string]string{
				"status": "queued",
				"reason": reason,
			})
			return
		}
		middleware.RespondError(w, http.StatusBadGateway, "failed to fetch issue body")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{"body": body})
}

// freshnessResult is the per-issue response shape shared by the single-issue
// and batch freshness endpoints (D2a).
type freshnessResult struct {
	State           string `json:"state"` // "open" or "closed"
	RefreshEnqueued bool   `json:"refresh_enqueued,omitempty"`
}

// checkOneFreshness performs the live, on-demand GitHub state check (D16)
// for one issue using the caller's own GitHub authorization. If GitHub
// reports the issue closed, it persists that (D2a — GitHub's own updatedAt,
// never now()) so the saved/working lists catch up without waiting on the
// crawl, and enqueues a priority resync so the full catalog entry catches up
// too.
func (h *handlers) checkOneFreshness(ctx context.Context, issueID int, token string) (*freshnessResult, error) {
	it, err := h.issueSvc.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}
	owner, name, ok := strings.Cut(it.Repository, "/")
	if !ok {
		return nil, fmt.Errorf("malformed repository reference for issue %d", issueID)
	}

	state, err := h.ghClient.FetchIssueState(ctx, token, owner, name, it.Number)
	if err != nil {
		return nil, fmt.Errorf("check issue state on github: %w", err)
	}

	if state.State != "CLOSED" {
		return &freshnessResult{State: "open"}, nil
	}

	if err := h.issueSvc.SetState(ctx, issueID, "CLOSED", state.UpdatedAt); err != nil {
		// Never blocks the caller over this — it already has the live
		// answer — but a silent DB write failure here is exactly the kind
		// CLAUDE.md requires stay observable.
		slog.Warn("freshness check: failed to persist closed state", "issue_id", issueID, "error", err)
	}

	result := &freshnessResult{State: "closed"}
	if repoID, err := h.issueSvc.GetRepositoryID(ctx, issueID); err == nil {
		// Best-effort: the caller already has what it needs to block the
		// action and explain why, regardless of whether this nudge lands.
		go func(repoID int) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := h.enqueueResync(bgCtx, repoID); err != nil {
				slog.Warn("freshness check: failed to enqueue resync", "repository_id", repoID, "error", err)
			}
		}(repoID)
		result.RefreshEnqueued = true
	}
	return result, nil
}

// checkIssueFreshness handles POST /issues/{id}/freshness (D16) — made only
// at the moment a user commits to an issue (fork/clone).
func (h *handlers) checkIssueFreshness(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}

	userID := middleware.GetUserID(r.Context())
	token, ok := h.userGitHubToken(w, r, userID, "Connect GitHub to check issue freshness.")
	if !ok {
		return
	}

	result, err := h.checkOneFreshness(r.Context(), id, token)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			middleware.RespondError(w, http.StatusNotFound, "issue not found")
			return
		}
		middleware.RespondError(w, http.StatusBadGateway, "failed to check issue state on GitHub")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, result)
}

// checkIssuesFreshnessBatch handles POST /issues/freshness (D2a) — a client
// calls this once per Saved/Working render with the handful of issues it is
// actually showing, never the whole catalog. Capped by a live setting, and
// fails open per issue: a lookup failure for one id is dropped from the
// response rather than failing the whole batch, so a client never loses the
// issues it could check over the ones it couldn't.
func (h *handlers) checkIssuesFreshnessBatch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	token, ok := h.userGitHubToken(w, r, userID, "Connect GitHub to check issue freshness.")
	if !ok {
		return
	}

	var body struct {
		IssueIDs []int `json:"issue_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "issue_ids is required")
		return
	}

	batchCap := h.live.Current().Int(settings.IssuesFreshnessBatchCap)
	if len(body.IssueIDs) > batchCap {
		body.IssueIDs = body.IssueIDs[:batchCap]
	}

	results := make(map[string]freshnessResult, len(body.IssueIDs))
	for _, id := range body.IssueIDs {
		result, err := h.checkOneFreshness(r.Context(), id, token)
		if err != nil {
			slog.Warn("batch freshness check failed for issue", "issue_id", id, "error", err)
			continue
		}
		results[strconv.Itoa(id)] = *result
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{"results": results})
}
