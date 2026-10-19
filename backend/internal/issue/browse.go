package issue

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// catalogOrder is the order the catalog is browsed in: easiest first,
// unclassified last (ASC puts NULL scores last), most recently updated first
// within a score, and id last so that no two issues ever tie.
// issues_open_browse_idx and issues_open_difficulty_browse_idx (migration
// 00035) are built in exactly this order.
const catalogOrder = "i.difficulty_score ASC, i.updated_at DESC, i.id DESC"

// Cursor is the position of the last issue on a browse page; the next page
// starts right after it.
type Cursor struct {
	// Bucket is 0 for a caller's preferred languages and 1 for every other
	// issue. Without preferred languages there is one bucket, 0.
	Bucket int
	// Score is nil for an UNCLASSIFIED issue.
	Score *int
	// Updated is updated_at in Unix microseconds, Postgres' own precision.
	Updated int64
	ID      int
}

// ErrBadCursor is returned for a cursor this package did not produce.
var ErrBadCursor = errors.New("issue: malformed cursor")

// Encode renders the cursor as an opaque, URL-safe token.
func (c Cursor) Encode() string {
	score := ""
	if c.Score != nil {
		score = strconv.Itoa(*c.Score)
	}
	raw := fmt.Sprintf("%d.%s.%d.%d", c.Bucket, score, c.Updated, c.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a token produced by Encode.
func DecodeCursor(token string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, ErrBadCursor
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 4 {
		return Cursor{}, ErrBadCursor
	}
	var c Cursor
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
	if c.ID, err = strconv.Atoi(parts[3]); err != nil || c.ID <= 0 {
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

// BrowseIssues returns one page of the catalog without a search term, in
// catalogOrder, starting after the cursor (at the top for nil), and the cursor
// for the page after it — nil once the catalog is exhausted. With preferred
// languages, issues in those languages come first and every other issue after
// them, as SearchIssues orders them.
//
// A page is read as a run of ordered index scans rather than one query. "After
// the cursor" mixes an ascending key with descending ones, which no single
// index range can express, and Postgres sorts a UNION of the ranges in full
// rather than merging them (3.3s on 2M open issues). So the page is filled
// range by range — the rest of the cursor's score, then higher scores, then
// unclassified issues, per bucket — each an index range read in order, until
// the page is full. A first page is one query; a typical page is one or two.
func (s *Service) BrowseIssues(ctx context.Context, f Filters, after *Cursor, prefLangs []string) ([]Issue, *Cursor, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Under a language filter every issue falls in the same bucket (all in
	// the preferred languages or none), so the preference changes nothing
	// and is dropped.
	langs := normaliseLanguages(prefLangs)
	if f.Language != "" {
		langs = nil
	}
	buckets := 1
	if len(langs) > 0 {
		buckets = 2
	}

	order, err := s.browseOrder(ctx, f, langs)
	if err != nil {
		return nil, nil, err
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
			segOrder := order
			if bucket == 1 {
				// Everything outside the preferred languages is never rare.
				segOrder = catalogOrder
			}
			rows, err := s.browseSegment(ctx, f, bucket, langs, seg, segOrder, limit-len(page))
			if err != nil {
				return nil, nil, err
			}
			for _, it := range rows {
				last = Cursor{Bucket: bucket, Score: it.DifficultyScore, Updated: it.updated.UnixMicro(), ID: it.ID}
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
// the matching issues by repository and sorts them instead of walking the
// catalog index in order. "+ 0" leaves every value, NULL included, as it is.
const sortedOrder = "i.difficulty_score + 0 ASC, i.updated_at DESC, i.id DESC"

// browseOrder chooses how a browse reads a language-constrained page. The
// planner cannot: language lives on repositories and score on issues, so it
// assumes a language's issues are spread evenly through the catalog index
// and, for a small LIMIT, bets on walking the index until it meets enough of
// them. For a common language that is right and takes a millisecond. For a
// rare one it walks most of the index — 4 seconds on 2M open issues for a
// language with a few. So the language's open issues are counted, up to the
// threshold (languageCounts), and a rare one is sorted.
func (s *Service) browseOrder(ctx context.Context, f Filters, langs []string) (string, error) {
	constrained := langs
	if f.Language != "" {
		constrained = []string{strings.ToLower(f.Language)}
	}
	if len(constrained) == 0 || f.RareLanguageBelow <= 0 {
		return catalogOrder, nil
	}

	counts, err := s.languages.counts(ctx, s.db, constrained, f.RareLanguageBelow, f.LanguageCountTTL)
	if err != nil {
		return "", err
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if total < f.RareLanguageBelow {
		return sortedOrder, nil
	}
	return catalogOrder, nil
}

func (s *Service) browseSegment(ctx context.Context, f Filters, bucket int, langs []string, seg segment, order string, limit int) ([]Issue, error) {
	var q browseQuery
	q.sb.WriteString(baseIssueSelect)
	q.sb.WriteString(" WHERE i.state = 'OPEN'")

	if f.Difficulty != "" {
		q.sb.WriteString(" AND i.difficulty = " + q.arg(strings.ToUpper(f.Difficulty)))
	}
	if f.Language != "" {
		q.sb.WriteString(" AND lower(r.language) = lower(" + q.arg(f.Language) + ")")
	}
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

	// Planned for these values, never from a cached generic plan, which
	// estimates for an average language rather than this one.
	args := append([]any{pgx.QueryExecModeExec}, q.args...)

	var out []Issue
	err := s.db.QueryRows(ctx, q.sb.String(), args, func(rows pgx.Rows) error {
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
