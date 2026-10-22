package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// IssueRef names one issue on GitHub.
type IssueRef struct {
	Owner  string
	Name   string
	Number int
}

// TrackedIssueState is one result of FetchIssueStates.
type TrackedIssueState struct {
	// Found is false when GitHub has no such issue any more: the issue or
	// its repository was deleted, made private, or the issue transferred.
	Found     bool
	State     string // "OPEN" or "CLOSED"; empty when !Found
	UpdatedAt time.Time
}

// FetchIssueStates checks the open/closed state of up to 100 issues in one
// GraphQL request, one alias per issue. Results are in the order of refs.
// An issue GitHub cannot resolve comes back Found=false rather than failing
// the batch; any other per-issue error fails it.
func (c *Client) FetchIssueStates(ctx context.Context, token string, refs []IssueRef) ([]TrackedIssueState, GraphQLRateLimit, error) {
	ctx = metrics.WithOperation(ctx, "FetchIssueStates")
	if len(refs) == 0 {
		return nil, GraphQLRateLimit{}, nil
	}
	if len(refs) > 100 {
		return nil, GraphQLRateLimit{}, fmt.Errorf("github: FetchIssueStates takes at most 100 issues, got %d", len(refs))
	}

	var params, fields strings.Builder
	variables := make(map[string]any, len(refs)*3)
	for i, r := range refs {
		fmt.Fprintf(&params, "$o%d: String!, $n%d: String!, $k%d: Int!, ", i, i, i)
		fmt.Fprintf(&fields, "i%d: repository(owner: $o%d, name: $n%d) { issue(number: $k%d) { state updatedAt } }\n", i, i, i, i)
		variables[fmt.Sprintf("o%d", i)] = r.Owner
		variables[fmt.Sprintf("n%d", i)] = r.Name
		variables[fmt.Sprintf("k%d", i)] = r.Number
	}
	query := "query IssueStates(" + strings.TrimSuffix(params.String(), ", ") + ") {\n" +
		fields.String() + "rateLimit { limit remaining used resetAt cost }\n}"

	var rawResponse struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []graphQLError             `json:"errors"`
	}
	if err := c.postGraphQL(ctx, token, query, variables, &rawResponse); err != nil {
		return nil, GraphQLRateLimit{}, err
	}
	for _, e := range rawResponse.Errors {
		if e.Type != "NOT_FOUND" {
			return nil, GraphQLRateLimit{}, newGraphQLError(e)
		}
	}

	var rl rawRateLimit
	if raw, ok := rawResponse.Data["rateLimit"]; ok {
		if err := json.Unmarshal(raw, &rl); err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: parse rateLimit: %w", err)
		}
	}

	out := make([]TrackedIssueState, len(refs))
	for i, r := range refs {
		raw, ok := rawResponse.Data[fmt.Sprintf("i%d", i)]
		if !ok || string(raw) == "null" {
			continue // repository not found
		}
		var repo struct {
			Issue *struct {
				State     string `json:"state"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"issue"`
		}
		if err := json.Unmarshal(raw, &repo); err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: parse state of %s/%s#%d: %w", r.Owner, r.Name, r.Number, err)
		}
		if repo.Issue == nil {
			continue // issue not found
		}
		updatedAt, err := time.Parse(time.RFC3339, repo.Issue.UpdatedAt)
		if err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: parse updatedAt of %s/%s#%d: %w", r.Owner, r.Name, r.Number, err)
		}
		out[i] = TrackedIssueState{Found: true, State: strings.ToUpper(repo.Issue.State), UpdatedAt: updatedAt}
	}
	return out, rl.parse(), nil
}
