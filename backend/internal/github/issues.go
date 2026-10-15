package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
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
//
// There is no Body field: D17 removes issue bodies from the ingest query
// entirely (they're ~92% of the payload and no ingestion-time hot path
// needs them — see internal/ingest for the on-demand read path instead).
type GraphQLIssueNode struct {
	DatabaseID    int64     `json:"database_id"`
	Number        int       `json:"number"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	State         string    `json:"state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AuthorLogin   string    `json:"author_login"`
	CommentsCount int       `json:"comments_count"`
	Labels        []string  `json:"labels"`
}

// RepositoryIssuesPage is one page of a repository's open issues and, when
// asked for, one page of the issues closed since the pass's watermark.
type RepositoryIssuesPage struct {
	Issues         []GraphQLIssueNode `json:"issues"`
	HasNextPage    bool               `json:"has_next_page"`
	EndCursor      string             `json:"end_cursor"`
	StargazerCount int                `json:"stargazer_count"`
	RateLimit      GraphQLRateLimit   `json:"rate_limit"`

	ClosedIssues      []ClosedIssue `json:"closed_issues"`
	ClosedHasNextPage bool          `json:"closed_has_next_page"`
	ClosedEndCursor   string        `json:"closed_end_cursor"`
}

// ClosedIssue is an issue GitHub reports as closed. UpdatedAt is GitHub's
// own updatedAt — closing an issue updates it, so it is the closing time
// unless something touched the issue after.
type ClosedIssue struct {
	Number    int       `json:"number"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IssuesQuery selects what one FetchRepositoryIssues call returns.
type IssuesQuery struct {
	// FetchOpen requests a page of open issues from OpenCursor.
	FetchOpen  bool
	OpenCursor *string
	PageSize   int

	// Since limits both connections to issues updated at or after it. Nil
	// fetches every open issue (a repository's first, full pass).
	Since *time.Time

	// FetchClosed requests a page of issues closed since Since, from
	// ClosedCursor. Meaningless without Since: a full pass has nothing
	// stored yet that could have closed.
	FetchClosed    bool
	ClosedCursor   *string
	ClosedPageSize int
}

const repositoryIssuesQuery = `query RepositoryIssues($owner: String!, $name: String!, $after: String, $since: DateTime, $first: Int!, $withOpen: Boolean!, $withClosed: Boolean!, $closedAfter: String, $closedFirst: Int!) {
    repository(owner: $owner, name: $name) {
        issues(first: $first, after: $after, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}, filterBy: {since: $since}) @include(if: $withOpen) {
            nodes {
                databaseId
                number
                title
                url
                state
                createdAt
                updatedAt
                author { login }
                comments { totalCount }
                labels(first: 20) { nodes { name } }
            }
            pageInfo { hasNextPage endCursor }
        }
        closed: issues(first: $closedFirst, after: $closedAfter, states: CLOSED, orderBy: {field: UPDATED_AT, direction: DESC}, filterBy: {since: $since}) @include(if: $withClosed) {
            nodes { number updatedAt }
            pageInfo { hasNextPage endCursor }
        }
        stargazerCount
    }
    rateLimit { limit remaining used resetAt cost }
}`

// clampPageSize keeps a page size inside GitHub's 1..100 range; an
// oversized page also raises the chance of the query exceeding the 10s limit.
func clampPageSize(n int) int {
	if n < 1 {
		return 1
	}
	if n > 100 {
		return 100
	}
	return n
}

// FetchRepositoryIssues fetches one page of a repository's open issues
// and/or of the issues closed since q.Since, in a single request.
func (c *Client) FetchRepositoryIssues(ctx context.Context, token, owner, name string, q IssuesQuery) (*RepositoryIssuesPage, error) {
	ctx = metrics.WithOperation(ctx, "FetchRepositoryIssues")
	fetchClosed := q.FetchClosed && q.Since != nil && !q.Since.IsZero()

	variables := map[string]any{
		"owner":       owner,
		"name":        name,
		"first":       clampPageSize(q.PageSize),
		"withOpen":    q.FetchOpen,
		"withClosed":  fetchClosed,
		"closedFirst": clampPageSize(q.ClosedPageSize),
	}
	if q.OpenCursor != nil && *q.OpenCursor != "" {
		variables["after"] = *q.OpenCursor
	}
	if q.ClosedCursor != nil && *q.ClosedCursor != "" {
		variables["closedAfter"] = *q.ClosedCursor
	}
	if q.Since != nil && !q.Since.IsZero() {
		variables["since"] = q.Since.UTC().Format(time.RFC3339)
	}

	type pageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	}
	var rawResponse struct {
		Data struct {
			Repository *struct {
				Issues struct {
					Nodes []struct {
						DatabaseID int64  `json:"databaseId"`
						Number     int    `json:"number"`
						Title      string `json:"title"`
						URL        string `json:"url"`
						State      string `json:"state"`
						CreatedAt  string `json:"createdAt"`
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
					PageInfo pageInfo `json:"pageInfo"`
				} `json:"issues"`
				Closed struct {
					Nodes []struct {
						Number    int    `json:"number"`
						UpdatedAt string `json:"updatedAt"`
					} `json:"nodes"`
					PageInfo pageInfo `json:"pageInfo"`
				} `json:"closed"`
				StargazerCount int `json:"stargazerCount"`
			} `json:"repository"`
			RateLimit rawRateLimit `json:"rateLimit"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	if err := c.postGraphQL(ctx, token, repositoryIssuesQuery, variables, &rawResponse); err != nil {
		return nil, err
	}
	if len(rawResponse.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", rawResponse.Errors[0].Message)
	}
	if rawResponse.Data.Repository == nil {
		return nil, fmt.Errorf("github: repository not found")
	}
	repo := rawResponse.Data.Repository

	var issues []GraphQLIssueNode
	for _, n := range repo.Issues.Nodes {
		createdAt, _ := time.Parse(time.RFC3339, n.CreatedAt)
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
			URL:           n.URL,
			State:         strings.ToUpper(n.State),
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
			AuthorLogin:   author,
			CommentsCount: n.Comments.TotalCount,
			Labels:        labelNames,
		})
	}

	closed := make([]ClosedIssue, 0, len(repo.Closed.Nodes))
	for _, n := range repo.Closed.Nodes {
		updatedAt, err := time.Parse(time.RFC3339, n.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("github: parse updatedAt of closed issue #%d: %w", n.Number, err)
		}
		closed = append(closed, ClosedIssue{Number: n.Number, UpdatedAt: updatedAt})
	}

	return &RepositoryIssuesPage{
		Issues:            issues,
		HasNextPage:       repo.Issues.PageInfo.HasNextPage,
		EndCursor:         repo.Issues.PageInfo.EndCursor,
		StargazerCount:    repo.StargazerCount,
		RateLimit:         rawResponse.Data.RateLimit.parse(),
		ClosedIssues:      closed,
		ClosedHasNextPage: repo.Closed.PageInfo.HasNextPage,
		ClosedEndCursor:   repo.Closed.PageInfo.EndCursor,
	}, nil
}
