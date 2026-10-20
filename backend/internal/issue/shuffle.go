package issue

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The shuffled feed: every eligible repository in the random order of
// repositories.shuffle_key, each showing its best open issue (easiest, then
// most recently updated) that passes the filters. A repository with ten
// thousand open issues gets the same one slot as one with three, which is the
// point: the goal is the whole backlog, not the biggest projects. ingestd
// redraws the order every ingest.catalog.shuffle_interval_seconds, so every
// repository spends time near the top, and between redraws the order is the
// same for everyone, so a page can be cached.
//
// A redraw moves repositories under a cursor someone is paging with; the
// pages after it then follow the new order from the same position.

// shuffleFeed returns one page of the shuffled feed after the cursor.
func (s *Service) shuffleFeed(ctx context.Context, f Filters, after *Cursor) ([]Issue, *Cursor, error) {
	limit := pageLimit(f.Limit)
	rare, err := s.isRare(ctx, f, nil)
	if err != nil {
		return nil, nil, err
	}
	var hits []shuffleHit
	var next *Cursor
	if rare {
		hits, next, err = s.shuffleFromMatches(ctx, f, after, limit)
	} else {
		hits, next, err = s.shuffleWalk(ctx, f, after, limit)
	}
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int, len(hits))
	for i, h := range hits {
		ids[i] = h.issue
	}
	byID := make(map[int]Issue, len(ids))
	err = s.db.QueryRows(ctx, baseIssueSelect+" WHERE i.id = ANY($1)", []any{ids}, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		byID[it.ID] = it
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("issue: load shuffled issues: %w", err)
	}
	page := make([]Issue, 0, len(hits))
	for _, h := range hits {
		if it, ok := byID[h.issue]; ok { // closed between the queries: skipped
			page = append(page, it)
		}
	}
	return page, next, nil
}

// shuffleHit is one repository's slot in the feed.
type shuffleHit struct {
	key, repo, issue int
}

// shuffleWalk reads repositories in shuffle order after the cursor, a batch at
// a time, and takes each one's best matching open issue, until the page is
// full or the repositories run out. Batches double, so a filter only a few
// repositories pass costs a handful of queries, not hundreds.
func (s *Service) shuffleWalk(ctx context.Context, f Filters, after *Cursor, limit int) ([]shuffleHit, *Cursor, error) {
	var hits []shuffleHit
	var key, repo int
	started := after != nil
	if started {
		key, repo = after.Key, after.ID
	}
	for batch := 2 * limit; ; batch *= 2 {
		var q searchQuery
		var position string
		if started {
			position = " AND (r.shuffle_key, r.id) > (" + q.arg(key) + ", " + q.arg(repo) + ")"
		}
		sql := `WITH repos AS (
			  SELECT r.id, r.shuffle_key FROM repositories r
			  WHERE r.is_eligible` + f.repoConds(q.arg) + position + `
			  ORDER BY r.shuffle_key, r.id LIMIT ` + q.arg(batch) + `
			)
			SELECT repos.shuffle_key, repos.id, i.id FROM repos LEFT JOIN LATERAL (
			  SELECT i.id FROM issues i
			  WHERE i.repository_id = repos.id AND i.state = 'OPEN'` + f.issueConds(q.arg) + `
			  ORDER BY ` + catalogOrder + ` LIMIT 1
			) i ON true
			ORDER BY repos.shuffle_key, repos.id`
		scanned := 0
		full := false
		err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, q.args...), func(rows pgx.Rows) error {
			var k, r int
			var issue *int
			if err := rows.Scan(&k, &r, &issue); err != nil {
				return err
			}
			scanned++
			if full {
				return nil // the rest of the batch belongs to the next page
			}
			key, repo, started = k, r, true
			if issue != nil {
				hits = append(hits, shuffleHit{k, r, *issue})
				full = len(hits) == limit
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("issue: walk shuffled repositories: %w", err)
		}
		if full {
			return hits, &Cursor{Sort: SortShuffle, Key: key, ID: repo}, nil
		}
		if scanned < batch {
			return hits, nil, nil // every repository has been walked
		}
	}
}

// shuffleFromMatches serves a rare filter: the few matching issues are found
// through the filters' own indexes, each repository's best kept, and ordered
// by the repositories' shuffle positions. "+ 0" keeps the planner from
// walking repositories in shuffle order instead, which for a rare filter is
// most of them.
func (s *Service) shuffleFromMatches(ctx context.Context, f Filters, after *Cursor, limit int) ([]shuffleHit, *Cursor, error) {
	var q searchQuery
	var position string
	if after != nil {
		position = " AND (r.shuffle_key, r.id) > (" + q.arg(after.Key) + ", " + q.arg(after.ID) + ")"
	}
	sql := `SELECT DISTINCT ON (r.shuffle_key + 0, r.id) r.shuffle_key, r.id, i.id
		FROM issues i JOIN repositories r ON r.id = i.repository_id
		WHERE i.state = 'OPEN' AND r.is_eligible` + f.conds(q.arg) + position + `
		ORDER BY r.shuffle_key + 0, r.id, ` + sortedOrder + `
		LIMIT ` + q.arg(limit)
	var hits []shuffleHit
	err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, q.args...), func(rows pgx.Rows) error {
		var h shuffleHit
		if err := rows.Scan(&h.key, &h.repo, &h.issue); err != nil {
			return err
		}
		hits = append(hits, h)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("issue: shuffle filtered issues: %w", err)
	}
	if len(hits) < limit {
		return hits, nil, nil
	}
	last := hits[len(hits)-1]
	return hits, &Cursor{Sort: SortShuffle, Key: last.key, ID: last.repo}, nil
}
