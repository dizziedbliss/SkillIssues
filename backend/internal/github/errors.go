package github

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPError is a GitHub response that was not 200. Callers classify it with
// IsRateLimited, IsResourceLimit and IsNotFound, never by its text.
type HTTPError struct {
	Status int
	Body   string
	// RateLimited is true for a 429, and for a 403 GitHub marks as a rate
	// limit (no requests left, a Retry-After, or a rate limit message). Any
	// other 403 is a refusal, not a reason to pause the credential.
	RateLimited bool
	// RetryAfter is how long GitHub asked to wait (Retry-After, or the
	// primary limit's reset when nothing is left); 0 when it did not say.
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	switch {
	case e.RateLimited:
		return fmt.Sprintf("github: rate limit exceeded (HTTP %d): %s", e.Status, e.Body)
	case e.Status == http.StatusUnauthorized:
		return "github: HTTP 401 Unauthorized (Bad credentials) - the token is invalid, expired, or revoked"
	default:
		return fmt.Sprintf("github: HTTP %d: %s", e.Status, e.Body)
	}
}

// GraphQLError is an entry of a GraphQL response's errors list, which GitHub
// sends with a 200: NOT_FOUND, RATE_LIMITED, RESOURCE_LIMITS_EXCEEDED, …
type GraphQLError struct {
	Type    string
	Message string
}

func (e *GraphQLError) Error() string {
	if e.Type == "" {
		return "github graphql: " + e.Message
	}
	return "github graphql: " + e.Type + ": " + e.Message
}

func newGraphQLError(e graphQLError) error {
	return &GraphQLError{Type: e.Type, Message: e.Message}
}

// ErrRepositoryNotFound is a query whose repository came back null: deleted,
// renamed away or private.
var ErrRepositoryNotFound = errors.New("github: repository not found")

// newHTTPError reads a non-200 response into an HTTPError.
func newHTTPError(resp *http.Response, body string, now time.Time) *HTTPError {
	e := &HTTPError{Status: resp.StatusCode, Body: body}
	retryAfter := resp.Header.Get("Retry-After")
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		e.RateLimited = true
	case http.StatusForbidden:
		e.RateLimited = remaining == "0" || retryAfter != "" ||
			strings.Contains(strings.ToLower(body), "rate limit")
	}
	if !e.RateLimited {
		return e
	}
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s > 0 {
		e.RetryAfter = time.Duration(s) * time.Second
	} else if remaining == "0" {
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			if d := time.Unix(reset, 0).Sub(now); d > 0 {
				e.RetryAfter = d
			}
		}
	}
	return e
}

// IsRateLimited reports whether err is GitHub refusing for a rate limit, and
// how long it asked to wait (0 when it did not say).
func IsRateLimited(err error) (bool, time.Duration) {
	var he *HTTPError
	if errors.As(err, &he) && he.RateLimited {
		return true, he.RetryAfter
	}
	var ge *GraphQLError
	if errors.As(err, &ge) && ge.Type == "RATE_LIMITED" {
		return true, 0
	}
	return false, 0
}

// IsResourceLimit reports whether a smaller query might succeed: GitHub found
// the query too expensive, timed out computing it (502/504), or the call ran
// past its own deadline. A cancelled context is not one: the caller gave up.
func IsResourceLimit(err error) bool {
	var ge *GraphQLError
	if errors.As(err, &ge) && ge.Type == "RESOURCE_LIMITS_EXCEEDED" {
		return true
	}
	var he *HTTPError
	if errors.As(err, &he) && (he.Status == http.StatusBadGateway || he.Status == http.StatusGatewayTimeout) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

// IsNotFound reports whether the repository a query named does not exist for
// this credential.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrRepositoryNotFound) {
		return true
	}
	var ge *GraphQLError
	return errors.As(err, &ge) && ge.Type == "NOT_FOUND"
}
