package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/issue"
)

// GET /stats is public, readable from any origin, and lists what ingestd
// folded in.
func TestGetStats(t *testing.T) {
	h, _ := bodyHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	h.getStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if !strings.HasPrefix(rec.Header().Get("Cache-Control"), "public") {
		t.Errorf("Cache-Control = %q, want public", rec.Header().Get("Cache-Control"))
	}
	var stats issue.CatalogStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.TrackingSince == nil || stats.Activity == nil {
		t.Errorf("stats = %+v, want tracking_since and an activity list", stats)
	}
}

// The lookup finds a repository whatever the case or URL form, counts its
// open issues, says why an ineligible one is not crawled, and refuses a name
// that is not owner/repo.
func TestLookupRepository(t *testing.T) {
	h, pool := bodyHandlers(t)
	ctx := context.Background()
	name := fmt.Sprintf("Lookup-Test/r%d", time.Now().UnixNano())
	var repoID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url, is_eligible, eligibility_reason) VALUES ($1, 'u', false, 'archived') RETURNING id`,
		[]any{name}, func(row pgx.Row) error { return row.Scan(&repoID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if err := pool.Exec(ctx, `INSERT INTO issues (repository_id, number, title, url, difficulty, state)
		VALUES ($1, 1, 't', 'u', 'UNCLASSIFIED', 'OPEN'), ($1, 2, 't', 'u', 'UNCLASSIFIED', 'CLOSED')`, repoID); err != nil {
		t.Fatal(err)
	}

	lookup := func(q string) (int, issue.RepositoryLookup) {
		req := httptest.NewRequest(http.MethodGet, "/repositories/lookup?name="+q, nil)
		rec := httptest.NewRecorder()
		h.lookupRepository(rec, req)
		var out issue.RepositoryLookup
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return rec.Code, out
	}
	for _, q := range []string{strings.ToLower(name), "https://github.com/" + name + "/"} {
		code, got := lookup(q)
		if code != http.StatusOK || !got.Found || got.FullName != name || got.OpenIssues != 1 || got.Eligible || got.Reason != "archived" {
			t.Errorf("lookup %q: %d %+v", q, code, got)
		}
	}
	if code, got := lookup("nobody-has/this-repo-" + fmt.Sprint(time.Now().UnixNano())); code != http.StatusOK || got.Found {
		t.Errorf("unknown repository: %d %+v, want found=false", code, got)
	}
	for _, bad := range []string{"", "no-slash", "a/b/c", "%27%3B--/x"} {
		if code, _ := lookup(bad); code != http.StatusBadRequest {
			t.Errorf("lookup %q: %d, want 400", bad, code)
		}
	}
}
