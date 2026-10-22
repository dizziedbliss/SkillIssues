package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// LookupRepositories fetches up to 100 repositories by "owner/name" in one
// GraphQL request, one alias per repository, described exactly as a search
// describes them. A repository GitHub cannot resolve (deleted, private, or a
// malformed name) is absent from the result rather than failing the batch;
// any other per-repository error fails it.
func (c *Client) LookupRepositories(ctx context.Context, token string, names []string) (map[string]SearchedRepository, GraphQLRateLimit, error) {
	ctx = metrics.WithOperation(ctx, "LookupRepositories")
	if len(names) == 0 {
		return map[string]SearchedRepository{}, GraphQLRateLimit{}, nil
	}
	if len(names) > 100 {
		return nil, GraphQLRateLimit{}, fmt.Errorf("github: LookupRepositories takes at most 100 repositories, got %d", len(names))
	}

	var params, fields strings.Builder
	variables := make(map[string]any, len(names)*2)
	asked := make([]string, 0, len(names))
	for _, full := range names {
		owner, name, ok := strings.Cut(full, "/")
		if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
			continue // cannot name a repository; same as not found
		}
		i := len(asked)
		asked = append(asked, full)
		fmt.Fprintf(&params, "$o%d: String!, $n%d: String!, ", i, i)
		fmt.Fprintf(&fields, "r%d: repository(owner: $o%d, name: $n%d) { ...RepositoryFields }\n", i, i, i)
		variables[fmt.Sprintf("o%d", i)] = owner
		variables[fmt.Sprintf("n%d", i)] = name
	}
	if len(asked) == 0 {
		return map[string]SearchedRepository{}, GraphQLRateLimit{}, nil
	}
	query := "query LookupRepositories(" + strings.TrimSuffix(params.String(), ", ") + ") {\n" +
		fields.String() + "rateLimit { limit remaining used resetAt cost }\n}\n" + repositoryFragment

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

	out := make(map[string]SearchedRepository, len(asked))
	for i, full := range asked {
		raw, ok := rawResponse.Data[fmt.Sprintf("r%d", i)]
		if !ok || string(raw) == "null" {
			continue // not found
		}
		var node repositoryNode
		if err := json.Unmarshal(raw, &node); err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: parse repository %s: %w", full, err)
		}
		r, err := node.repository()
		if err != nil {
			return nil, GraphQLRateLimit{}, err
		}
		out[full] = r
	}
	return out, rl.parse(), nil
}
