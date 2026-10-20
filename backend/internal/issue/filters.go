package issue

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Sort is the order a browse lists the catalog in.
type Sort string

const (
	// SortShuffle walks repositories in a random order that ingestd redraws
	// periodically (repositories.shuffle_key) and shows each one's best open
	// issue: every repository gets the same one slot, however many issues it
	// has. The default when browsing by cursor.
	SortShuffle Sort = "shuffle"
	// SortEasiest is catalogOrder: difficulty, then most recently updated.
	// The default when paging by number, and the only sort a signed-in
	// caller's preferred languages reorder.
	SortEasiest Sort = "easiest"
	// SortNewest is the most recently opened first.
	SortNewest Sort = "newest"
)

// ParseSort accepts "", "shuffle", "easiest" or "newest".
func ParseSort(s string) (Sort, error) {
	switch Sort(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return "", nil
	case SortShuffle:
		return SortShuffle, nil
	case SortEasiest:
		return SortEasiest, nil
	case SortNewest:
		return SortNewest, nil
	}
	return "", fmt.Errorf("unknown sort %q: use shuffle, easiest or newest", s)
}

// Difficulties are the values the difficulty filter accepts.
var Difficulties = []string{"BEGINNER", "INTERMEDIATE", "ADVANCED", "UNCLASSIFIED"}

// ErrBadFilter is wrapped by every filter a caller got wrong.
var ErrBadFilter = errors.New("issue: invalid filter")

// ParseList splits a comma-separated filter value, trimming and dropping
// blanks and repeats. More than maxValues values is an error: every value is
// a condition the query has to evaluate.
func ParseList(name, raw string, maxValues int) ([]string, error) {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		v = strings.TrimSpace(v)
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if len(out) > maxValues {
		return nil, fmt.Errorf("%w: %s names %d values, more than %d", ErrBadFilter, name, len(out), maxValues)
	}
	return out, nil
}

// ParseDifficulties validates and upper-cases a difficulty list.
func ParseDifficulties(raw string, maxValues int) ([]string, error) {
	vals, err := ParseList("difficulty", raw, maxValues)
	if err != nil {
		return nil, err
	}
	for i, v := range vals {
		v = strings.ToUpper(v)
		if !slices.Contains(Difficulties, v) {
			return nil, fmt.Errorf("%w: unknown difficulty %q", ErrBadFilter, v)
		}
		vals[i] = v
	}
	return vals, nil
}

func lowerAll(vals []string) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = strings.ToLower(v)
	}
	return out
}

// hasFilters reports whether anything narrows the catalog.
func (f Filters) hasFilters() bool {
	return len(f.Difficulties) > 0 || len(f.Languages) > 0 || len(f.Labels) > 0 ||
		len(f.Topics) > 0 || len(f.Tech) > 0 || f.CommentsMin != nil || f.CommentsMax != nil
}

// onlyLanguages reports whether the languages filter is the only filter, the
// case languageCounts can answer from its cache.
func (f Filters) onlyLanguages() bool {
	g := f
	g.Languages = nil
	return !g.hasFilters()
}

// issueConds renders the filters on issues (alias i) as " AND …" conditions.
// Within one filter any value matches; across filters every one must.
func (f Filters) issueConds(arg func(any) string) string {
	var b strings.Builder
	if len(f.Difficulties) > 0 {
		b.WriteString(" AND i.difficulty = ANY(" + arg(f.Difficulties) + "::text[])")
	}
	if len(f.Labels) > 0 {
		// Case-insensitively, through issues_open_labels_lower_idx (00041):
		// labels are stored as GitHub has them, "Bug" and "bug" alike.
		b.WriteString(" AND catalog_lower_labels(i.labels) && " + arg(lowerAll(f.Labels)) + "::text[]")
	}
	if f.CommentsMin != nil {
		b.WriteString(" AND i.comments >= " + arg(*f.CommentsMin))
	}
	if f.CommentsMax != nil {
		b.WriteString(" AND i.comments <= " + arg(*f.CommentsMax))
	}
	return b.String()
}

// repoConds renders the filters on repositories (alias r).
func (f Filters) repoConds(arg func(any) string) string {
	var b strings.Builder
	if len(f.Languages) > 0 {
		b.WriteString(" AND lower(r.language) = ANY(" + arg(lowerAll(f.Languages)) + "::text[])")
	}
	if len(f.Topics) > 0 {
		b.WriteString(" AND r.topics && " + arg(lowerAll(f.Topics)) + "::text[]")
	}
	if len(f.Tech) > 0 {
		b.WriteString(" AND r.tech && " + arg(lowerAll(f.Tech)) + "::text[]")
	}
	return b.String()
}

// conds is every filter, for a query over issues i joined to repositories r.
func (f Filters) conds(arg func(any) string) string {
	return f.issueConds(arg) + f.repoConds(arg)
}

// isRare reports whether the filtered catalog (and, for the easiest sort, the
// caller's preferred languages) holds fewer than f.RareLanguageBelow open
// issues. The planner cannot tell: filters on repositories and labels say
// nothing about where the matches sit in an index on issues, so for a small
// LIMIT it bets on walking that index until it meets enough of them — right
// for a common filter, most of the index for a rare one. A rare result is
// read through the filters' own indexes and sorted instead.
func (s *Service) isRare(ctx context.Context, f Filters, prefLangs []string) (bool, error) {
	if f.RareLanguageBelow <= 0 {
		return false, nil
	}
	if !f.hasFilters() && len(prefLangs) == 0 {
		return false, nil
	}

	// Languages alone (a filter, or a caller's preferences) are counted per
	// language and cached: the commonest case, and the same for everyone.
	if f.onlyLanguages() {
		langs := lowerAll(f.Languages)
		if len(langs) == 0 {
			langs = prefLangs
		}
		counts, err := s.languages.counts(ctx, s.db, langs, f.RareLanguageBelow, f.LanguageCountTTL)
		if err != nil {
			return false, err
		}
		total := 0
		for _, n := range counts {
			total += n
		}
		return total < f.RareLanguageBelow, nil
	}

	// Any other combination is counted on the spot, up to the threshold. No
	// ORDER BY, so the planner reads it through the most selective filter's
	// index and stops at the threshold.
	var q searchQuery
	sql := `SELECT count(*) FROM (
		  SELECT 1 FROM issues i JOIN repositories r ON r.id = i.repository_id
		  WHERE i.state = 'OPEN'` + f.conds(q.arg) + `
		  LIMIT ` + q.arg(f.RareLanguageBelow) + `
		) t`
	var n int
	err := s.db.QueryRow(ctx, sql, append([]any{pgx.QueryExecModeExec}, q.args...),
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return false, fmt.Errorf("issue: count filtered issues: %w", err)
	}
	return n < f.RareLanguageBelow, nil
}
