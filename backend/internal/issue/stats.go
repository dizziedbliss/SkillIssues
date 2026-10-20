package issue

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// CatalogStats are the live counts behind GET /stats. ingestd keeps them
// (ingest/stats.go) from deltas the issues triggers write (migration 00043);
// nothing here counts issues.
type CatalogStats struct {
	OpenIssues          int64      `json:"open_issues"`
	ClosedSince         int64      `json:"closed_since"`
	TrackingSince       *time.Time `json:"tracking_since"`
	ArrivedToday        int64      `json:"arrived_today"`
	ClosedToday         int64      `json:"closed_today"`
	IssuesIndexed       int64      `json:"issues_indexed"`
	RepositoriesIndexed int64      `json:"repositories_indexed"`
	Developers          int64      `json:"developers"`
	Activity            []Activity `json:"activity"`
	// UpdatedAt is when ingestd last folded changes in; nil before it first has.
	UpdatedAt *time.Time `json:"updated_at"`
}

// Activity is one recent commit that brought issues in or saw them close.
type Activity struct {
	At         time.Time `json:"at"`
	Repository string    `json:"repository"`
	Arrived    int       `json:"arrived"`
	Closed     int       `json:"closed"`
}

// Stats reads the live catalog counts and up to activity recent commits.
func (s *Service) Stats(ctx context.Context, activity int) (CatalogStats, error) {
	out := CatalogStats{Activity: []Activity{}}
	err := s.db.QueryRows(ctx, `SELECT key, value, since, updated_at FROM catalog_stats`, nil, func(rows pgx.Rows) error {
		var key string
		var value int64
		var since, updated time.Time
		if err := rows.Scan(&key, &value, &since, &updated); err != nil {
			return err
		}
		switch key {
		case "open_issues":
			out.OpenIssues = value
		case "closed_since":
			out.ClosedSince = value
			out.TrackingSince = &since
		case "issues_indexed":
			out.IssuesIndexed = value
		case "repositories_indexed":
			out.RepositoriesIndexed = value
		}
		if key != "closed_since" && (out.UpdatedAt == nil || updated.After(*out.UpdatedAt)) {
			out.UpdatedAt = &updated
		}
		return nil
	})
	if err != nil {
		return CatalogStats{}, fmt.Errorf("issue: read catalog stats: %w", err)
	}

	err = s.db.QueryRow(ctx, `
		SELECT coalesce((SELECT arrived FROM catalog_stats_daily WHERE day = (now() AT TIME ZONE 'UTC')::date), 0),
		       coalesce((SELECT closed  FROM catalog_stats_daily WHERE day = (now() AT TIME ZONE 'UTC')::date), 0),
		       (SELECT count(*) FROM users)`, nil,
		func(row pgx.Row) error { return row.Scan(&out.ArrivedToday, &out.ClosedToday, &out.Developers) })
	if err != nil {
		return CatalogStats{}, fmt.Errorf("issue: read today's stats: %w", err)
	}

	if activity <= 0 {
		return out, nil
	}
	err = s.db.QueryRows(ctx, `
		SELECT a.at, r.full_name, a.arrived, a.closed
		FROM catalog_activity a JOIN repositories r ON r.id = a.repository_id
		ORDER BY a.at DESC, a.id DESC LIMIT $1`, []any{activity}, func(rows pgx.Rows) error {
		var a Activity
		if err := rows.Scan(&a.At, &a.Repository, &a.Arrived, &a.Closed); err != nil {
			return err
		}
		out.Activity = append(out.Activity, a)
		return nil
	})
	if err != nil {
		return CatalogStats{}, fmt.Errorf("issue: read stats activity: %w", err)
	}
	return out, nil
}

// RepositoryLookup answers "is my repository indexed?".
type RepositoryLookup struct {
	// Found is whether the repository is stored at all. One that is not was
	// never discovered, or did not pass the eligibility rules at import.
	Found    bool   `json:"found"`
	FullName string `json:"full_name,omitempty"`
	URL      string `json:"url,omitempty"`
	Eligible bool   `json:"eligible"`
	// Reason is why a stored repository is not crawled (eligibility_reason),
	// "" when it is.
	Reason     string `json:"reason,omitempty"`
	Stars      int    `json:"stars"`
	OpenIssues int    `json:"open_issues"`
}

// ErrBadRepositoryName is returned for a name that is not owner/repo.
var ErrBadRepositoryName = errors.New("issue: repository name must be owner/repo")

var repositoryName = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9._-]{1,100}$`)

// LookupRepository finds a repository by owner/repo, case-insensitively, and
// counts its open issues. A repository with many is still one index range
// (issues_open_repo_browse_idx).
func (s *Service) LookupRepository(ctx context.Context, name string) (RepositoryLookup, error) {
	name = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(name), "https://github.com/"), "/")
	if !repositoryName.MatchString(name) {
		return RepositoryLookup{}, ErrBadRepositoryName
	}
	var out RepositoryLookup
	var reason *string
	err := s.db.QueryRow(ctx, `
		SELECT r.full_name, r.url, r.is_eligible, r.eligibility_reason, r.stars,
		       (SELECT count(*) FROM issues i WHERE i.repository_id = r.id AND i.state = 'OPEN')
		FROM repositories r WHERE lower(r.full_name) = lower($1)`, []any{name},
		func(row pgx.Row) error {
			return row.Scan(&out.FullName, &out.URL, &out.Eligible, &reason, &out.Stars, &out.OpenIssues)
		})
	if errors.Is(err, db.ErrNotFound) {
		return RepositoryLookup{}, nil
	}
	if err != nil {
		return RepositoryLookup{}, fmt.Errorf("issue: look up repository: %w", err)
	}
	out.Found = true
	if reason != nil && !out.Eligible {
		out.Reason = *reason
	}
	return out, nil
}
