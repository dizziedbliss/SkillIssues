package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// IssueState is the result of a live, on-demand single-issue state check
// (D16) — made at the moment a user commits to an issue (fork/clone),
// never on list render or detail open, using the user's own credential.
type IssueState struct {
	State string // "OPEN" or "CLOSED"
}

const issueStateQuery = `query IssueState($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
        issue(number: $number) {
            state
        }
    }
}`

// FetchIssueState fetches the live open/closed state of exactly one issue.
func (c *Client) FetchIssueState(ctx context.Context, token, owner, name string, number int) (*IssueState, error) {
	reqBody := graphQLRequest{
		Query: issueStateQuery,
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
					State string `json:"state"`
				} `json:"issue"`
			} `json:"repository"`
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

	return &IssueState{State: strings.ToUpper(rawResponse.Data.Repository.Issue.State)}, nil
}
