package github

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// SearchResultCap is the most results GitHub search returns for one query,
// however many match: a search reporting more must be narrowed to see them
// all.
const SearchResultCap = 1000

// SearchedRepository is one repository from a repository search, with what
// eligibility needs and what cmd/repoimport stores for a new row.
type SearchedRepository struct {
	DatabaseID      int64
	NameWithOwner   string
	URL             string
	Description     *string
	PrimaryLanguage *string
	Languages       []RepositoryLanguage
	Topics          []string
	License         *string
	Stars           int
	Forks           int
	Watchers        int
	IsFork          bool
	IsArchived      bool
	ForkingAllowed  bool
	IssueCount      int // open + closed
	OpenIssueCount  int
	CreatedAt       *time.Time
	PushedAt        *time.Time
}

// RepositoryLanguage matches the dataset's languages JSON ({name, size}).
type RepositoryLanguage struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// RepositorySearchPage is one page of a repository search.
type RepositorySearchPage struct {
	// RepositoryCount is how many repositories match the query in total,
	// not how many this page — or the whole search — can return.
	RepositoryCount int
	Repositories    []SearchedRepository
	HasNextPage     bool
	EndCursor       string
	RateLimit       GraphQLRateLimit
}

const searchRepositoriesQuery = `query SearchRepositories($q: String!, $first: Int!, $after: String) {
    search(query: $q, type: REPOSITORY, first: $first, after: $after) {
        repositoryCount
        pageInfo { hasNextPage endCursor }
        nodes {
            ... on Repository {
                databaseId
                nameWithOwner
                url
                description
                primaryLanguage { name }
                languages(first: 10, orderBy: {field: SIZE, direction: DESC}) { edges { size node { name } } }
                repositoryTopics(first: 20) { nodes { topic { name } } }
                licenseInfo { spdxId }
                stargazerCount
                forkCount
                watchers { totalCount }
                isFork
                isArchived
                forkingAllowed
                issues { totalCount }
                openIssues: issues(states: OPEN) { totalCount }
                createdAt
                pushedAt
            }
        }
    }
    rateLimit { limit remaining used resetAt cost }
}`

// SearchRepositories runs one page of a GitHub repository search. query is
// GitHub search syntax (e.g. "stars:>=25 pushed:2026-01-01..2026-01-02").
func (c *Client) SearchRepositories(ctx context.Context, token, query string, first int, after *string) (*RepositorySearchPage, error) {
	ctx = metrics.WithOperation(ctx, "SearchRepositories")
	variables := map[string]any{
		"q":     query,
		"first": clampPageSize(first),
	}
	if after != nil && *after != "" {
		variables["after"] = *after
	}

	type totalCount struct {
		TotalCount int `json:"totalCount"`
	}
	var raw struct {
		Data struct {
			Search *struct {
				RepositoryCount int `json:"repositoryCount"`
				PageInfo        struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					DatabaseID      int64   `json:"databaseId"`
					NameWithOwner   string  `json:"nameWithOwner"`
					URL             string  `json:"url"`
					Description     *string `json:"description"`
					PrimaryLanguage *struct {
						Name string `json:"name"`
					} `json:"primaryLanguage"`
					Languages struct {
						Edges []struct {
							Size int64 `json:"size"`
							Node struct {
								Name string `json:"name"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"languages"`
					RepositoryTopics struct {
						Nodes []struct {
							Topic struct {
								Name string `json:"name"`
							} `json:"topic"`
						} `json:"nodes"`
					} `json:"repositoryTopics"`
					LicenseInfo *struct {
						SpdxID *string `json:"spdxId"`
					} `json:"licenseInfo"`
					StargazerCount int        `json:"stargazerCount"`
					ForkCount      int        `json:"forkCount"`
					Watchers       totalCount `json:"watchers"`
					IsFork         bool       `json:"isFork"`
					IsArchived     bool       `json:"isArchived"`
					ForkingAllowed bool       `json:"forkingAllowed"`
					Issues         totalCount `json:"issues"`
					OpenIssues     totalCount `json:"openIssues"`
					CreatedAt      *string    `json:"createdAt"`
					PushedAt       *string    `json:"pushedAt"`
				} `json:"nodes"`
			} `json:"search"`
			RateLimit rawRateLimit `json:"rateLimit"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	if err := c.postGraphQL(ctx, token, searchRepositoriesQuery, variables, &raw); err != nil {
		return nil, err
	}
	if len(raw.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", raw.Errors[0].Message)
	}
	if raw.Data.Search == nil {
		return nil, fmt.Errorf("github: search returned no result")
	}
	s := raw.Data.Search

	repos := make([]SearchedRepository, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		// Search nodes are a union; a non-repository (never expected for
		// type: REPOSITORY) decodes with no name and is skipped.
		if n.NameWithOwner == "" {
			continue
		}
		r := SearchedRepository{
			DatabaseID:     n.DatabaseID,
			NameWithOwner:  n.NameWithOwner,
			URL:            n.URL,
			Description:    n.Description,
			Stars:          n.StargazerCount,
			Forks:          n.ForkCount,
			Watchers:       n.Watchers.TotalCount,
			IsFork:         n.IsFork,
			IsArchived:     n.IsArchived,
			ForkingAllowed: n.ForkingAllowed,
			IssueCount:     n.Issues.TotalCount,
			OpenIssueCount: n.OpenIssues.TotalCount,
		}
		if n.PrimaryLanguage != nil && n.PrimaryLanguage.Name != "" {
			name := n.PrimaryLanguage.Name
			r.PrimaryLanguage = &name
		}
		for _, e := range n.Languages.Edges {
			r.Languages = append(r.Languages, RepositoryLanguage{Name: e.Node.Name, Size: e.Size})
		}
		for _, t := range n.RepositoryTopics.Nodes {
			r.Topics = append(r.Topics, t.Topic.Name)
		}
		if n.LicenseInfo != nil {
			r.License = n.LicenseInfo.SpdxID
		}
		var err error
		if r.CreatedAt, err = parseOptionalTime(n.CreatedAt); err != nil {
			return nil, fmt.Errorf("github: parse createdAt of %s: %w", n.NameWithOwner, err)
		}
		if r.PushedAt, err = parseOptionalTime(n.PushedAt); err != nil {
			return nil, fmt.Errorf("github: parse pushedAt of %s: %w", n.NameWithOwner, err)
		}
		repos = append(repos, r)
	}

	return &RepositorySearchPage{
		RepositoryCount: s.RepositoryCount,
		Repositories:    repos,
		HasNextPage:     s.PageInfo.HasNextPage,
		EndCursor:       s.PageInfo.EndCursor,
		RateLimit:       raw.Data.RateLimit.parse(),
	}, nil
}

func parseOptionalTime(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
