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

// ForkResult holds repository fork information.
type ForkResult struct {
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	HTMLURL  string `json:"html_url"`
}

// CreateFork creates or retrieves a fork of the target repository.
func (c *Client) CreateFork(ctx context.Context, token, owner, name string) (*ForkResult, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/forks", owner, name)
	reqBody := map[string]any{"default_branch_only": true}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	authHeader := token
	if !strings.HasPrefix(token, "Bearer ") && !strings.HasPrefix(token, "token ") {
		authHeader = "Bearer " + token
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SkillIssues-Go/2.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var res ForkResult
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}
		return &res, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("github fork: HTTP %d: %s", resp.StatusCode, string(body))
}

// PullRequestCreateResult holds the response after creating a PR.
type PullRequestCreateResult struct {
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
}

// CreatePullRequest opens a pull request on GitHub.
func (c *Client) CreatePullRequest(ctx context.Context, token, owner, name, title, head, base, body string) (*PullRequestCreateResult, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, name)
	payload, _ := json.Marshal(map[string]any{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  body,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	authHeader := token
	if !strings.HasPrefix(token, "Bearer ") && !strings.HasPrefix(token, "token ") {
		authHeader = "Bearer " + token
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SkillIssues-Go/2.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		var pr struct {
			HTMLURL string `json:"html_url"`
			Number  int    `json:"number"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			return nil, err
		}
		return &PullRequestCreateResult{HTMLURL: pr.HTMLURL, Number: pr.Number}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("github create pr: HTTP %d: %s", resp.StatusCode, string(respBody))
}
