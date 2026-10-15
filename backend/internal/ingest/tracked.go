package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/jackc/pgx/v5"
)

// TrackedIssue is an open issue a user has saved or is working on (a
// contribution that is STARTED or has its PR open). These are checked on
// GitHub on their own short cadence, so a user learns their issue closed
// within ingest.tracked_issue_check_seconds rather than whenever the
// repository's next freshness pass happens to run.
type TrackedIssue struct {
	ID         int
	Repository string
	Number     int
}

// DueTrackedIssues returns up to limit tracked issues that are still OPEN
// here and were last synced longer ago than staleAfter, stalest first.
func DueTrackedIssues(ctx context.Context, pool *db.Pool, staleAfter time.Duration, limit int) ([]TrackedIssue, error) {
	var out []TrackedIssue
	err := pool.QueryRows(ctx,
		`WITH tracked AS (
			SELECT issue_id FROM saved_issues
			UNION
			SELECT issue_id FROM contributions WHERE state IN ('STARTED', 'PR_OPEN')
		 )
		 SELECT i.id, r.full_name, i.number
		 FROM tracked t
		 JOIN issues i ON i.id = t.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE i.state = 'OPEN'
		   AND (i.last_synced_at IS NULL OR i.last_synced_at < now() - ($1 * interval '1 second'))
		 ORDER BY i.last_synced_at NULLS FIRST, i.id
		 LIMIT $2`,
		[]any{staleAfter.Seconds(), limit},
		func(rows pgx.Rows) error {
			var t TrackedIssue
			if err := rows.Scan(&t.ID, &t.Repository, &t.Number); err != nil {
				return err
			}
			out = append(out, t)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ingest: select due tracked issues: %w", err)
	}
	return out, nil
}

// ApplyTrackedStates writes the live states GitHub returned for issues (in
// the same order) and returns how many it marked CLOSED.
//
// An issue GitHub no longer has (deleted, transferred, or its repository
// gone or private) is marked CLOSED too: nobody can contribute to it. Its
// updated_at is left alone, since GitHub gave no time for it.
func ApplyTrackedStates(ctx context.Context, pool *db.Pool, issues []TrackedIssue, states []github.TrackedIssueState) (int, error) {
	if len(issues) != len(states) {
		return 0, fmt.Errorf("ingest: %d tracked issues but %d states", len(issues), len(states))
	}
	ids := make([]int, len(issues))
	newState := make([]string, len(issues))
	updatedAt := make([]*time.Time, len(issues))
	for i, s := range states {
		ids[i] = issues[i].ID
		if !s.Found {
			newState[i] = "CLOSED"
			continue
		}
		newState[i] = s.State
		t := s.UpdatedAt
		updatedAt[i] = &t
	}

	var closed int
	err := pool.QueryRow(ctx,
		`WITH applied AS (
			UPDATE issues i
			SET state = s.state,
			    updated_at = coalesce(s.updated_at, i.updated_at),
			    last_synced_at = now()
			FROM unnest($1::int[], $2::text[], $3::timestamptz[]) AS s(id, state, updated_at)
			WHERE i.id = s.id AND i.state = 'OPEN'
			RETURNING i.state
		 )
		 SELECT count(*) FILTER (WHERE state = 'CLOSED') FROM applied`,
		[]any{ids, newState, updatedAt},
		func(row pgx.Row) error { return row.Scan(&closed) },
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: apply tracked issue states: %w", err)
	}
	return closed, nil
}

// trackedRefs splits each issue's owner/name for FetchIssueStates. An
// unsplittable name is a data error the caller reports, not one to skip.
func trackedRefs(issues []TrackedIssue) ([]github.IssueRef, error) {
	refs := make([]github.IssueRef, len(issues))
	for i, t := range issues {
		owner, name, ok := strings.Cut(t.Repository, "/")
		if !ok {
			return nil, fmt.Errorf("ingest: tracked issue %d has malformed repository %q", t.ID, t.Repository)
		}
		refs[i] = github.IssueRef{Owner: owner, Name: name, Number: t.Number}
	}
	return refs, nil
}
