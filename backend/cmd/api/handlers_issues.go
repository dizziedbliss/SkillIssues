package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	// ingestd's write triggers take the same admin token as /admin/*.
	req.Header.Set("Authorization", "Bearer "+h.cfg.AdminSyncToken)
	resp, err := h.outbound.Do(req)
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
			limit = min(l, cfg.Int(settings.IssuesMaxLimit))
		}
	}
	page := 1
	if pStr := q.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = min(p, cfg.Int(settings.IssuesMaxPage))
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
