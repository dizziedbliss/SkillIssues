package github

import (
	"context"
	"strings"
	"testing"
)

// One alias per repository; one GitHub cannot resolve is left out, the rest
// are returned keyed by the name asked for, and a malformed name is never sent.
func TestLookupRepositories(t *testing.T) {
	c, vars, query := fakeGraphQL(t, `{"data":{
		"r0":{"databaseId":7,"nameWithOwner":"Acme/Widget","url":"u","primaryLanguage":{"name":"Go"},
		      "stargazerCount":120,"forkCount":3,"watchers":{"totalCount":4},"isFork":false,"isArchived":false,
		      "issues":{"totalCount":40},"openIssues":{"totalCount":9},"pushedAt":"2026-10-01T00:00:00Z"},
		"r1":null,
		"rateLimit":{"limit":12500,"remaining":12400,"used":100,"resetAt":"2026-10-19T12:00:00Z","cost":1}},
		"errors":[{"type":"NOT_FOUND","path":["r1"],"message":"Could not resolve to a Repository"}]}`)

	got, rl, err := c.LookupRepositories(context.Background(), "t", []string{"acme/widget", "gone/away", "not-a-name"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d repositories, want 1: %+v", len(got), got)
	}
	r, ok := got["acme/widget"]
	if !ok || r.DatabaseID != 7 || r.NameWithOwner != "Acme/Widget" || r.Stars != 120 ||
		r.OpenIssueCount != 9 || r.PrimaryLanguage == nil || *r.PrimaryLanguage != "Go" || r.PushedAt == nil {
		t.Errorf("acme/widget = %+v", r)
	}
	if rl.Remaining != 12400 {
		t.Errorf("rate limit remaining = %d, want 12400", rl.Remaining)
	}
	if (*vars)["o0"] != "acme" || (*vars)["n1"] != "away" {
		t.Errorf("variables = %v", *vars)
	}
	if _, ok := (*vars)["o2"]; ok {
		t.Error("a malformed name was sent to GitHub")
	}
	if !strings.Contains(*query, "fragment RepositoryFields on Repository") {
		t.Error("query does not carry the repository fragment")
	}
}

// Any error other than NOT_FOUND fails the whole batch.
func TestLookupRepositoriesFailsOnOtherErrors(t *testing.T) {
	c, _, _ := fakeGraphQL(t, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"slow down"}]}`)
	if _, _, err := c.LookupRepositories(context.Background(), "t", []string{"a/b"}); err == nil {
		t.Fatal("want an error")
	}
}

func TestFetchIssueBody(t *testing.T) {
	c, vars, _ := fakeGraphQL(t, `{"data":{"repository":{"issue":{"body":"## Steps\n1. run","updatedAt":"2026-10-19T08:00:00Z"}},
		"rateLimit":{"limit":12500,"remaining":12499,"used":1,"resetAt":"2026-10-19T09:00:00Z","cost":1}}}`)
	got, rl, err := c.FetchIssueBody(context.Background(), "t", IssueRef{Owner: "o", Name: "r", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.Body != "## Steps\n1. run" || got.UpdatedAt.IsZero() || rl.Remaining != 12499 {
		t.Errorf("got %+v, rate limit %+v", got, rl)
	}
	if (*vars)["number"] != float64(7) || (*vars)["owner"] != "o" {
		t.Errorf("variables = %v", *vars)
	}
}

func TestFetchIssueBodyNotFound(t *testing.T) {
	c, _, _ := fakeGraphQL(t, `{"data":{"repository":null,"rateLimit":{"limit":1,"remaining":1,"used":0,"resetAt":"2026-10-19T09:00:00Z","cost":1}},
		"errors":[{"type":"NOT_FOUND","message":"Could not resolve"}]}`)
	got, _, err := c.FetchIssueBody(context.Background(), "t", IssueRef{Owner: "o", Name: "gone", Number: 1})
	if err != nil || got.Found {
		t.Errorf("got %+v, err %v; want not found and no error", got, err)
	}
}
