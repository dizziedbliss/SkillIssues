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

// IssueBody is the result of an on-demand single-issue body fetch (plan
// §3.1). This is the only place issue body text is ever fetched from
// GitHub — D17 removed it from the broad ingest query entirely.
type IssueBody struct {
	Body      string
	RateLimit GraphQLRateLimit
}

const issueBodyQuery = `query IssueBody($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
        issue(number: $number) {
            body
        }
    }
    rateLimit { limit remaining used resetAt cost }
}`

// FetchIssueBody fetches the body of exactly one issue. Callers are
// responsible for single-flight coalescing and caching the result — this
// function makes no attempt at either.
func (c *Client) FetchIssueBody(ctx context.Context, token, owner, name string, number int) (*IssueBody, error) {
	ctx = metrics.WithOperation(ctx, "FetchIssueBody")
	reqBody := graphQLRequest{
		Query: issueBodyQuery,
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("github: HTTP 401 Unauthorized (Bad credentials)")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: rate limit exceeded (HTTP %d): %s", resp.StatusCode, string(b))
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var rawResponse struct {
		Data struct {
			Repository *struct {
				Issue *struct {
					Body string `json:"body"`
				} `json:"issue"`
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
	if rawResponse.Data.Repository == nil || rawResponse.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("github: issue not found")
	}

	resetTime, _ := time.Parse(time.RFC3339, rawResponse.Data.RateLimit.ResetAt)
	return &IssueBody{
		Body: rawResponse.Data.Repository.Issue.Body,
		RateLimit: GraphQLRateLimit{
			Limit:     rawResponse.Data.RateLimit.Limit,
			Remaining: rawResponse.Data.RateLimit.Remaining,
			Used:      rawResponse.Data.RateLimit.Used,
			ResetAt:   resetTime,
			Cost:      rawResponse.Data.RateLimit.Cost,
		},
	}, nil
}
