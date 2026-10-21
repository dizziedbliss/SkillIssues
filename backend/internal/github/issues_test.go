package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGraphQL serves one canned response and records the request's variables.
func fakeGraphQL(t *testing.T, response string) (*Client, *map[string]any, *string) {
	t.Helper()
	var vars map[string]any
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		vars, query = req.Variables, req.Query
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, func() time.Duration { return 10 * time.Second }), &vars, &query
}

func TestFetchRepositoryIssuesFullCrawlSkipsClosed(t *testing.T) {
	c, vars, _ := fakeGraphQL(t, `{"data":{"repository":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}},"stargazerCount":1},"rateLimit":{"limit":5000,"remaining":4999,"used":1,"resetAt":"2026-10-16T00:00:00Z","cost":1}}}`)
	// FetchClosed without Since must not ask GitHub for closed issues: a
	// full crawl has nothing stored that could have closed.
	if _, err := c.FetchRepositoryIssues(context.Background(), "t", "o", "n", IssuesQuery{FetchOpen: true, PageSize: 50, FetchClosed: true, ClosedPageSize: 40}); err != nil {
		t.Fatal(err)
	}
	if (*vars)["withClosed"] != false || (*vars)["withOpen"] != true {
		t.Errorf("withOpen=%v withClosed=%v, want true/false", (*vars)["withOpen"], (*vars)["withClosed"])
	}
	if _, ok := (*vars)["since"]; ok {
		t.Errorf("since sent on a full crawl: %v", (*vars)["since"])
	}
}

func TestFetchRepositoryIssuesIncrementalParsesClosed(t *testing.T) {
	c, vars, query := fakeGraphQL(t, `{"data":{"repository":{
		"issues":{"nodes":[{"databaseId":9,"number":3,"title":"t","url":"u","state":"OPEN","createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-15T00:00:00Z","author":{"login":"a"},"comments":{"totalCount":2},"labels":{"nodes":[{"name":"bug"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":"o1"}},
		"closed":{"nodes":[{"number":1,"updatedAt":"2026-10-14T12:00:00Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}},
		"stargazerCount":1},
		"rateLimit":{"limit":5000,"remaining":4998,"used":2,"resetAt":"2026-10-16T00:00:00Z","cost":1}}}`)
	since := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	cursor := "prev"
	page, err := c.FetchRepositoryIssues(context.Background(), "t", "o", "n", IssuesQuery{
		FetchOpen: true, PageSize: 50, Since: &since, FetchClosed: true, ClosedCursor: &cursor, ClosedPageSize: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	if (*vars)["withClosed"] != true || (*vars)["since"] != "2026-10-13T00:00:00Z" || (*vars)["closedAfter"] != "prev" {
		t.Errorf("variables = %v", *vars)
	}
	if !strings.Contains(*query, "states: CLOSED") {
		t.Error("query has no closed-issues connection")
	}
	if len(page.Issues) != 1 || page.Issues[0].Number != 3 || page.HasNextPage {
		t.Errorf("open page = %+v", page.Issues)
	}
	want := ClosedIssue{Number: 1, UpdatedAt: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
	if len(page.ClosedIssues) != 1 || page.ClosedIssues[0] != want {
		t.Errorf("closed = %+v, want [%+v]", page.ClosedIssues, want)
	}
	if !page.ClosedHasNextPage || page.ClosedEndCursor != "c1" {
		t.Errorf("closed pageInfo = %v %q", page.ClosedHasNextPage, page.ClosedEndCursor)
	}
}

// One missing issue must not fail the batch; it comes back Found=false.
func TestFetchIssueStatesToleratesNotFound(t *testing.T) {
	c, vars, _ := fakeGraphQL(t, `{"data":{
		"i0":{"issue":{"state":"CLOSED","updatedAt":"2026-10-15T08:00:00Z"}},
		"i1":{"issue":null},
		"i2":null,
		"rateLimit":{"limit":5000,"remaining":4990,"used":10,"resetAt":"2026-10-16T00:00:00Z","cost":1}},
		"errors":[{"type":"NOT_FOUND","path":["i1","issue"],"message":"Could not resolve to an Issue"},
		          {"type":"NOT_FOUND","path":["i2"],"message":"Could not resolve to a Repository"}]}`)
	states, rl, err := c.FetchIssueStates(context.Background(), "t", []IssueRef{
		{Owner: "o", Name: "a", Number: 1}, {Owner: "o", Name: "a", Number: 2}, {Owner: "gone", Name: "b", Number: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if (*vars)["k2"] != float64(3) || (*vars)["o2"] != "gone" {
		t.Errorf("variables = %v", *vars)
	}
	if !states[0].Found || states[0].State != "CLOSED" || !states[0].UpdatedAt.Equal(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("states[0] = %+v", states[0])
	}
	if states[1].Found || states[2].Found {
		t.Errorf("missing issues reported found: %+v %+v", states[1], states[2])
	}
	if rl.Remaining != 4990 {
		t.Errorf("rate limit remaining = %d", rl.Remaining)
	}
}

func TestFetchIssueStatesFailsOnOtherErrors(t *testing.T) {
	c, _, _ := fakeGraphQL(t, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`)
	if _, _, err := c.FetchIssueStates(context.Background(), "t", []IssueRef{{Owner: "o", Name: "a", Number: 1}}); err == nil {
		t.Fatal("want error for a non-NOT_FOUND GraphQL error")
	}
}

// Only an https page on github.com is kept as a contributing guide: it becomes
// a link the apps open.
func TestIsGitHubPage(t *testing.T) {
	for u, want := range map[string]bool{
		"https://github.com/o/n/blob/main/CONTRIBUTING.md":       true,
		"https://github.com/o/.github/blob/main/CONTRIBUTING.md": true,
		"http://github.com/o/n/blob/main/CONTRIBUTING.md":        false,
		"https://gist.github.com/x":                              false,
		"https://github.com.evil.example/o/n":                    false,
		"https://user@github.com/o/n":                            false,
		"javascript:alert(1)":                                    false,
		"":                                                       false,
	} {
		if got := isGitHubPage(u); got != want {
			t.Errorf("isGitHubPage(%q) = %v, want %v", u, got, want)
		}
	}
}

// Several people's pull requests come back from one request, in order, and an
// unknown login doesn't fail the others.
func TestFetchUserPullRequests(t *testing.T) {
	c, vars, query := fakeGraphQL(t, `{"data":{
		"u0":{"databaseId":42,"pullRequests":{"pageInfo":{"hasNextPage":true,"endCursor":"c2"},"nodes":[
			{"databaseId":7001,"number":12,"title":"Fix the docs","url":"https://github.com/o/r/pull/12","state":"MERGED",
			 "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-03T00:00:00Z","mergedAt":"2026-10-03T00:00:00Z","closedAt":"2026-10-03T00:00:00Z",
			 "authorAssociation":"CONTRIBUTOR","additions":10,"deletions":2,
			 "repository":{"databaseId":555,"nameWithOwner":"o/r","owner":{"__typename":"Organization"}},
			 "approved":{"totalCount":1},"changes":{"totalCount":0},
			 "commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]},
			 "closingIssuesReferences":{"nodes":[{"number":3,"repository":{"databaseId":555,"nameWithOwner":"o/r"}}]}},
			{"databaseId":7002,"number":1,"title":"Mine","url":"u","state":"OPEN","createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-02T00:00:00Z",
			 "mergedAt":null,"closedAt":null,"authorAssociation":"OWNER","additions":1,"deletions":0,
			 "repository":{"databaseId":556,"nameWithOwner":"me/x","owner":{"__typename":"User","databaseId":42}},
			 "approved":{"totalCount":0},"changes":{"totalCount":0},"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]},
			 "closingIssuesReferences":{"nodes":[]}},
			{"databaseId":7003,"number":2,"title":"gone","url":"u","state":"OPEN","createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z","repository":null}
		]}},
		"u1":null,
		"rateLimit":{"limit":5000,"remaining":4990,"used":10,"resetAt":"2026-10-16T00:00:00Z","cost":3}},
		"errors":[{"type":"NOT_FOUND","path":["u1"],"message":"Could not resolve to a User"}]}`)
	after := "c1"
	pages, rl, err := c.FetchUserPullRequests(context.Background(), "t", []UserPullsQuery{
		{Login: "someone", First: 20, After: &after}, {Login: "renamed", First: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if (*vars)["l0"] != "someone" || (*vars)["a0"] != "c1" || (*vars)["f1"] != float64(20) {
		t.Errorf("variables = %v", *vars)
	}
	if _, ok := (*vars)["a1"]; ok {
		t.Error("a first page sent a cursor")
	}
	if !strings.Contains(*query, "orderBy: {field: UPDATED_AT, direction: DESC}") {
		t.Error("not ordered newest-updated first")
	}
	if rl.Cost != 3 || len(pages) != 2 || pages[1].Found {
		t.Fatalf("rate limit %+v, pages %+v", rl, pages)
	}
	p := pages[0]
	if !p.Found || p.DatabaseID != 42 || !p.HasNextPage || p.EndCursor != "c2" || len(p.Pulls) != 2 {
		t.Fatalf("page = %+v", p)
	}
	m := p.Pulls[0]
	if m.State != "MERGED" || m.MergedAt == nil || m.Checks == nil || *m.Checks != "SUCCESS" || m.Approvals != 1 ||
		m.RepositoryID != 555 || m.OwnerID != nil || len(m.Closes) != 1 || m.Closes[0].Number != 3 {
		t.Errorf("merged = %+v", m)
	}
	own := p.Pulls[1]
	if own.Checks != nil || own.OwnerID == nil || *own.OwnerID != 42 || own.MergedAt != nil {
		t.Errorf("own = %+v", own)
	}
}

// Health counts merges from outside the project and takes the median time to
// merge, the middle pair averaged when there are an even number.
func TestHealthOf(t *testing.T) {
	node := func(assoc string, hours int) mergedPullNode {
		created := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		return mergedPullNode{AuthorAssociation: assoc, CreatedAt: created.Format(time.RFC3339),
			MergedAt: created.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)}
	}
	h, err := healthOf([]mergedPullNode{node("OWNER", 1), node("CONTRIBUTOR", 10), node("FIRST_TIME_CONTRIBUTOR", 30), node("MEMBER", 100)})
	if err != nil {
		t.Fatal(err)
	}
	if h.Sampled != 4 || h.Outside != 2 || h.MedianMergeHours == nil || *h.MedianMergeHours != 20 {
		t.Errorf("health = %+v (median %v)", h, h.MedianMergeHours)
	}
	if h, err := healthOf(nil); err != nil || h.Sampled != 0 || h.MedianMergeHours != nil {
		t.Errorf("nothing merged = %+v, %v", h, err)
	}
}
