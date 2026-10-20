package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// Search is balanced across four match categories, in this precedence:
//
//	Repository  the repository's name contains the term
//	Title       the issue's title or labels match it (full-text, search_vector)
//	Language    the repository's language is the term
//	Author      the issue's author's login contains it
//
// An issue appears once, in the first category it matches, and each category
// contributes at most api.search.per_category_cap issues. Results interleave
// the categories rank by rank (every category's best, then every category's
// second, …), so no one category floods the page.
//
// Each category is its own query on its own index, run concurrently, rather
// than one query ORing every kind of match across a join: no index serves
// that, and on the 7.5M-issue catalog it ran past the query timeout. Title and
// Author matches can number in the hundreds of thousands for a common word, so
// they are ordered by stars within a sample of api.search.sample_size matches
// read straight off the index, which bounds the work whatever the word.

// matchCategories in precedence order, as the API names them.
var matchCategories = []string{"Repository", "Title", "Language", "Author"}

// minSubstringRunes is the shortest term the substring categories (Repository,
// Author) search for: their trigram indexes cannot serve a shorter pattern,
// which would scan every row. Title and Language still match it.
const minSubstringRunes = 3

// searchQuery accumulates one category query and its arguments.
type searchQuery struct {
	args []any
}

func (q *searchQuery) arg(v any) string {
	q.args = append(q.args, v)
	return "$" + strconv.Itoa(len(q.args))
}

// issueFilter and repoFilter render the request's difficulty and language
// filters, on issues alias i and repositories alias r.
func (q *searchQuery) issueFilter(f Filters) string {
	if f.Difficulty == "" {
		return ""
	}
	return " AND i.difficulty = " + q.arg(strings.ToUpper(f.Difficulty))
}

func (q *searchQuery) repoFilter(f Filters) string {
	if f.Language == "" {
		return ""
	}
	return " AND lower(r.language) = lower(" + q.arg(f.Language) + ")"
}

// searchPerRepo is api.search.per_repo_cap, at least 1.
func (f Filters) searchPerRepo() int {
	return max(1, f.SearchPerRepo)
}

// escapeLike makes a user's term literal inside a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// searchCategories runs the category queries and returns one page of the
// interleaved result.
func (s *Service) searchCategories(ctx context.Context, f Filters, limit, offset int) ([]Issue, error) {
	perCategory := s.categoryCap()
	// Fetch more than the cap so that, once issues a higher category already
	// took are removed, a category can still fill its share.
	fetch := perCategory * len(matchCategories)
	sample := f.SearchSample
	if sample < fetch {
		sample = fetch
	}
	term := strings.TrimSpace(f.Search)
	substr := utf8.RuneCountInString(term) >= minSubstringRunes

	queries := make([]func(context.Context) ([]int, error), len(matchCategories))
	queries[0] = func(ctx context.Context) ([]int, error) {
		if !substr {
			return nil, nil
		}
		return s.repositoryMatches(ctx, f, term, fetch)
	}
	queries[1] = func(ctx context.Context) ([]int, error) { return s.titleMatches(ctx, f, term, fetch, sample) }
	queries[2] = func(ctx context.Context) ([]int, error) {
		if f.Language != "" && !strings.EqualFold(f.Language, term) {
			return nil, nil // filtered to another language: nothing can match
		}
		return s.languageMatches(ctx, f, term, fetch)
	}
	queries[3] = func(ctx context.Context) ([]int, error) {
		if !substr {
			return nil, nil
		}
		return s.authorMatches(ctx, f, term, fetch, sample)
	}

	found := make([][]int, len(matchCategories))
	g, gctx := errgroup.WithContext(ctx)
	for i, q := range queries {
		g.Go(func() error {
			ids, err := q(gctx)
			if err != nil {
				return fmt.Errorf("issue: search %s matches: %w", matchCategories[i], err)
			}
			found[i] = ids
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Each issue in its first category only, each category capped.
	seen := make(map[int]bool)
	ranked := make([][]int, len(matchCategories))
	for c, ids := range found {
		for _, id := range ids {
			if len(ranked[c]) == perCategory {
				break
			}
			if !seen[id] {
				seen[id] = true
				ranked[c] = append(ranked[c], id)
			}
		}
	}

	type hit struct {
		id       int
		category string
	}
	var order []hit
	for rank := 0; rank < perCategory; rank++ {
		for c := range matchCategories {
			if rank < len(ranked[c]) {
				order = append(order, hit{ranked[c][rank], matchCategories[c]})
			}
		}
	}
	if offset >= len(order) {
		return []Issue{}, nil
	}
	order = order[offset:min(offset+limit, len(order))]

	ids := make([]int, len(order))
	for i, h := range order {
		ids[i] = h.id
	}
	byID := make(map[int]Issue, len(ids))
	err := s.db.QueryRows(ctx, baseIssueSelect+" WHERE i.id = ANY($1)", []any{ids}, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		byID[it.ID] = it
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: load search results: %w", err)
	}
	out := make([]Issue, 0, len(order))
	for _, h := range order {
		if it, ok := byID[h.id]; ok { // gone between the queries: skipped
			it.MatchCategory = h.category
			out = append(out, it)
		}
	}
	return out, nil
}

// matchIDs runs one category query returning issue ids in rank order. Each is
// planned for its own values (QueryExecModeExec): how selective a pattern is
// decides the plan.
func (s *Service) matchIDs(ctx context.Context, sql string, args []any) ([]int, error) {
	var ids []int
	err := s.db.QueryRows(ctx, sql, append([]any{pgx.QueryExecModeExec}, args...), func(rows pgx.Rows) error {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	})
	return ids, err
}

// issuesOfRepos ranks the open issues of the repositories the CTE `repos`
// (id, stars, rk) yields: repositories in (rk, stars desc) order, and within
// one, easiest then most recently updated — the catalog's own order. A
// repository contributes at most perRepo issues, so one large repository
// cannot fill a category on its own (every repository gets equal priority),
// and the query reads a few issues from each repository rather than
// everything from the biggest.
func issuesOfRepos(q *searchQuery, f Filters, fetch, perRepo int) string {
	return `
		SELECT i.id FROM repos JOIN LATERAL (
		  SELECT i.id, i.difficulty_score, i.updated_at FROM issues i
		  WHERE i.repository_id = repos.id AND i.state = 'OPEN'` + q.issueFilter(f) + `
		  ORDER BY i.difficulty_score ASC NULLS LAST, i.updated_at DESC LIMIT ` + q.arg(perRepo) + `
		) i ON true
		ORDER BY repos.rk, repos.stars DESC, i.difficulty_score ASC NULLS LAST, i.updated_at DESC
		LIMIT ` + q.arg(fetch)
}

// reposFor is how many repositories a repository-led category reads: enough
// to fill fetch at perRepo each, with room for repositories that have no
// matching open issue.
func reposFor(fetch, perRepo int) int {
	return 2 * (fetch + perRepo - 1) / perRepo
}

// repositoryMatches: repositories whose name contains the term, closest names
// first (exactly the term, then the term as a word in it, then starting with
// it), then by stars.
func (s *Service) repositoryMatches(ctx context.Context, f Filters, term string, fetch int) ([]int, error) {
	var q searchQuery
	lower := strings.ToLower(escapeLike(term))
	contains := q.arg("%" + escapeLike(term) + "%")
	exact := q.arg(strings.ToLower(term))
	word := q.arg(lower + "-%")
	under := q.arg(lower + `\_%`)
	suffix := q.arg("%-" + lower)
	prefix := q.arg(lower + "%")
	repoFilter := q.repoFilter(f)
	repoLimit := q.arg(reposFor(fetch, f.searchPerRepo()))
	sql := `WITH repos AS (
		  SELECT r.id, r.stars,
		         CASE WHEN lower(split_part(r.full_name, '/', 2)) = ` + exact + ` THEN 1
		              WHEN lower(split_part(r.full_name, '/', 2)) LIKE ` + word + `
		                OR lower(split_part(r.full_name, '/', 2)) LIKE ` + under + `
		                OR lower(split_part(r.full_name, '/', 2)) LIKE ` + suffix + ` THEN 2
		              WHEN lower(split_part(r.full_name, '/', 2)) LIKE ` + prefix + ` THEN 3
		              ELSE 4 END AS rk
		  FROM repositories r
		  WHERE r.full_name ILIKE ` + contains + repoFilter + `
		  ORDER BY rk, r.stars DESC LIMIT ` + repoLimit + `
		)` + issuesOfRepos(&q, f, fetch, f.searchPerRepo())
	return s.matchIDs(ctx, sql, q.args)
}

// languageMatches: issues of repositories written in the term, most-starred
// repositories first.
func (s *Service) languageMatches(ctx context.Context, f Filters, term string, fetch int) ([]int, error) {
	var q searchQuery
	lang := q.arg(term)
	repoLimit := q.arg(reposFor(fetch, f.searchPerRepo()))
	sql := `WITH repos AS (
		  SELECT r.id, r.stars, 1 AS rk FROM repositories r
		  WHERE lower(r.language) = lower(` + lang + `)
		  ORDER BY r.stars DESC LIMIT ` + repoLimit + `
		)` + issuesOfRepos(&q, f, fetch, f.searchPerRepo())
	return s.matchIDs(ctx, sql, q.args)
}

// sampledByStars orders a sample of matching open issues by their
// repository's stars. where is the match condition on issues alias i.
func (s *Service) sampledByStars(ctx context.Context, q *searchQuery, f Filters, where string, fetch, sample int) ([]int, error) {
	sql := `SELECT m.id FROM (
		  SELECT i.id, i.repository_id, i.difficulty_score, i.updated_at FROM issues i
		  WHERE i.state = 'OPEN' AND ` + where + q.issueFilter(f) + `
		  LIMIT ` + q.arg(sample) + `
		) m JOIN repositories r ON r.id = m.repository_id` + q.repoFilter(f) + `
		ORDER BY r.stars DESC, m.difficulty_score ASC NULLS LAST, m.updated_at DESC
		LIMIT ` + q.arg(fetch)
	return s.matchIDs(ctx, sql, q.args)
}

// titleMatches: full-text matches on title and labels (search_vector).
func (s *Service) titleMatches(ctx context.Context, f Filters, term string, fetch, sample int) ([]int, error) {
	var q searchQuery
	where := "i.search_vector @@ websearch_to_tsquery('english', " + q.arg(term) + ")"
	return s.sampledByStars(ctx, &q, f, where, fetch, sample)
}

// authorMatches: issues whose author's login contains the term.
func (s *Service) authorMatches(ctx context.Context, f Filters, term string, fetch, sample int) ([]int, error) {
	var q searchQuery
	where := "i.author ILIKE " + q.arg("%"+escapeLike(term)+"%")
	return s.sampledByStars(ctx, &q, f, where, fetch, sample)
}
