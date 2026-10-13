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
		userToken, _ = h.authSvc.GetAccessToken(r.Context(), userID)
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

// checkIssueFreshness handles POST /issues/{id}/freshness (D16) — a live
// GitHub state check made only at the moment a user commits to an issue
// (fork/clone), never on list render or detail open, using the user's own
// GitHub authorization rather than the server's. If GitHub reports the
// issue closed, this enqueues a priority resync so the catalog catches up
// and tells the caller to block the action with an explanation.
func (h *handlers) checkIssueFreshness(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}

	userID := middleware.GetUserID(r.Context())
	token, _ := h.authSvc.GetAccessToken(r.Context(), userID)
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub to check issue freshness.")
		return
	}

	it, err := h.issueSvc.GetIssue(r.Context(), id)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return
	}
	owner, name, ok := strings.Cut(it.Repository, "/")
	if !ok {
		middleware.RespondError(w, http.StatusInternalServerError, "malformed repository reference")
		return
	}

	state, err := h.ghClient.FetchIssueState(r.Context(), token, owner, name, it.Number)
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, "failed to check issue state on GitHub")
		return
	}

	if state.State != "CLOSED" {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"state": "open"})
		return
	}

	refreshEnqueued := false
	if repoID, err := h.issueSvc.GetRepositoryID(r.Context(), id); err == nil {
		// Best-effort: the client already has what it needs to block the
		// action and explain why, regardless of whether this nudge lands.
		go func(repoID int) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := h.enqueueResync(ctx, repoID); err != nil {
				slog.Warn("freshness check: failed to enqueue resync", "repository_id", repoID, "error", err)
			}
		}(repoID)
		refreshEnqueued = true
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"state":            "closed",
		"refresh_enqueued": refreshEnqueued,
	})
}
