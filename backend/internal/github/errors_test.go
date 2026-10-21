package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// statusServer answers every request with status, headers and body.
func statusServer(t *testing.T, status int, headers map[string]string, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, func() time.Duration { return 10 * time.Second })
}

func fetchErr(t *testing.T, c *Client) error {
	t.Helper()
	_, err := c.FetchRepositoryIssues(context.Background(), "t", "o", "r", IssuesQuery{PageSize: 10})
	if err == nil {
		t.Fatal("want an error")
	}
	return err
}

func TestForbiddenWithoutRateLimitIsNotOne(t *testing.T) {
	err := fetchErr(t, statusServer(t, 403, nil, `{"message":"Resource not accessible by integration"}`))
	if limited, _ := IsRateLimited(err); limited {
		t.Errorf("a permissions 403 was read as a rate limit: %v", err)
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 403 {
		t.Errorf("want *HTTPError 403, got %v", err)
	}
}

func TestSecondaryLimitReadsRetryAfter(t *testing.T) {
	err := fetchErr(t, statusServer(t, 403, map[string]string{"Retry-After": "120"},
		`{"message":"You have exceeded a secondary rate limit."}`))
	limited, after := IsRateLimited(err)
	if !limited || after != 2*time.Minute {
		t.Errorf("IsRateLimited = %v, %v; want true, 2m", limited, after)
	}
}

func TestPrimaryLimitWaitsForReset(t *testing.T) {
	reset := time.Now().Add(10 * time.Minute).Unix()
	err := fetchErr(t, statusServer(t, 403, map[string]string{
		"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(reset, 10)}, `{}`))
	limited, after := IsRateLimited(err)
	if !limited || after < 9*time.Minute || after > 10*time.Minute {
		t.Errorf("IsRateLimited = %v, %v; want true, ~10m", limited, after)
	}
}

func TestTooManyRequestsIsALimit(t *testing.T) {
	if limited, _ := IsRateLimited(fetchErr(t, statusServer(t, 429, nil, ``))); !limited {
		t.Error("429 not read as a rate limit")
	}
}

func TestGraphQLErrorTypes(t *testing.T) {
	cases := []struct {
		body                         string
		resource, notFound, rateLimd bool
	}{
		{`{"data":null,"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED","message":"Resource limits for this query exceeded."}]}`, true, false, false},
		{`{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a Repository with the name 'o/r'."}]}`, false, true, false},
		{`{"data":null,"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`, false, false, true},
		{`{"data":{"repository":null}}`, false, true, false},
	}
	for _, c := range cases {
		cl, _, _ := fakeGraphQL(t, c.body)
		err := fetchErr(t, cl)
		limited, _ := IsRateLimited(err)
		if IsResourceLimit(err) != c.resource || IsNotFound(err) != c.notFound || limited != c.rateLimd {
			t.Errorf("%s: resource=%v notFound=%v rateLimited=%v", c.body, IsResourceLimit(err), IsNotFound(err), limited)
		}
	}
}

func TestUnreadableUpdatedAtFailsThePage(t *testing.T) {
	cl, _, _ := fakeGraphQL(t, `{"data":{"repository":{"issues":{"nodes":[{"number":7,"title":"x","updatedAt":"yesterday","createdAt":"","labels":{"nodes":[]},"comments":{"totalCount":0},"assignees":{"totalCount":0}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}},"rateLimit":{"limit":5000,"remaining":4999,"used":1,"resetAt":"2026-10-16T00:00:00Z","cost":1}}}`)
	if _, err := cl.FetchRepositoryIssues(context.Background(), "t", "o", "r", IssuesQuery{PageSize: 10}); err == nil {
		t.Fatal("an issue with an unreadable updatedAt was accepted")
	}
}
