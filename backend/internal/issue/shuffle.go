package issue

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	// The walk is bounded (f.ShuffleWalkMaxRepos) and finishes from the
	// matches when it runs long, so it needs no rarity count up front — that
	// count could cost more than the page. Only a languages-only filter is
	// checked, from the cached per-language counts, since a rare language is
	// read far faster from its matches.
	rare := false
	if f.onlyLanguages() {
		var err error
		if rare, err = s.isRare(ctx, f, nil); err != nil {
			return nil, nil, err
		}
	}
	var hits []shuffleHit
	var next *Cursor
	var err error
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
//
// Two bounds keep a page from running long:
//
//   - A filter with plenty of matches can sit in few repositories ("kind/bug"
//     lives in a few hundred of 300k). Past f.ShuffleWalkMaxRepos the rest
//     of the page comes from the matching issues instead
//     (shuffleFromMatches), the cheaper read for such a filter.
//   - Some combinations match nothing anywhere (a label the classifier
//     always rates, with "unrated"), and no read of them is cheap. Past
//     f.ShuffleWalkBudget the page is returned as it is, short or empty,
//     with the cursor where the walk stopped: a short page with a cursor
//     means "more may follow", and only a nil cursor means the end.
func (s *Service) shuffleWalk(ctx context.Context, f Filters, after *Cursor, limit int) ([]shuffleHit, *Cursor, error) {
	var hits []shuffleHit
	var key, repo int
	started := after != nil
	if started {
		key, repo = after.Key, after.ID
	}
	var deadline time.Time
	if f.ShuffleWalkBudget > 0 {
		deadline = time.Now().Add(f.ShuffleWalkBudget)
	}
	stopHere := func() ([]shuffleHit, *Cursor, error) {
		if !started {
			return hits, nil, nil
		}
		return hits, &Cursor{Sort: SortShuffle, Key: key, ID: repo}, nil
	}

	walked := 0
	for batch := 2 * limit; ; batch = min(batch*2, maxShuffleBatch) {
		// At least one batch per page, so a page always moves the cursor on.
		if walked > 0 && !deadline.IsZero() && time.Now().After(deadline) {
			return stopHere()
		}
		if f.ShuffleWalkMaxRepos > 0 && walked >= f.ShuffleWalkMaxRepos {
			var from *Cursor
			if started {
				from = &Cursor{Sort: SortShuffle, Key: key, ID: repo}
			}
			mctx, cancel := ctx, context.CancelFunc(func() {})
			if !deadline.IsZero() {
				mctx, cancel = context.WithDeadline(ctx, deadline)
			}
			rest, next, err := s.shuffleFromMatches(mctx, f, from, limit-len(hits))
			cancel()
			if err != nil && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return stopHere()
			}
			if err != nil {
				return nil, nil, err
			}
			return append(hits, rest...), next, nil
		}
		if f.ShuffleWalkMaxRepos > 0 {
			batch = min(batch, max(f.ShuffleWalkMaxRepos-walked, 2*limit))
		}

		slots, err := s.shuffleBatch(ctx, f, started, key, repo, batch)
		if err != nil {
			return nil, nil, err
		}
		for _, sl := range slots {
			key, repo, started = sl.key, sl.repo, true
			if sl.issue != 0 {
				hits = append(hits, sl)
				if len(hits) == limit {
					// The rest of the batch belongs to the next page.
					return hits, &Cursor{Sort: SortShuffle, Key: key, ID: repo}, nil
				}
			}
		}
		walked += len(slots)
		if len(slots) < batch {
			return hits, nil, nil // every repository has been walked
		}
	}
}

// maxShuffleBatch caps one batch of the walk: past it a batch is slower to
// read than the page needs.
const maxShuffleBatch = 2000

// shuffleBatch reads the next batch of repositories in shuffle order and each
// one's best matching open issue (issue 0 when it has none).
//
// Each repository's issues are read in catalog order through
// issues_open_repo_browse_idx — unless the filter names labels. Then the
// label's matches are found for the whole batch at once: a label's GIN
// posting list is read once per batch rather than once per repository, which
// for a common label (bug: 780k issues) is the difference between 0.2s and
// 8ms times every repository walked.
func (s *Service) shuffleBatch(ctx context.Context, f Filters, started bool, key, repo, batch int) ([]shuffleHit, error) {
	var q searchQuery
	var position string
	if started {
		position = " AND (r.shuffle_key, r.id) > (" + q.arg(key) + ", " + q.arg(repo) + ")"
	}
	repos := `SELECT r.id, r.shuffle_key FROM repositories r
		  WHERE r.is_eligible` + f.repoConds(q.arg) + position + `
		  ORDER BY r.shuffle_key, r.id LIMIT ` + q.arg(batch)

	var slots []shuffleHit
	scan := func(rows pgx.Rows) error {
		var h shuffleHit
		var issue *int
		if err := rows.Scan(&h.key, &h.repo, &issue); err != nil {
			return err
		}
		if issue != nil {
			h.issue = *issue
		}
		slots = append(slots, h)
		return nil
	}

	if len(f.Labels) == 0 {
		sql := `WITH repos AS (` + repos + `)
			SELECT repos.shuffle_key, repos.id, i.id FROM repos LEFT JOIN LATERAL (
			  SELECT i.id FROM issues i
			  WHERE i.repository_id = repos.id AND i.state = 'OPEN'` + f.issueConds(q.arg) + `
			  ORDER BY ` + catalogOrder + ` LIMIT 1
			) i ON true
			ORDER BY repos.shuffle_key, repos.id`
		if err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, q.args...), scan); err != nil {
			return nil, fmt.Errorf("issue: walk shuffled repositories: %w", err)
		}
		return slots, nil
	}

	// Labels: the batch's repositories first, then their matches in one read.
	if err := s.db.QueryRows(ctx, `SELECT shuffle_key, id, NULL::int FROM (`+repos+`) b ORDER BY shuffle_key, id`,
		append([]any{pgx.QueryExecModeExec}, q.args...), scan); err != nil {
		return nil, fmt.Errorf("issue: walk shuffled repositories: %w", err)
	}
	if len(slots) == 0 {
		return slots, nil
	}
	ids := make([]int, len(slots))
	for i, sl := range slots {
		ids[i] = sl.repo
	}
	var m searchQuery
	sql := `SELECT DISTINCT ON (i.repository_id) i.repository_id, i.id FROM issues i
		WHERE i.repository_id = ANY(` + m.arg(ids) + `::int[]) AND i.state = 'OPEN'` + f.issueConds(m.arg) + `
		ORDER BY i.repository_id, ` + catalogOrder
	best := make(map[int]int, len(ids))
	err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, m.args...), func(rows pgx.Rows) error {
		var r, id int
		if err := rows.Scan(&r, &id); err != nil {
			return err
		}
		best[r] = id
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: match shuffled repositories' labels: %w", err)
	}
	for i := range slots {
		slots[i].issue = best[slots[i].repo]
	}
	return slots, nil
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
