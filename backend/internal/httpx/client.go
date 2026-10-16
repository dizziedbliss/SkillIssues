// Package httpx holds the outbound HTTP client the server uses for calls to
// GitHub and to its own sibling services.
//
// http.DefaultClient has no timeout, so one stalled upstream pins a goroutine
// (and, in the sign-in path, a user's request) indefinitely. This client takes
// its timeout from a function, so the live setting applies to the next call
// without a restart.
package httpx

import (
	"errors"
	"net/http"
	"time"
)

// Doer is what the callers need from a client.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// ErrNoTimeout is returned instead of making a call that would have no deadline.
var ErrNoTimeout = errors.New("httpx: outbound timeout is not configured")

// Client applies timeout() to every request it sends.
type Client struct {
	transport http.RoundTripper
	timeout   func() time.Duration
}

// New returns a Client. A nil transport uses http.DefaultTransport.
func New(timeout func() time.Duration, transport http.RoundTripper) *Client {
	return &Client{transport: transport, timeout: timeout}
}

// Do sends req with the current timeout. It refuses to send without one.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.timeout == nil {
		return nil, ErrNoTimeout
	}
	d := c.timeout()
	if d <= 0 {
		return nil, ErrNoTimeout
	}
	return (&http.Client{Transport: c.transport, Timeout: d}).Do(req)
}
