package github

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// IssueBody is one issue's body as GitHub has it now.
type IssueBody struct {
	// Found is false when GitHub has no such issue any more: it, or its
	// repository, was deleted, made private or transferred.
	Found     bool
	Body      string
	UpdatedAt time.Time
}

const issueBodyQuery = `query IssueBody($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) { issue(number: $number) { body updatedAt } }
    rateLimit { limit remaining used resetAt cost }
}`

// FetchIssueBody reads one issue's Markdown body.
func (c *Client) FetchIssueBody(ctx context.Context, token string, ref IssueRef) (IssueBody, GraphQLRateLimit, error) {
	ctx = metrics.WithOperation(ctx, "FetchIssueBody")
	var raw struct {
		Data struct {
			Repository *struct {
				Issue *struct {
					Body      string `json:"body"`
					UpdatedAt string `json:"updatedAt"`
				} `json:"issue"`
			} `json:"repository"`
			RateLimit rawRateLimit `json:"rateLimit"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	variables := map[string]any{"owner": ref.Owner, "name": ref.Name, "number": ref.Number}
	if err := c.postGraphQL(ctx, token, issueBodyQuery, variables, &raw); err != nil {
		return IssueBody{}, GraphQLRateLimit{}, err
	}
	for _, e := range raw.Errors {
		if e.Type != "NOT_FOUND" {
			return IssueBody{}, GraphQLRateLimit{}, fmt.Errorf("github graphql: %s", e.Message)
		}
	}
	rl := raw.Data.RateLimit.parse()
	if raw.Data.Repository == nil || raw.Data.Repository.Issue == nil {
		return IssueBody{}, rl, nil
	}
	updated, err := time.Parse(time.RFC3339, raw.Data.Repository.Issue.UpdatedAt)
	if err != nil {
		return IssueBody{}, rl, fmt.Errorf("github: parse updatedAt of %s/%s#%d: %w", ref.Owner, ref.Name, ref.Number, err)
	}
	return IssueBody{Found: true, Body: raw.Data.Repository.Issue.Body, UpdatedAt: updated}, rl, nil
}
