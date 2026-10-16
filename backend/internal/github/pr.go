// Package github provides the server's GitHub GraphQL client: crawling and
// issue-state checks, always with the server credential.
//
// All operations use the GitHub GraphQL API exclusively.
package github

import (
	"net/http"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// DefaultGraphQLURL is the default GitHub GraphQL API endpoint.
const DefaultGraphQLURL = "https://api.github.com/graphql"

// Client interacts with GitHub GraphQL API.
type Client struct {
	httpClient *http.Client
	graphqlURL string
}

// NewClient creates a new GitHub GraphQL client.
func NewClient(graphqlURL string) *Client {
	if graphqlURL == "" {
		graphqlURL = DefaultGraphQLURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second, Transport: metrics.InstrumentedTransport{}},
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
