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
	return NewClient(srv.URL), &vars, &query
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
