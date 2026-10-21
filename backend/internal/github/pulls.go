package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// UserPullsQuery asks for one person's pull requests, newest-updated first.
type UserPullsQuery struct {
	Login string
	First int
	After *string // nil: the first page
}

// UserPullsPage is one person's page of pull requests.
type UserPullsPage struct {
	// Found is false when GitHub has no account by that login any more.
	Found bool
	// DatabaseID is the account's numeric id: the caller checks it is still
	// the person it meant, since a login can pass to someone else.
	DatabaseID  int64
	Pulls       []UserPullRequest
	HasNextPage bool
	EndCursor   string
}

// UserPullRequest is a pull request as the tracker stores it.
type UserPullRequest struct {
	DatabaseID        int64
	Number            int
	Title             string
	URL               string
	State             string // OPEN, CLOSED or MERGED
	CreatedAt         time.Time
	UpdatedAt         time.Time
	MergedAt          *time.Time
	ClosedAt          *time.Time
	AuthorAssociation string
	Additions         int
	Deletions         int
	// Checks is the last commit's check rollup, or nil with no checks.
	Checks           *string
	Approvals        int
	ChangesRequested int
	// RepositoryID is GitHub's numeric repository id; Repository its name.
	RepositoryID int64
	Repository   string
	// OwnerID is the repository owner's account id when the owner is a
	// person (nil for an organisation).
	OwnerID *int64
	Closes  []ClosingIssue
}

// ClosingIssue is an issue a pull request says it closes.
type ClosingIssue struct {
	RepositoryID int64
	Repository   string
	Number       int
}

// pullFields is what one pull request costs: four small nested connections.
const pullFields = `databaseId number title url state createdAt updatedAt mergedAt closedAt authorAssociation additions deletions
        repository { databaseId nameWithOwner owner { __typename ... on User { databaseId } } }
        approved: reviews(states: APPROVED, first: 1) { totalCount }
        changes: reviews(states: CHANGES_REQUESTED, first: 1) { totalCount }
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
        closingIssuesReferences(first: 5) { nodes { number repository { databaseId nameWithOwner } } }`

// FetchUserPullRequests reads a page of pull requests for each of up to 50
// people in one request, one alias each. Results are in the order of qs; a
// login GitHub doesn't know comes back Found=false without failing the rest.
func (c *Client) FetchUserPullRequests(ctx context.Context, token string, qs []UserPullsQuery) ([]UserPullsPage, GraphQLRateLimit, error) {
	ctx = metrics.WithOperation(ctx, "FetchUserPullRequests")
	if len(qs) == 0 {
		return nil, GraphQLRateLimit{}, nil
	}
	if len(qs) > 50 {
		return nil, GraphQLRateLimit{}, fmt.Errorf("github: FetchUserPullRequests takes at most 50 people, got %d", len(qs))
	}
	var params, fields strings.Builder
	variables := make(map[string]any, len(qs)*3)
	for i, q := range qs {
		fmt.Fprintf(&params, "$l%d: String!, $f%d: Int!, $a%d: String, ", i, i, i)
		fmt.Fprintf(&fields, "u%d: user(login: $l%d) {\n    databaseId\n    pullRequests(first: $f%d, after: $a%d, orderBy: {field: UPDATED_AT, direction: DESC}) {\n      pageInfo { hasNextPage endCursor }\n      nodes { %s }\n    }\n  }\n",
			i, i, i, i, pullFields)
		variables[fmt.Sprintf("l%d", i)] = q.Login
		variables[fmt.Sprintf("f%d", i)] = clampPageSize(q.First)
		if q.After != nil {
			variables[fmt.Sprintf("a%d", i)] = *q.After
		}
	}
	query := "query UserPulls(" + strings.TrimSuffix(params.String(), ", ") + ") {\n  " +
		fields.String() + "  rateLimit { limit remaining used resetAt cost }\n}"

	var raw struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []graphQLError             `json:"errors"`
	}
	if err := c.postGraphQL(ctx, token, query, variables, &raw); err != nil {
		return nil, GraphQLRateLimit{}, err
	}
	for _, e := range raw.Errors {
		if e.Type != "NOT_FOUND" {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github graphql: %s", e.Message)
		}
	}
	var rl rawRateLimit
	if r, ok := raw.Data["rateLimit"]; ok {
		if err := json.Unmarshal(r, &rl); err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: parse rateLimit: %w", err)
		}
	}

	out := make([]UserPullsPage, len(qs))
	for i, q := range qs {
		r, ok := raw.Data[fmt.Sprintf("u%d", i)]
		if !ok || string(r) == "null" {
			continue // no such account
		}
		page, err := parseUserPulls(r)
		if err != nil {
			return nil, GraphQLRateLimit{}, fmt.Errorf("github: pull requests of %s: %w", q.Login, err)
		}
		out[i] = page
	}
	return out, rl.parse(), nil
}

type rawPull struct {
	DatabaseID        int64   `json:"databaseId"`
	Number            int     `json:"number"`
	Title             string  `json:"title"`
	URL               string  `json:"url"`
	State             string  `json:"state"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	MergedAt          *string `json:"mergedAt"`
	ClosedAt          *string `json:"closedAt"`
	AuthorAssociation string  `json:"authorAssociation"`
	Additions         int     `json:"additions"`
	Deletions         int     `json:"deletions"`
	Repository        *struct {
		DatabaseID    int64  `json:"databaseId"`
		NameWithOwner string `json:"nameWithOwner"`
		Owner         struct {
			Typename   string `json:"__typename"`
			DatabaseID *int64 `json:"databaseId"`
		} `json:"owner"`
	} `json:"repository"`
	Approved struct {
		TotalCount int `json:"totalCount"`
	} `json:"approved"`
	Changes struct {
		TotalCount int `json:"totalCount"`
	} `json:"changes"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Closing struct {
		Nodes []struct {
			Number     int `json:"number"`
			Repository *struct {
				DatabaseID    int64  `json:"databaseId"`
				NameWithOwner string `json:"nameWithOwner"`
			} `json:"repository"`
		} `json:"nodes"`
	} `json:"closingIssuesReferences"`
}

func parseUserPulls(r json.RawMessage) (UserPullsPage, error) {
	var u struct {
		DatabaseID   int64 `json:"databaseId"`
		PullRequests struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []*rawPull `json:"nodes"`
		} `json:"pullRequests"`
	}
	if err := json.Unmarshal(r, &u); err != nil {
		return UserPullsPage{}, err
	}
	page := UserPullsPage{
		Found: true, DatabaseID: u.DatabaseID,
		HasNextPage: u.PullRequests.PageInfo.HasNextPage, EndCursor: u.PullRequests.PageInfo.EndCursor,
	}
	for _, n := range u.PullRequests.Nodes {
		if n == nil || n.Repository == nil {
			continue // a pull request on a repository since deleted or made private
		}
		p, err := n.pull()
		if err != nil {
			return UserPullsPage{}, fmt.Errorf("#%d: %w", n.Number, err)
		}
		page.Pulls = append(page.Pulls, p)
	}
	return page, nil
}

func (n *rawPull) pull() (UserPullRequest, error) {
	created, err := time.Parse(time.RFC3339, n.CreatedAt)
	if err != nil {
		return UserPullRequest{}, fmt.Errorf("parse createdAt: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, n.UpdatedAt)
	if err != nil {
		return UserPullRequest{}, fmt.Errorf("parse updatedAt: %w", err)
	}
	merged, err := parseOptionalTime(n.MergedAt)
	if err != nil {
		return UserPullRequest{}, fmt.Errorf("parse mergedAt: %w", err)
	}
	closed, err := parseOptionalTime(n.ClosedAt)
	if err != nil {
		return UserPullRequest{}, fmt.Errorf("parse closedAt: %w", err)
	}
	p := UserPullRequest{
		DatabaseID: n.DatabaseID, Number: n.Number, Title: n.Title, URL: n.URL,
		State: strings.ToUpper(n.State), CreatedAt: created, UpdatedAt: updated, MergedAt: merged, ClosedAt: closed,
		AuthorAssociation: n.AuthorAssociation, Additions: n.Additions, Deletions: n.Deletions,
		Approvals: n.Approved.TotalCount, ChangesRequested: n.Changes.TotalCount,
		RepositoryID: n.Repository.DatabaseID, Repository: n.Repository.NameWithOwner,
	}
	if n.Repository.Owner.Typename == "User" {
		p.OwnerID = n.Repository.Owner.DatabaseID
	}
	if len(n.Commits.Nodes) > 0 && n.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
		s := n.Commits.Nodes[0].Commit.StatusCheckRollup.State
		p.Checks = &s
	}
	for _, c := range n.Closing.Nodes {
		if c.Repository != nil {
			p.Closes = append(p.Closes, ClosingIssue{RepositoryID: c.Repository.DatabaseID, Repository: c.Repository.NameWithOwner, Number: c.Number})
		}
	}
	return p, nil
}
