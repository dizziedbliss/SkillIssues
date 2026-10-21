package issue

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// catalogOrder is the order the catalog is browsed in: easiest first,
// unclassified last (ASC puts NULL scores last), most recently updated first
// within a score, and id last so that no two issues ever tie.
// issues_open_browse_idx and issues_open_difficulty_browse_idx (migration
// 00035) are built in exactly this order.
const catalogOrder = "i.difficulty_score ASC, i.updated_at DESC, i.id DESC"

// newestOrder is the Newest sort, issues_open_newest_idx's order (00041).
const newestOrder = "i.created_at DESC, i.id DESC"

// ratedNewestOrder is newestOrder within one difficulty score:
// issues_open_rated_newest_idx's order (00063), which nothing else provides.
const ratedNewestOrder = "i.difficulty_score ASC, i.created_at DESC, i.id DESC"

// Cursor is the position of the last item on a browse page; the next page
// starts right after it. Which fields mean anything depends on Sort.
type Cursor struct {
	Sort Sort

	// Easiest. Bucket is 0 for a caller's preferred languages and 1 for every
	// other issue (one bucket, 0, without preferences); Score is nil for an
	// UNCLASSIFIED issue; Updated is updated_at in Unix microseconds,
	// Postgres' own precision; ID is the issue.
	Bucket int
	Score  *int

	// Easiest: updated_at. Newest: created_at. Unix microseconds.
	Updated int64
	// Easiest and newest: the issue. Shuffle: the repository.
	ID int

	// Shuffle: the repository's shuffle_key.
	Key int

	// For you: Key and ID are where the current chunk's walk began (ID 0 with
	// Key -1 is the top of the order); Offset is how far into that chunk's
	// order the page reached; Start is the shuffle_key the whole walk began
	// at, and Wrapped says it has gone round past the end once; Seed fixes
	// the draw, so every page of one walk agrees.
	Seed    int64
	Start   int
	Offset  int
	Wrapped bool
}

// ErrBadCursor is returned for a cursor this package did not produce, or one
// produced for another sort.
var ErrBadCursor = errors.New("issue: malformed cursor")

// Encode renders the cursor as an opaque, URL-safe token. An easiest cursor
// keeps the four-field form it had before there were other sorts, so tokens
// clients already hold stay valid.
func (c Cursor) Encode() string {
	var raw string
	switch c.Sort {
	case SortNewest:
		raw = fmt.Sprintf("n.%d.%d", c.Updated, c.ID)
	case SortShuffle:
		raw = fmt.Sprintf("s.%d.%d", c.Key, c.ID)
	case SortForYou:
		wrapped := 0
		if c.Wrapped {
			wrapped = 1
		}
		raw = fmt.Sprintf("y.%d.%d.%d.%d.%d.%d", c.Seed, c.Start, c.Key, c.ID, c.Offset, wrapped)
	default:
		score := ""
		if c.Score != nil {
			score = strconv.Itoa(*c.Score)
		}
		raw = fmt.Sprintf("%d.%s.%d.%d", c.Bucket, score, c.Updated, c.ID)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a token produced by Encode.
func DecodeCursor(token string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, ErrBadCursor
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) == 7 && parts[0] == "y" {
		return decodeForYou(parts[1:])
	}
	var c Cursor
	switch {
	case len(parts) == 3 && parts[0] == "n":
		c.Sort = SortNewest
		if c.Updated, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			return Cursor{}, ErrBadCursor
		}
	case len(parts) == 3 && parts[0] == "s":
		c.Sort = SortShuffle
		if c.Key, err = strconv.Atoi(parts[1]); err != nil {
			return Cursor{}, ErrBadCursor
		}
	case len(parts) == 4:
		c.Sort = SortEasiest
		if c.Bucket, err = strconv.Atoi(parts[0]); err != nil || c.Bucket < 0 || c.Bucket > 1 {
			return Cursor{}, ErrBadCursor
		}
		if parts[1] != "" {
			score, err := strconv.Atoi(parts[1])
			if err != nil {
				return Cursor{}, ErrBadCursor
			}
			c.Score = &score
		}
		if c.Updated, err = strconv.ParseInt(parts[2], 10, 64); err != nil {
			return Cursor{}, ErrBadCursor
		}
	default:
		return Cursor{}, ErrBadCursor
	}
	if c.ID, err = strconv.Atoi(parts[len(parts)-1]); err != nil || c.ID <= 0 {
		return Cursor{}, ErrBadCursor
	}
	return c, nil
}

// browseQuery accumulates one segment query's SQL and its arguments.
type browseQuery struct {
	sb   strings.Builder
	args []any
}

// arg binds v and returns its placeholder.
func (q *browseQuery) arg(v any) string {
	q.args = append(q.args, v)
	return "$" + strconv.Itoa(len(q.args))
}

// pageLimit clamps a requested page size.
func pageLimit(n int) int {
	if n <= 0 {
		return 20
	}
	return min(n, 100)
}

// BrowseIssues returns one page of the catalog without a search term, in
// f.Sort (easiest when empty), starting after the cursor (at the top for nil),
// and the cursor for the page after it — nil once the catalog is exhausted.
// Preferred languages reorder the easiest sort only: shuffle and newest are
// the same for everyone.
func (s *Service) BrowseIssues(ctx context.Context, f Filters, after *Cursor, prefLangs []string) ([]Issue, *Cursor, error) {
	// Hidden issues leave after the cursor is taken from the page as read, so
	// a page that loses every issue still carries on from where it stopped.
	page, next, err := s.browseIssues(ctx, f, after, prefLangs)
	return s.visible(page, f.Unassigned), next, err
}

func (s *Service) browseIssues(ctx context.Context, f Filters, after *Cursor, prefLangs []string) ([]Issue, *Cursor, error) {
	sort := f.Sort
	if sort == "" {
		sort = SortEasiest
	}
	if after != nil && after.Sort != sort {
		return nil, nil, fmt.Errorf("%w: the cursor is for the %s sort, not %s", ErrBadCursor, after.Sort, sort)
	}
	switch sort {
	case SortForYou:
		return nil, nil, fmt.Errorf("issue: the for-you sort is read through ForYouPage")
	case SortShuffle:
		return s.shuffleFeed(ctx, f, after)
	case SortNewest:
		return s.newestPage(ctx, f, after)
	default:
		return s.easiestPage(ctx, f, after, prefLangs)
	}
}

// easiestPage reads the catalog in catalogOrder.
//
// A page is read as a run of ordered index scans rather than one query. "After
// the cursor" mixes an ascending key with descending ones, which no single
// index range can express, and Postgres sorts a UNION of the ranges in full
// rather than merging them (3.3s on 2M open issues). So the page is filled
// range by range — the rest of the cursor's score, then higher scores, then
// unclassified issues, per bucket — each an index range read in order, until
// the page is full. A first page is one query; a typical page is one or two.
func (s *Service) easiestPage(ctx context.Context, f Filters, after *Cursor, prefLangs []string) ([]Issue, *Cursor, error) {
	limit := pageLimit(f.Limit)

	// Under a language filter every issue falls in the same bucket (all in
	// the preferred languages or none), so the preference changes nothing
	// and is dropped.
	langs := normaliseLanguages(prefLangs)
	if len(f.Languages) > 0 {
		langs = nil
	}
	buckets := 1
	if len(langs) > 0 {
		buckets = 2
	}

	orders := make([]string, buckets)
	for b := range orders {
		var bucketLangs []string
		if b == 0 {
			bucketLangs = langs
		}
		// Everything outside the preferred languages is rare only if the
		// filters make it so.
		rare, err := s.isRare(ctx, f, bucketLangs)
		if err != nil {
			return nil, nil, err
		}
		orders[b] = catalogOrder
		if rare {
			orders[b] = sortedOrder
		}
	}

	var page []Issue
	var last Cursor
	for bucket := 0; bucket < buckets && len(page) < limit; bucket++ {
		var from *Cursor
		if after != nil {
			// Without preferred languages a cursor's bucket means nothing:
			// there is only one, so its position applies to it.
			b := min(after.Bucket, buckets-1)
			if bucket < b {
				continue
			}
			if bucket == b {
				from = after
			}
		}
		for _, seg := range segmentsAfter(from) {
			rows, err := s.browseSegment(ctx, f, bucket, langs, seg, orders[bucket], limit-len(page))
			if err != nil {
				return nil, nil, err
			}
			for _, it := range rows {
				last = Cursor{Sort: SortEasiest, Bucket: bucket, Score: it.DifficultyScore, Updated: it.updated.UnixMicro(), ID: it.ID}
			}
			page = append(page, rows...)
			if len(page) == limit {
				break
			}
		}
	}

	if len(page) < limit {
		return page, nil, nil
	}
	return page, &last, nil
}

// segment is one ordered index range of the catalog after a cursor: a
// condition on difficulty_score, and whether it continues within the cursor's
// own score from the cursor's (updated_at, id).
type segment struct {
	score  string // "=", ">", "null", or "" for the whole bucket
	within bool
	from   *Cursor
}

// segmentsAfter lists, in catalog order, the ranges that together hold every
// issue after the cursor.
func segmentsAfter(c *Cursor) []segment {
	switch {
	case c == nil:
		return []segment{{}}
	case c.Score == nil:
		// Unclassified issues sort last, so only the rest of them remain.
		return []segment{{score: "null", within: true, from: c}}
	default:
		return []segment{
			{score: "=", within: true, from: c},
			{score: ">", from: c},
			{score: "null"},
		}
	}
}

// sortedOrder is catalogOrder in a form no index provides, so Postgres reads
// the matching issues through the filters' own indexes and sorts them instead
// of walking the catalog index in order. "+ 0" leaves every value, NULL
// included, as it is.
const sortedOrder = "i.difficulty_score + 0 ASC, i.updated_at DESC, i.id DESC"

// sortedNewestOrder is newestOrder in a form no index provides, for a rare
// filter.
const sortedNewestOrder = "i.created_at + interval '0 seconds' DESC, i.id DESC"

func (s *Service) browseSegment(ctx context.Context, f Filters, bucket int, langs []string, seg segment, order string, limit int) ([]Issue, error) {
	var q browseQuery
	q.sb.WriteString(baseIssueSelect)
	q.sb.WriteString(" WHERE i.state = 'OPEN'")
	q.sb.WriteString(f.conds(q.arg))
	if len(langs) > 0 {
		// Bucket 1 is everything bucket 0 is not, a repository without a
		// language included — the same split as SearchIssues' CASE.
		cond := "lower(r.language) = ANY(" + q.arg(langs) + ")"
		if bucket == 1 {
			cond = "NOT COALESCE(" + cond + ", false)"
		}
		q.sb.WriteString(" AND " + cond)
	}

	switch seg.score {
	case "=":
		q.sb.WriteString(" AND i.difficulty_score = " + q.arg(*seg.from.Score))
	case ">":
		q.sb.WriteString(" AND i.difficulty_score > " + q.arg(*seg.from.Score))
	case "null":
		q.sb.WriteString(" AND i.difficulty_score IS NULL")
	}
	if seg.within {
		// Both keys descend, so this is one index range of the score.
		updated := time.UnixMicro(seg.from.Updated)
		q.sb.WriteString(" AND (i.updated_at, i.id) < (" + q.arg(updated) + ", " + q.arg(seg.from.ID) + ")")
	}

	q.sb.WriteString(" ORDER BY " + order + " LIMIT " + q.arg(limit))
	return s.queryIssues(ctx, q.sb.String(), q.args)
}

// newestPage reads open issues by created_at, newest first, as one index
// range after the cursor. An issue whose opening time is unknown is not in it.
//
// A difficulty filter is read one difficulty at a time and merged: walking
// every open issue by created_at to find the few rated ones among them took
// 10s for BEGINNER alone (recent issues are mostly unrated), while each
// difficulty's own range is one index read (issues_open_rated_newest_idx,
// 00063; unrated ones are most of the newest index anyway).
func (s *Service) newestPage(ctx context.Context, f Filters, after *Cursor) ([]Issue, *Cursor, error) {
	limit := pageLimit(f.Limit)
	rare, err := s.isRare(ctx, f, nil)
	if err != nil {
		return nil, nil, err
	}
	var page []Issue
	if !rare && len(f.Difficulties) > 0 {
		page, err = s.newestByDifficulty(ctx, f, after, limit)
	} else {
		order := newestOrder
		if rare {
			order = sortedNewestOrder
		}
		sql, args := newestQuery(f, after, order, limit)
		page, err = s.queryIssues(ctx, sql, args)
	}
	if err != nil {
		return nil, nil, err
	}
	if len(page) < limit {
		return page, nil, nil
	}
	lastIssue := page[len(page)-1]
	return page, &Cursor{Sort: SortNewest, Updated: lastIssue.created.UnixMicro(), ID: lastIssue.ID}, nil
}

// newestQuery is one Newest read: the filters, after the cursor, in order.
func newestQuery(f Filters, after *Cursor, order string, limit int, extra ...string) (string, []any) {
	var q browseQuery
	q.sb.WriteString(baseIssueSelect)
	q.sb.WriteString(" WHERE i.state = 'OPEN' AND i.created_at IS NOT NULL")
	for _, cond := range extra {
		q.sb.WriteString(cond)
	}
	q.sb.WriteString(f.conds(q.arg))
	if after != nil {
		q.sb.WriteString(" AND (i.created_at, i.id) < (" + q.arg(time.UnixMicro(after.Updated)) + ", " + q.arg(after.ID) + ")")
	}
	q.sb.WriteString(" ORDER BY " + order + " LIMIT " + q.arg(limit))
	return q.sb.String(), q.args
}

// newestByDifficulty reads each wanted difficulty's newest issues on its own,
// concurrently, and keeps the newest limit of them all.
func (s *Service) newestByDifficulty(ctx context.Context, f Filters, after *Cursor, limit int) ([]Issue, error) {
	parts := make([][]Issue, len(f.Difficulties))
	g, gctx := errgroup.WithContext(ctx)
	for i, d := range f.Difficulties {
		one := f
		one.Difficulties = []string{d}
		// A plain equality on the score, so the planner sees the rated index's range.
		cond, order := " AND i.difficulty_score IS NULL", newestOrder
		if score, ok := difficultyScores[d]; ok {
			cond = fmt.Sprintf(" AND i.difficulty_score = %d", score)
			// The same order (the score is one value here), but only the rated
			// index provides it: ordered by created_at alone, the planner bets
			// on finding LIMIT rated issues early in issues_open_newest_idx and
			// walks millions of unrated ones instead (11s in production).
			order = ratedNewestOrder
		}
		g.Go(func() error {
			sql, args := newestQuery(one, after, order, limit, cond)
			got, err := s.queryIssues(gctx, sql, args)
			parts[i] = got
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	var all []Issue
	for _, p := range parts {
		all = append(all, p...)
	}
	sort.Slice(all, func(a, b int) bool {
		if !all[a].created.Equal(all[b].created) {
			return all[a].created.After(all[b].created)
		}
		return all[a].ID > all[b].ID
	})
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// queryIssues runs a catalog query selecting baseIssueSelect's columns.
// Planned for these values, never from a cached generic plan, which estimates
// for average filters rather than these.
func (s *Service) queryIssues(ctx context.Context, sql string, args []any) ([]Issue, error) {
	var out []Issue
	err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, args...), func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		out = append(out, it)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: browse issues: %w", err)
	}
	return out, nil
}

// normaliseLanguages lower-cases and trims preferred languages, dropping
// blanks, the way SearchIssues compares them.
func normaliseLanguages(langs []string) []string {
	var out []string
	for _, l := range langs {
		if tl := strings.TrimSpace(l); tl != "" {
			out = append(out, strings.ToLower(tl))
		}
	}
	return out
}
