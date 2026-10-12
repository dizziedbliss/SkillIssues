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
)

// GraphQLRateLimit encapsulates GitHub GraphQL rate limit telemetry.
type GraphQLRateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Used      int       `json:"used"`
	ResetAt   time.Time `json:"reset_at"`
	Cost      int       `json:"cost"`
}

// GraphQLIssueNode represents an individual GitHub issue item.
type GraphQLIssueNode struct {
	DatabaseID    int64     `json:"database_id"`
	Number        int       `json:"number"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	URL           string    `json:"url"`
	State         string    `json:"state"`
	UpdatedAt     time.Time `json:"updated_at"`
	AuthorLogin   string    `json:"author_login"`
	CommentsCount int       `json:"comments_count"`
	Labels        []string  `json:"labels"`
}

// RepositoryIssuesPage represents a single paginated batch of repository issues.
type RepositoryIssuesPage struct {
	Issues         []GraphQLIssueNode `json:"issues"`
	HasNextPage    bool               `json:"has_next_page"`
	EndCursor      string             `json:"end_cursor"`
	StargazerCount int                `json:"stargazer_count"`
	RateLimit      GraphQLRateLimit   `json:"rate_limit"`
}

const repositoryIssuesQuery = `query RepositoryIssues($owner: String!, $name: String!, $after: String, $since: DateTime, $first: Int!) {
    repository(owner: $owner, name: $name) {
        issues(first: $first, after: $after, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}, filterBy: {since: $since}) {
            nodes {
                databaseId
                number
                title
                body
                url
                state
                updatedAt
                author { login }
                comments { totalCount }
                labels(first: 10) { nodes { name } }
            }
            pageInfo { hasNextPage endCursor }
        }
        stargazerCount
    }
    rateLimit { limit remaining used resetAt cost }
}`

// FetchRepositoryIssues queries the GitHub GraphQL API for open issues in a repository.
func (c *Client) FetchRepositoryIssues(ctx context.Context, token, owner, name string, after *string, since *time.Time, pageSize int) (*RepositoryIssuesPage, error) {
	// GitHub rejects anything outside 1..100, and an oversized page also raises
	// the chance of the query exceeding the 10s limit.
	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}

	variables := map[string]any{
		"owner": owner,
		"name":  name,
		"first": pageSize,
	}
	if after != nil && *after != "" {
		variables["after"] = *after
	}
	if since != nil && !since.IsZero() {
		variables["since"] = since.Format(time.RFC3339)
	}

	reqBody := graphQLRequest{
		Query:     repositoryIssuesQuery,
		Variables: variables,
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("github: HTTP 401 Unauthorized (Bad credentials) - GITHUB_TOKEN is invalid, expired, or revoked on github.com")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: rate limit exceeded (HTTP %d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rawResponse struct {
		Data struct {
			Repository *struct {
				Issues struct {
					Nodes []struct {
						DatabaseID int64  `json:"databaseId"`
						Number     int    `json:"number"`
						Title      string `json:"title"`
						Body       string `json:"body"`
						URL        string `json:"url"`
						State      string `json:"state"`
						UpdatedAt  string `json:"updatedAt"`
						Author     *struct {
							Login string `json:"login"`
						} `json:"author"`
						Comments struct {
							TotalCount int `json:"totalCount"`
						} `json:"comments"`
						Labels struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"issues"`
				StargazerCount int `json:"stargazerCount"`
			} `json:"repository"`
			RateLimit struct {
				Limit     int    `json:"limit"`
				Remaining int    `json:"remaining"`
				Used      int    `json:"used"`
				ResetAt   string `json:"resetAt"`
				Cost      int    `json:"cost"`
			} `json:"rateLimit"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("github: parse response: %w", err)
	}

	if len(rawResponse.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", rawResponse.Errors[0].Message)
	}

	resetTime, _ := time.Parse(time.RFC3339, rawResponse.Data.RateLimit.ResetAt)
	rateLimit := GraphQLRateLimit{
		Limit:     rawResponse.Data.RateLimit.Limit,
		Remaining: rawResponse.Data.RateLimit.Remaining,
		Used:      rawResponse.Data.RateLimit.Used,
		ResetAt:   resetTime,
		Cost:      rawResponse.Data.RateLimit.Cost,
	}

	if rawResponse.Data.Repository == nil {
		return nil, fmt.Errorf("github: repository not found")
	}

	var issues []GraphQLIssueNode
	for _, n := range rawResponse.Data.Repository.Issues.Nodes {
		updatedAt, _ := time.Parse(time.RFC3339, n.UpdatedAt)
		author := ""
		if n.Author != nil {
			author = n.Author.Login
		}
		var labelNames []string
		for _, l := range n.Labels.Nodes {
			if l.Name != "" {
				labelNames = append(labelNames, l.Name)
			}
		}

		issues = append(issues, GraphQLIssueNode{
			DatabaseID:    n.DatabaseID,
			Number:        n.Number,
			Title:         n.Title,
			Body:          n.Body,
			URL:           n.URL,
			State:         strings.ToUpper(n.State),
			UpdatedAt:     updatedAt,
			AuthorLogin:   author,
			CommentsCount: n.Comments.TotalCount,
			Labels:        labelNames,
		})
	}

	return &RepositoryIssuesPage{
		Issues:         issues,
		HasNextPage:    rawResponse.Data.Repository.Issues.PageInfo.HasNextPage,
		EndCursor:      rawResponse.Data.Repository.Issues.PageInfo.EndCursor,
		StargazerCount: rawResponse.Data.Repository.StargazerCount,
		RateLimit:      rateLimit,
	}, nil
}
