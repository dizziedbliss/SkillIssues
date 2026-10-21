package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
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

// issuePage is GET /issues' response when the caller pages by ?cursor=.
// next_cursor is null on the last page.
type issuePage struct {
	Issues     []issue.Issue `json:"issues"`
	NextCursor *string       `json:"next_cursor"`
}

// listIssues serves the catalog. Without ?cursor= the response is a bare JSON
// array of issues, paged by ?page; with ?cursor= (empty for the first page)
// it is an issuePage, paged by the cursor each page returns. Cursor paging is
// for browsing only: a search ranks by match category and pages by ?page.
//
// A signed-out response is the same for every signed-out caller, so it is
// cached in Redis, shared by concurrent misses, and marked public for browsers
// and edge caches. A signed-in caller's browse is ordered by their preferred
// languages and is never shared.
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
	f, err := parseIssueFilters(q, cfg)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.Search = q.Get("search")
	if f.Category, err = issue.ParseCategory(q.Get("category")); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.Limit = limit
	f.Page = page
	f.RareLanguageBelow = cfg.Int(settings.IssuesRareLanguageThreshold)
	f.LanguageCountTTL = cfg.Seconds(settings.IssuesLanguageCountTTLSeconds)
	f.RareProbe = cfg.Millis(settings.IssuesRareProbeMs)
	f.ShuffleWalkMaxRepos = cfg.Int(settings.IssuesShuffleWalkMaxRepos)
	f.ShuffleWalkBudget = cfg.Millis(settings.IssuesShuffleWalkBudgetMs)
	f.SearchSample = cfg.Int(settings.SearchSampleSize)
	f.SearchPerRepo = cfg.Int(settings.SearchPerRepoCap)

	_, cursorMode := q["cursor"]
	rawCursor := q.Get("cursor")
	var after *issue.Cursor
	if cursorMode {
		if f.Search != "" {
			middleware.RespondError(w, http.StatusBadRequest, "cursor paging is for browsing; page a search with ?page")
			return
		}
		if rawCursor != "" {
			c, err := issue.DecodeCursor(rawCursor)
			if err != nil {
				middleware.RespondError(w, http.StatusBadRequest, "invalid cursor")
				return
			}
			after = &c
		}
	}
	// Cursor paging defaults to the shuffled feed; paging by number keeps the
	// easiest order it always had. Shuffle and newest page only by cursor.
	if f.Sort == "" {
		f.Sort = issue.SortEasiest
		if cursorMode {
			f.Sort = issue.SortShuffle
		}
	}
	if !cursorMode && f.Page > 1 && f.Search == "" && f.Sort != issue.SortEasiest {
		middleware.RespondError(w, http.StatusBadRequest, "page through the "+string(f.Sort)+" sort with ?cursor")
		return
	}

	userID := middleware.GetUserID(r.Context())
	load := func(ctx context.Context) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, cfg.Millis(settings.IssuesQueryTimeoutMs))
		defer cancel()
		return h.loadIssues(ctx, userID, f, cursorMode, after)
	}

	if userID != 0 {
		data, err := load(r.Context())
		if err != nil {
			respondIssueQueryError(w, r, err)
			return
		}
		respondIssues(w, r, cfg, data, false)
		return
	}

	// The hot path is the unfiltered first page at the default size. This
	// previously compared against a literal 50 while the default was 20, so the
	// cache was unreachable; comparing against the configured default fixes it.
	isDefaultHot := !cursorMode && f.Search == "" && f.Sort == issue.SortEasiest && !hasIssueFilters(f) &&
		f.Limit == cfg.Int(settings.IssuesDefaultPageSize) && f.Page == 1
	key := cache.CacheSearchKey(issueCacheKey(f, cursorMode, rawCursor))
	get := func() []byte { return h.cache.GetCachedSearch(r.Context(), key) }
	set := func(data []byte) {
		// A TTL of zero means caching is switched off for that path.
		if ttl := cfg.Seconds(settings.CacheSearchTTLSeconds); ttl > 0 {
			h.cache.SetCachedSearch(r.Context(), key, data, ttl)
		}
	}
	if isDefaultHot {
		key = "hot"
		get = func() []byte { return h.cache.GetHotIssues(r.Context()) }
		set = func(data []byte) {
			if ttl := cfg.Seconds(settings.CacheHotIssuesTTLSeconds); ttl > 0 {
				h.cache.SetHotIssues(r.Context(), data, ttl)
			}
		}
	}

	if cached := get(); cached != nil {
		respondIssues(w, r, cfg, cached, true)
		return
	}

	// Every request that misses at once waits on one query rather than each
	// running its own: a cold cache under load is exactly when the catalog
	// query is slowest. The shared query must not die with whichever caller
	// happened to start it, so it runs detached from that request's
	// cancellation, bounded by the query timeout alone.
	v, err, _ := h.issuesFlight.Do(key, func() (any, error) {
		data, err := load(context.WithoutCancel(r.Context()))
		if err != nil {
			return nil, err
		}
		set(data)
		return data, nil
	})
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	respondIssues(w, r, cfg, v.([]byte), true)
}

// loadIssues runs the catalog or search query and encodes the response body.
func (h *handlers) loadIssues(ctx context.Context, userID int, f issue.Filters, cursorMode bool, after *issue.Cursor) ([]byte, error) {
	var prefLangs []string
	var userName string
	if userID != 0 {
		p, err := h.profileSvc.GetFullProfile(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load profile for catalog order: %w", err)
		}
		prefLangs = p.PreferredLanguages
		userName = p.Username
	}

	// The first page of a browse and every cursor page are read in index
	// order; only a deeper ?page, or a search, takes the OFFSET query.
	if f.Search == "" && (cursorMode || f.Page == 1) {
		issues, next, err := h.issueSvc.BrowseIssues(ctx, f, after, prefLangs)
		if err != nil {
			return nil, err
		}
		if issues == nil {
			issues = []issue.Issue{}
		}
		if !cursorMode {
			return json.Marshal(issues)
		}
		body := issuePage{Issues: issues}
		if next != nil {
			token := next.Encode()
			body.NextCursor = &token
		}
		return json.Marshal(body)
	}

	issues, err := h.issueSvc.SearchIssues(ctx, f, prefLangs, userName)
	if err != nil {
		return nil, err
	}
	if issues == nil {
		issues = []issue.Issue{}
	}
	return json.Marshal(issues)
}

// respondIssues writes a catalog body with the caching headers it is allowed.
// A signed-out body is public — the same for every signed-out caller — and
// carries an ETag, so a repeat visit can be answered with a 304. A signed-in
// body depends on who asked and must not be stored anywhere.
func respondIssues(w http.ResponseWriter, r *http.Request, cfg *settings.Snapshot, data []byte, public bool) {
	w.Header().Add("Vary", "Cookie, Authorization")
	if !public {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	respondPublic(w, r, data, cfg.Int(settings.CachePublicMaxAgeSeconds), cfg.Int(settings.CachePublicStaleSeconds))
}

// respondPublic writes a body that is the same for every caller: cacheable
// for maxAge seconds (0: revalidate every time), served stale for up to stale
// more while revalidating, with an ETag so a repeat request gets a 304.
func respondPublic(w http.ResponseWriter, r *http.Request, data []byte, maxAge, stale int) {
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d", maxAge, stale))
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	sum := sha256.Sum256(data)
	etag := `W/"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// etagMatches reports whether an If-None-Match header names etag. The
// comparison is weak, as RFC 9110 requires for If-None-Match: W/"x" and "x"
// are the same entity tag.
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == want {
			return true
		}
	}
	return false
}

// respondIssueQueryError answers a failed catalog or issue query. A query
// that ran past api.issues.query_timeout_ms was cancelled: that is load, not a
// bug, and the caller may retry, so it is a 503 rather than a 500.
func respondIssueQueryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, issue.ErrBadCursor):
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		slog.Warn("issue query timed out", "route", r.Pattern, "error", err)
		middleware.RespondError(w, http.StatusServiceUnavailable, "the catalog is busy, try again shortly")
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		// The caller went away; there is nobody to answer.
		slog.Debug("issue query abandoned by client", "route", r.Pattern)
	default:
		slog.Error("issue query failed", "route", r.Pattern, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load issues")
	}
}

func (h *handlers) getIssue(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.live.Current().Millis(settings.IssuesQueryTimeoutMs))
	defer cancel()
	it, err := h.issueSvc.GetIssue(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return
	}
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	middleware.RespondJSON(w, http.StatusOK, it)
}

// getIssueBody is the issue body fallback, for a client that cannot read the
// body from GitHub itself (a signed-out browser past GitHub's 60
// unauthenticated requests an hour). A cached body is served to anyone. A
// miss spends the server's GitHub budget, so it is capped per caller at
// api.bodies.fallback_misses_per_hour, refused outright when Redis cannot
// count it, and fetched by ingestd, which holds the GitHub credential.
//
// 200 {"body"}; 202 {"status": "queued", "reason"} when the body cannot be
// fetched now; 404 when the issue does not exist; 429 past the cap.
func (h *handlers) getIssueBody(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		middleware.RespondError(w, http.StatusNotFound, "invalid issue id")
		return
	}
	cfg := h.live.Current()
	ctx, cancel := context.WithTimeout(r.Context(), cfg.Millis(settings.IssuesQueryTimeoutMs))
	defer cancel()

	body, ok, err := h.issueSvc.CachedBody(ctx, id)
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	if ok {
		h.respondBody(w, r, cfg, body)
		return
	}
	if _, err := h.issueSvc.GetIssue(ctx, id); errors.Is(err, db.ErrNotFound) {
		middleware.RespondError(w, http.StatusNotFound, "issue not found")
		return
	} else if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}

	limit := cfg.Int(settings.BodiesFallbackMissesPerHour)
	if limit == 0 {
		respondBodyQueued(w, "disabled")
		return
	}
	if h.cache == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "issue bodies cannot be fetched right now")
		return
	}
	// Fixed window, as the global limiter counts.
	res := h.cache.CheckRateLimit(r.Context(), "bodies:"+middleware.ClientKey(r, h.trustedProxies), int64(limit), time.Hour)
	if res.Unavailable {
		// An uncounted miss would be an uncapped one.
		middleware.RespondError(w, http.StatusServiceUnavailable, "issue bodies cannot be fetched right now")
		return
	}
	if !res.Allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(res.ResetAfter.Seconds()))))
		middleware.RespondError(w, http.StatusTooManyRequests, "too many issue bodies fetched; try again later")
		return
	}

	base := strings.TrimRight(h.cfg.SyncdURL, "/")
	if base == "" {
		middleware.RespondError(w, http.StatusInternalServerError, "ingestion service not configured")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, fmt.Sprintf("%s/bodies/%d", base, id), nil)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to create body request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.AdminSyncToken)
	resp, err := h.outbound.Do(req)
	if err != nil {
		slog.Error("issue body fetch from ingestd failed", "issue_id", id, "error", err)
		middleware.RespondError(w, http.StatusBadGateway, "ingestion service unavailable")
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIngestdBodyBytes))
	if err != nil {
		slog.Error("reading the issue body from ingestd failed", "issue_id", id, "error", err)
		middleware.RespondError(w, http.StatusBadGateway, "ingestion service unavailable")
		return
	}

	if resp.StatusCode == http.StatusOK {
		var fetched struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal(data, &fetched); err != nil {
			slog.Error("ingestd returned an unreadable issue body", "issue_id", id, "error", err)
			middleware.RespondError(w, http.StatusBadGateway, "ingestion service returned an unreadable body")
			return
		}
		h.respondBody(w, r, cfg, fetched.Body)
		return
	}
	// 202 queued, 404 gone, 5xx: ingestd's answer already has the API's shape.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(data)
}

// maxIngestdBodyBytes bounds ingestd's reply to a body fetch. GitHub caps an
// issue body at 65,536 characters; JSON-escaped at four bytes each plus
// escaping that is still under 1 MiB, so a larger reply is not a body.
const maxIngestdBodyBytes = 1 << 20

// respondBody answers with a body. It is the same for every caller, so it is
// public whoever asked.
func (h *handlers) respondBody(w http.ResponseWriter, r *http.Request, cfg *settings.Snapshot, body string) {
	data, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		slog.Error("encoding an issue body failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to encode the issue body")
		return
	}
	respondIssues(w, r, cfg, data, true)
}

// respondBodyQueued tells the client the body cannot be had now, in the
// shape the clients already poll for.
func respondBodyQueued(w http.ResponseWriter, reason string) {
	w.Header().Set("Cache-Control", "no-store")
	middleware.RespondJSON(w, http.StatusAccepted, map[string]string{"status": "queued", "reason": reason})
}

// parseIssueFilters reads GET /issues' filters and sort. A list filter takes
// comma-separated values, any of which matches; filters combine with AND.
// Anything malformed is an error for a 400, never silently dropped.
func parseIssueFilters(q url.Values, cfg *settings.Snapshot) (issue.Filters, error) {
	maxValues := cfg.Int(settings.IssuesFilterMaxValues)
	var f issue.Filters
	var err error
	if f.Difficulties, err = issue.ParseDifficulties(q.Get("difficulty"), maxValues); err != nil {
		return f, err
	}
	for _, list := range []struct {
		name string
		dst  *[]string
	}{
		{"language", &f.Languages}, {"labels", &f.Labels}, {"topics", &f.Topics}, {"tech", &f.Tech},
		{"repo", &f.Repos}, {"owner", &f.Owners},
	} {
		if *list.dst, err = issue.ParseList(list.name, q.Get(list.name), maxValues); err != nil {
			return f, err
		}
	}
	for _, bound := range []struct {
		name string
		dst  **int
	}{
		{"comments_min", &f.CommentsMin}, {"comments_max", &f.CommentsMax},
	} {
		raw := q.Get(bound.name)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return f, fmt.Errorf("%s must be a whole number of comments, 0 or more", bound.name)
		}
		*bound.dst = &n
	}
	if f.CommentsMin != nil && f.CommentsMax != nil && *f.CommentsMin > *f.CommentsMax {
		return f, fmt.Errorf("comments_min is more than comments_max")
	}
	switch q.Get("unassigned") {
	case "", "false":
	case "true":
		f.Unassigned = true
	default:
		return f, fmt.Errorf("unassigned must be true or false")
	}
	if f.Sort, err = issue.ParseSort(q.Get("sort")); err != nil {
		return f, err
	}
	return f, nil
}

func hasIssueFilters(f issue.Filters) bool {
	return len(f.Difficulties) > 0 || len(f.Languages) > 0 || len(f.Labels) > 0 || len(f.Topics) > 0 ||
		len(f.Tech) > 0 || len(f.Repos) > 0 || len(f.Owners) > 0 || f.CommentsMin != nil || f.CommentsMax != nil
}

// issueCacheKey is the canonical form of everything a signed-out catalog
// response depends on: the same query, however spelled, shares one entry.
func issueCacheKey(f issue.Filters, cursorMode bool, cursor string) string {
	list := func(vals []string) string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = strings.ToLower(v)
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	bound := func(p *int) string {
		if p == nil {
			return ""
		}
		return strconv.Itoa(*p)
	}
	return strings.Join([]string{
		"sort=" + string(f.Sort), "q=" + f.Search, "cat=" + f.Category,
		"d=" + list(f.Difficulties), "lang=" + list(f.Languages), "labels=" + list(f.Labels),
		"topics=" + list(f.Topics), "tech=" + list(f.Tech),
		"repo=" + list(f.Repos), "owner=" + list(f.Owners), "unassigned=" + strconv.FormatBool(f.Unassigned),
		"cmin=" + bound(f.CommentsMin), "cmax=" + bound(f.CommentsMax),
		"limit=" + strconv.Itoa(f.Limit), "page=" + strconv.Itoa(f.Page),
		"cursor_mode=" + strconv.FormatBool(cursorMode), "cursor=" + cursor,
	}, "|")
}

// getIssueFacets serves the filter dropdowns' values (issue.Facets): the
// same for everyone, and public.
func (h *handlers) getIssueFacets(w http.ResponseWriter, r *http.Request) {
	cfg := h.live.Current()
	ctx, cancel := context.WithTimeout(r.Context(), cfg.Millis(settings.IssuesQueryTimeoutMs))
	defer cancel()
	facets, err := h.issueSvc.Facets(ctx)
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}
	data, err := json.Marshal(facets)
	if err != nil {
		slog.Error("encoding facets failed", "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to encode facets")
		return
	}
	respondIssues(w, r, cfg, data, true)
}
