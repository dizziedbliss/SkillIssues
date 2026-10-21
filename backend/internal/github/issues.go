package github

import (
	"context"
	"fmt"
	"net/url"
	"slices"
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
	// Assignees is how many people are assigned; nil when the source didn't say.
	Assignees *int `json:"assignees,omitempty"`
}

// RepositoryIssuesPage is one page of a repository's open issues and, when
// asked for, one page of the issues closed since the pass's watermark.
type RepositoryIssuesPage struct {
	Issues      []GraphQLIssueNode `json:"issues"`
	HasNextPage bool               `json:"has_next_page"`
	EndCursor   string             `json:"end_cursor"`
	RateLimit   GraphQLRateLimit   `json:"rate_limit"`

	// Stats is the repository's live metadata, present only when
	// IssuesQuery.FetchStats asked for it.
	Stats *RepositoryStats `json:"stats,omitempty"`

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

// RepositoryStats is a repository's metadata as GitHub reports it now —
// the live replacement for the dataset snapshot's numbers.
type RepositoryStats struct {
	// DatabaseID is GitHub's numeric repository id: stable across renames
	// and transfers, unlike the name the repository was fetched by.
	DatabaseID     int64
	Stars          int
	Forks          int
	Watchers       int
	OpenIssueCount int
	IsArchived     bool
	PushedAt       *time.Time
	// ContributingURL is the repository's contributing guide on GitHub, or
	// nil when it has none (or GitHub gave something other than a github.com
	// page, which is never stored).
	ContributingURL *string
	// Health is how the repository's latest merges went, when asked for.
	Health *RepositoryHealth
}

// RepositoryHealth summarises a repository's latest merged pull requests.
type RepositoryHealth struct {
	// Sampled is how many merged pull requests were read (up to the sample).
	Sampled int
	// Outside is how many were by people outside the project: not its
	// owner, a member of its organisation or a collaborator.
	Outside int
	// MedianMergeHours is the median time from opening to merging; nil with
	// nothing sampled.
	MedianMergeHours *int
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

	// FetchStats also requests the repository's live metadata. Scalars and
	// one totalCount, so it does not raise the request's point cost; asked
	// once per pass, on its first page.
	FetchStats bool
	// HealthSample, with FetchStats, also reads the latest this-many merged
	// pull requests for RepositoryStats.Health; 0 skips it.
	HealthSample int
}

const repositoryIssuesQuery = `query RepositoryIssues($owner: String!, $name: String!, $after: String, $since: DateTime, $first: Int!, $withOpen: Boolean!, $withClosed: Boolean!, $closedAfter: String, $closedFirst: Int!, $withStats: Boolean!, $withHealth: Boolean!, $healthLast: Int!) {
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
                assignees { totalCount }
                labels(first: 20) { nodes { name } }
            }
            pageInfo { hasNextPage endCursor }
        }
        closed: issues(first: $closedFirst, after: $closedAfter, states: CLOSED, orderBy: {field: UPDATED_AT, direction: DESC}, filterBy: {since: $since}) @include(if: $withClosed) {
            nodes { number updatedAt }
            pageInfo { hasNextPage endCursor }
        }
        databaseId @include(if: $withStats)
        stargazerCount @include(if: $withStats)
        forkCount @include(if: $withStats)
        watchers @include(if: $withStats) { totalCount }
        openIssues: issues(states: OPEN) @include(if: $withStats) { totalCount }
        isArchived @include(if: $withStats)
        pushedAt @include(if: $withStats)
        contributingGuidelines @include(if: $withStats) { url }
        mergedPulls: pullRequests(states: MERGED, last: $healthLast) @include(if: $withHealth) {
            nodes { authorAssociation createdAt mergedAt }
        }
        __typename
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
		"withStats":   q.FetchStats,
		"withHealth":  q.FetchStats && q.HealthSample > 0,
		"healthLast":  clampPageSize(max(q.HealthSample, 1)),
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
						Assignees struct {
							TotalCount int `json:"totalCount"`
						} `json:"assignees"`
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
				DatabaseID     int64 `json:"databaseId"`
				StargazerCount *int  `json:"stargazerCount"`
				ForkCount      int   `json:"forkCount"`
				Watchers       struct {
					TotalCount int `json:"totalCount"`
				} `json:"watchers"`
				OpenIssues struct {
					TotalCount int `json:"totalCount"`
				} `json:"openIssues"`
				IsArchived   bool    `json:"isArchived"`
				PushedAt     *string `json:"pushedAt"`
				Contributing *struct {
					URL string `json:"url"`
				} `json:"contributingGuidelines"`
				MergedPulls *struct {
					Nodes []mergedPullNode `json:"nodes"`
				} `json:"mergedPulls"`
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
			Assignees:     &n.Assignees.TotalCount,
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

	var stats *RepositoryStats
	if q.FetchStats {
		if repo.StargazerCount == nil {
			return nil, fmt.Errorf("github: repository stats requested but not returned")
		}
		stats = &RepositoryStats{
			DatabaseID:     repo.DatabaseID,
			Stars:          *repo.StargazerCount,
			Forks:          repo.ForkCount,
			Watchers:       repo.Watchers.TotalCount,
			OpenIssueCount: repo.OpenIssues.TotalCount,
			IsArchived:     repo.IsArchived,
		}
		if repo.Contributing != nil && isGitHubPage(repo.Contributing.URL) {
			u := repo.Contributing.URL
			stats.ContributingURL = &u
		}
		if q.HealthSample > 0 && repo.MergedPulls != nil {
			h, err := healthOf(repo.MergedPulls.Nodes)
			if err != nil {
				return nil, err
			}
			stats.Health = &h
		}
		if repo.PushedAt != nil {
			// A repository with no pushes has a null pushedAt; that stays nil.
			t, err := time.Parse(time.RFC3339, *repo.PushedAt)
			if err != nil {
				return nil, fmt.Errorf("github: parse pushedAt: %w", err)
			}
			stats.PushedAt = &t
		}
	}

	return &RepositoryIssuesPage{
		Issues:            issues,
		HasNextPage:       repo.Issues.PageInfo.HasNextPage,
		EndCursor:         repo.Issues.PageInfo.EndCursor,
		Stats:             stats,
		RateLimit:         rawResponse.Data.RateLimit.parse(),
		ClosedIssues:      closed,
		ClosedHasNextPage: repo.Closed.PageInfo.HasNextPage,
		ClosedEndCursor:   repo.Closed.PageInfo.EndCursor,
	}, nil
}

// isGitHubPage reports whether u is an https page on github.com: the only
// kind of link the apps are given to open.
func isGitHubPage(u string) bool {
	p, err := url.Parse(u)
	return err == nil && p.Scheme == "https" && p.Host == "github.com" && p.User == nil
}

type mergedPullNode struct {
	AuthorAssociation string `json:"authorAssociation"`
	CreatedAt         string `json:"createdAt"`
	MergedAt          string `json:"mergedAt"`
}

// healthOf summarises merged pull requests: how many came from outside the
// project, and the median hours each took to merge.
func healthOf(nodes []mergedPullNode) (RepositoryHealth, error) {
	h := RepositoryHealth{Sampled: len(nodes)}
	hours := make([]int, 0, len(nodes))
	for _, n := range nodes {
		switch n.AuthorAssociation {
		case "OWNER", "MEMBER", "COLLABORATOR":
		default:
			h.Outside++
		}
		created, err := time.Parse(time.RFC3339, n.CreatedAt)
		if err != nil {
			return h, fmt.Errorf("github: parse merged pull createdAt: %w", err)
		}
		merged, err := time.Parse(time.RFC3339, n.MergedAt)
		if err != nil {
			return h, fmt.Errorf("github: parse merged pull mergedAt: %w", err)
		}
		hours = append(hours, int(merged.Sub(created).Hours()))
	}
	if len(hours) > 0 {
		slices.Sort(hours)
		m := hours[len(hours)/2]
		if len(hours)%2 == 0 {
			m = (hours[len(hours)/2-1] + hours[len(hours)/2]) / 2
		}
		h.MedianMergeHours = &m
	}
	return h, nil
}
