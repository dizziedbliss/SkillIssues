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

// rawRateLimit is the rateLimit block every crawl query asks for.
type rawRateLimit struct {
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	Used      int    `json:"used"`
	ResetAt   string `json:"resetAt"`
	Cost      int    `json:"cost"`
}

func (r rawRateLimit) parse() GraphQLRateLimit {
	// A missing or malformed resetAt leaves the zero time, which CanProceed
	// reads as "already reset" — the budget gate fails open, never shut.
	resetAt, _ := time.Parse(time.RFC3339, r.ResetAt)
	return GraphQLRateLimit{
		Limit:     r.Limit,
		Remaining: r.Remaining,
		Used:      r.Used,
		ResetAt:   resetAt,
		Cost:      r.Cost,
	}
}

// postGraphQL sends one GraphQL request and decodes the response body into
// out, which must have `Errors []graphQLError` and `Data` fields the caller
// inspects. HTTP-level failures become errors whose text the ingest worker
// classifies ("rate limit exceeded", "HTTP 401", …), so keep those phrases.
func (c *Client) postGraphQL(ctx context.Context, token, query string, variables map[string]any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("github: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("github: create request: %w", err)
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
		return fmt.Errorf("github: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("github: HTTP 401 Unauthorized (Bad credentials) - the token is invalid, expired, or revoked")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("github: rate limit exceeded (HTTP %d), body unreadable: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("github: rate limit exceeded (HTTP %d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("github: HTTP %d, body unreadable: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: parse response: %w", err)
	}
	return nil
}
