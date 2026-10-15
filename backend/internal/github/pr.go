// Package github provides a GitHub GraphQL client for PR verification,
// user identification, and repository operations.
//
// All operations use the GitHub GraphQL API exclusively.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// PullRequestInfo contains PR metadata retrieved from GitHub GraphQL.
type PullRequestInfo struct {
	Number      int        `json:"number"`
	State       string     `json:"state"` // OPEN, CLOSED, MERGED
	Merged      bool       `json:"merged"`
	MergedAt    *time.Time `json:"merged_at,omitempty"`
	AuthorLogin string     `json:"author_login"`
	HeadRefName string     `json:"head_ref_name"`
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

// GetPullRequest queries GitHub GraphQL for PR details.
// Query:
//
//	query GetPullRequest($owner: String!, $name: String!, $number: Int!) {
//	    repository(owner: $owner, name: $name) {
//	        pullRequest(number: $number) {
//	            number
//	            state
//	            merged
//	            mergedAt
//	            author { login }
//	            headRefName
//	        }
//	    }
//	}
func (c *Client) GetPullRequest(ctx context.Context, token, owner, name string, number int) (*PullRequestInfo, error) {
	ctx = metrics.WithOperation(ctx, "GetPullRequest")
	const query = `query GetPullRequest($owner: String!, $name: String!, $number: Int!) {
		repository(owner: $owner, name: $name) {
			pullRequest(number: $number) {
				number
				state
				merged
				mergedAt
				author {
					login
				}
				headRefName
			}
		}
	}`

	reqBody := graphQLRequest{
		Query: query,
		Variables: map[string]any{
			"owner":  owner,
			"name":   name,
			"number": number,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("github: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("github: create request: %w", err)
	}

	token = strings.Trim(strings.TrimSpace(token), "\"' \t\r\n")
	authHeader := token
	if !strings.HasPrefix(token, "Bearer ") && !strings.HasPrefix(token, "token ") {
		authHeader = "Bearer " + token
	}
	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "SkillIssues-Go/2.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("github: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data struct {
			Repository *struct {
				PullRequest *struct {
					Number   int        `json:"number"`
					State    string     `json:"state"`
					Merged   bool       `json:"merged"`
					MergedAt *time.Time `json:"mergedAt"`
					Author   *struct {
						Login string `json:"login"`
					} `json:"author"`
					HeadRefName string `json:"headRefName"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("github: parse response: %w", err)
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", response.Errors[0].Message)
	}

	if response.Data.Repository == nil || response.Data.Repository.PullRequest == nil {
		return nil, fmt.Errorf("github: pull request not found")
	}

	pr := response.Data.Repository.PullRequest
	author := ""
	if pr.Author != nil {
		author = pr.Author.Login
	}

	return &PullRequestInfo{
		Number:      pr.Number,
		State:       pr.State,
		Merged:      pr.Merged,
		MergedAt:    pr.MergedAt,
		AuthorLogin: author,
		HeadRefName: pr.HeadRefName,
	}, nil
}
