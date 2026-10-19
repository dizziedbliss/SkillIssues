package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dizziedbliss/skillissues/internal/settings"
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
