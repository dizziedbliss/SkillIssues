package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

func TestEtagMatches(t *testing.T) {
	const etag = `W/"abc"`
	for _, tc := range []struct {
		header string
		want   bool
	}{
		{"", false},
		{`W/"abc"`, true},
		{`"abc"`, true}, // If-None-Match compares weakly
		{`"other", W/"abc"`, true},
		{`"other"`, false},
		{"*", true},
	} {
		if got := etagMatches(tc.header, etag); got != tc.want {
			t.Errorf("etagMatches(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

// A signed-out catalog response is cacheable by anyone and revalidates to a
// 304; a signed-in one is never stored.
func TestRespondIssuesCacheHeaders(t *testing.T) {
	cfg := settings.Defaults().
		With(settings.CachePublicMaxAgeSeconds, 60).
		With(settings.CachePublicStaleSeconds, 300)
	body := []byte(`[{"id":1}]`)

	rec := httptest.NewRecorder()
	respondIssues(rec, httptest.NewRequest(http.MethodGet, "/issues", nil), cfg, body, true)
	if rec.Code != http.StatusOK || rec.Body.String() != string(body) {
		t.Fatalf("public: %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=300" {
		t.Errorf("public Cache-Control = %q", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("public response has no ETag")
	}

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	respondIssues(rec, req, cfg, body, true)
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("revalidation: %d with %d body bytes, want 304 and none", rec.Code, rec.Body.Len())
	}

	rec = httptest.NewRecorder()
	respondIssues(rec, httptest.NewRequest(http.MethodGet, "/issues", nil), cfg.With(settings.CachePublicMaxAgeSeconds, 0), body, true)
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("max-age 0: Cache-Control = %q, want no-cache", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/issues", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	respondIssues(rec, req, cfg, body, false)
	if rec.Code != http.StatusOK || rec.Body.String() != string(body) {
		t.Errorf("private: %d %q, want the body even for a matching ETag", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("private Cache-Control = %q", got)
	}
	if rec.Header().Get("ETag") != "" {
		t.Error("private response carries an ETag")
	}
}

// bodyHandlers is a handlers value over the test database with no Redis and
// no ingestd: enough for the cache and the refusals.
func bodyHandlers(t *testing.T) (*handlers, *db.Pool) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := db.New(context.Background(), db.DefaultConfig(url))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &handlers{
		cfg:      &config.Config{},
		live:     settings.NewLive(context.Background(), pool.Raw(), url),
		db:       pool,
		issueSvc: issue.New(pool, nil),
	}, pool
}

// A cached body is served, publicly, and marked read; a miss without Redis
// to count it is refused rather than fetched uncapped; an unknown issue is
// 404.
func TestGetIssueBody(t *testing.T) {
	h, pool := bodyHandlers(t)
	ctx := context.Background()

	var repoID, cachedID, uncachedID int
	scan := func(dst *int) func(pgx.Row) error { return func(row pgx.Row) error { return row.Scan(dst) } }
	name := fmt.Sprintf("body-api-test/r%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`, []any{name}, scan(&repoID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	for n, dst := range map[int]*int{1: &cachedID, 2: &uncachedID} {
		if err := pool.QueryRow(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED') RETURNING id`,
			[]any{repoID, n}, scan(dst)); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.Exec(ctx,
		`INSERT INTO issue_bodies (issue_id, body, source, last_read_at) VALUES ($1, 'cached body', 'fetch', now() - interval '1 day')`,
		cachedID); err != nil {
		t.Fatal(err)
	}

	get := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/issues/"+id+"/body", nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h.getIssueBody(rec, req)
		return rec
	}

	rec := get(strconv.Itoa(cachedID))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cached body"`) {
		t.Fatalf("cached: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Cache-Control"), "public") {
		t.Errorf("cached body Cache-Control = %q, want public", rec.Header().Get("Cache-Control"))
	}
	var readRecently bool
	if err := pool.QueryRow(ctx, `SELECT last_read_at > now() - interval '1 minute' FROM issue_bodies WHERE issue_id = $1`,
		[]any{cachedID}, func(row pgx.Row) error { return row.Scan(&readRecently) }); err != nil || !readRecently {
		t.Errorf("serving the body did not mark it read (err %v)", err)
	}

	if rec := get(strconv.Itoa(uncachedID)); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("miss without Redis: %d %s, want 503", rec.Code, rec.Body.String())
	}
	if rec := get("999999999"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown issue: %d, want 404", rec.Code)
	}
	if rec := get("nope"); rec.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d, want 404", rec.Code)
	}
}

func TestParseIssueFilters(t *testing.T) {
	cfg := settings.Defaults().With(settings.IssuesFilterMaxValues, 3)
	parse := func(raw string) (issue.Filters, error) {
		t.Helper()
		q, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		return parseIssueFilters(q, cfg)
	}

	f, err := parse("difficulty=beginner,UNCLASSIFIED&language=Go&labels=bug,docs&topics=cli&tech=shell&comments_min=1&comments_max=5&sort=newest")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.Difficulties, []string{"BEGINNER", "UNCLASSIFIED"}) || !slices.Equal(f.Labels, []string{"bug", "docs"}) ||
		!slices.Equal(f.Languages, []string{"Go"}) || !slices.Equal(f.Topics, []string{"cli"}) || !slices.Equal(f.Tech, []string{"shell"}) ||
		*f.CommentsMin != 1 || *f.CommentsMax != 5 || f.Sort != issue.SortNewest {
		t.Errorf("parsed %+v", f)
	}

	for _, bad := range []string{
		"difficulty=trivial",
		"labels=a,b,c,d", // over the 3-value limit
		"comments_min=-1",
		"comments_max=lots",
		"comments_min=5&comments_max=2",
		"sort=stars",
	} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

// The same query, spelled differently, shares one cache entry; a different
// filter does not.
func TestIssueCacheKeyIsCanonical(t *testing.T) {
	a := issue.Filters{Labels: []string{"Bug", "docs"}, Sort: issue.SortShuffle, Limit: 20}
	b := issue.Filters{Labels: []string{"docs", "bug"}, Sort: issue.SortShuffle, Limit: 20}
	c := issue.Filters{Labels: []string{"docs"}, Sort: issue.SortShuffle, Limit: 20}
	if issueCacheKey(a, true, "") != issueCacheKey(b, true, "") {
		t.Error("reordered or re-cased labels gave a different key")
	}
	if issueCacheKey(a, true, "") == issueCacheKey(c, true, "") || issueCacheKey(a, true, "") == issueCacheKey(a, false, "") {
		t.Error("different requests shared a key")
	}
}
