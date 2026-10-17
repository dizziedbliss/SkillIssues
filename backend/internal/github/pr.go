// Package github provides the server's GitHub GraphQL client: crawling and
// issue-state checks, always with the server credential.
//
// All operations use the GitHub GraphQL API exclusively.
package github

import (
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// DefaultGraphQLURL is the default GitHub GraphQL API endpoint.
const DefaultGraphQLURL = "https://api.github.com/graphql"

// Client interacts with GitHub GraphQL API.
type Client struct {
	httpClient httpx.Doer
	graphqlURL string
}

// NewClient creates a new GitHub GraphQL client. timeout is read on every call,
// so the http.outbound_timeout_seconds setting applies without a restart.
func NewClient(graphqlURL string, timeout func() time.Duration) *Client {
	if graphqlURL == "" {
		graphqlURL = DefaultGraphQLURL
	}
	return &Client{
		httpClient: httpx.New(timeout, metrics.InstrumentedTransport{Base: flow.Transport{}}),
		graphqlURL: graphqlURL,
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
	// Type and Path identify a per-field error in a partial response, e.g.
	// NOT_FOUND on one alias of a batched query while the rest succeeded.
	Type string `json:"type,omitempty"`
	Path []any  `json:"path,omitempty"`
}
