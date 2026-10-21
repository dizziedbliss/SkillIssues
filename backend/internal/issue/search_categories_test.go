package issue

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_off\`); got != `50\%\_off\\` {
		t.Errorf("escapeLike = %q", got)
	}
}

// Search returns each issue once, in the first category it matches, with the
// categories interleaved rank by rank; a term's % is literal, not a wildcard.
func TestSearchCategories(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	word := fmt.Sprintf("zq%d", stamp) // one token: full-text matches it whole
	lang := "Lang" + word

	insertRepo := func(name, language string, stars int) int {
		t.Helper()
		var id int
		if err := pool.QueryRow(ctx,
			`INSERT INTO repositories (full_name, url, language, stars) VALUES ($1, 'u', $2, $3) RETURNING id`,
			[]any{name, language, stars}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
		return id
	}
	insertIssue := func(repo, number int, title, author string) int {
		t.Helper()
		var id int
		if err := pool.QueryRow(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty, author) VALUES ($1, $2, $3, 'u', 'UNCLASSIFIED', $4) RETURNING id`,
			[]any{repo, number, title, author}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
			t.Fatal(err)
		}
		return id
	}

	named := insertRepo("org/"+word, "Go", 10) // Repository: exact name
	other := insertRepo(fmt.Sprintf("org/plain-%d", stamp), "Go", 500)
	inLang := insertRepo(fmt.Sprintf("org/lang-%d", stamp), lang, 50)

	repoIssue := insertIssue(named, 1, "also mentions "+word, "someone") // Repository wins over Title
	titleIssue := insertIssue(other, 1, "crash in "+word+" parser", "someone")
	titleIssue2 := insertIssue(other, 2, word+" again", "someone")
	authorIssue := insertIssue(other, 3, "unrelated", "dev-"+word) // authors aren't searched
	langIssue := insertIssue(inLang, 1, "unrelated", "someone")
	// Language also matches a repository that uses the language without it
	// being the main one, after those whose main language it is.
	usesLang := insertRepo(fmt.Sprintf("org/uses-%d", stamp), "Go", 900)
	if err := pool.Exec(ctx, `UPDATE repositories SET languages = $2 WHERE id = $1`, usesLang,
		fmt.Sprintf(`[{"name":"Go","size":9},{"name":%q,"size":1}]`, lang)); err != nil {
		t.Fatal(err)
	}
	techIssue := insertIssue(usesLang, 1, "unrelated", "someone")

	svc := New(pool, func() int { return 25 })
	searchIn := func(category, term string, limit, page int) []Issue {
		t.Helper()
		got, err := svc.SearchIssues(ctx, Filters{Search: term, Category: category, Limit: limit, Page: page, SearchSample: 2000, SearchPerRepo: 3}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	search := func(term string, limit, page int) []Issue { t.Helper(); return searchIn("", term, limit, page) }

	got := search(word, 50, 1)
	want := []struct {
		id  int
		cat string
	}{
		// Rank 1 of each category, then rank 2. The two title matches share a
		// repository and a difficulty, so the more recently updated (the
		// later insert) ranks first.
		{repoIssue, "Repository"}, {titleIssue2, "Title"},
		{titleIssue, "Title"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results %v, want %d", len(got), summary(got), len(want))
	}
	for i, w := range want {
		if got[i].ID != w.id || got[i].MatchCategory != w.cat {
			t.Errorf("result %d = #%d %s, want #%d %s (all: %v)", i, got[i].ID, got[i].MatchCategory, w.id, w.cat, summary(got))
		}
	}

	for _, it := range got {
		if it.ID == authorIssue {
			t.Errorf("an author's login matched: %v", summary(got))
		}
	}

	if got := search(lang, 50, 1); len(got) != 2 || got[0].ID != langIssue || got[1].ID != techIssue || got[1].MatchCategory != "Language" {
		t.Errorf("language search = %v, want #%d then #%d, both Language", summary(got), langIssue, techIssue)
	}

	// Paging slices the same interleaved list.
	if p2 := search(word, 2, 2); len(p2) != 1 || p2[0].ID != titleIssue {
		t.Errorf("page 2 of 2 = %v", summary(p2))
	}

	// One category alone, paged on its own; an issue a higher category took
	// in the mixed list still belongs to every category it matches.
	if got := searchIn("Title", word, 50, 1); len(got) != 3 || got[0].MatchCategory != "Title" {
		t.Errorf("title alone = %v, want its 3 matches", summary(got))
	}
	if p := searchIn("Title", word, 2, 2); len(p) != 1 {
		t.Errorf("title alone, page 2 of 2 = %v, want 1", summary(p))
	}
	if got := searchIn("Repository", word, 50, 1); len(got) != 1 || got[0].ID != repoIssue {
		t.Errorf("repository alone = %v, want #%d", summary(got), repoIssue)
	}
	if p9 := search(word, 2, 9); len(p9) != 0 {
		t.Errorf("past the end = %v, want none", summary(p9))
	}

	// Sorted by newest when asked: the match order no longer applies.
	if err := pool.Exec(ctx, `UPDATE issues SET created_at = now() - make_interval(hours => id % 1000) WHERE id = ANY($1)`,
		[]int{repoIssue, titleIssue, titleIssue2}); err != nil {
		t.Fatal(err)
	}
	newest, err := svc.SearchIssues(ctx, Filters{Search: word, Limit: 50, Page: 1, SearchSample: 2000, SearchPerRepo: 3, SearchSort: SortNewest}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 3 {
		t.Fatalf("sorted search = %v, want 3", summary(newest))
	}
	for i := 1; i < len(newest); i++ {
		if newest[i-1].CreatedAt < newest[i].CreatedAt {
			t.Errorf("sorted search not newest first: %v", summary(newest))
		}
	}

	// % is literal: "zq%" must not match every zq-something repository.
	if got := search("zq%"+word[2:], 50, 1); len(got) != 0 {
		t.Errorf("a %% in the term acted as a wildcard: %v", summary(got))
	}
}

// Interleaving takes one of each category per rank, each issue once, and a
// category whose next issue was taken moves on to its following one. Any
// prefix of the list is the same however deep the categories were read.
func TestInterleave(t *testing.T) {
	found := [][]int{{1, 2, 3}, {2, 4, 5, 6}, {1, 7}}
	got := interleave(found)
	want := []searchHit{
		{1, matchCategories[0]}, {2, matchCategories[1]}, {7, matchCategories[2]},
		{3, matchCategories[0]}, {4, matchCategories[1]},
		{5, matchCategories[1]},
		{6, matchCategories[1]},
	}
	if !slices.Equal(got, want) {
		t.Errorf("interleave = %v, want %v", got, want)
	}
	// Each category read twice as deep as a prefix of n needs (searchCategories'
	// fetch): those n are the same as the deep read's.
	for n := 1; n <= len(want); n++ {
		cut := make([][]int, len(found))
		for c, ids := range found {
			cut[c] = ids[:min(len(ids), 2*n)]
		}
		if got := interleave(cut); len(got) < n || !slices.Equal(got[:n], want[:n]) {
			t.Errorf("read %d deep: %v, want a prefix %v", 2*n, got, want[:n])
		}
	}
	if got := interleave([][]int{nil, nil, nil}); len(got) != 0 {
		t.Errorf("nothing found = %v", got)
	}
}

func summary(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, it := range issues {
		out[i] = fmt.Sprintf("#%d %s", it.ID, it.MatchCategory)
	}
	return out
}

func TestParseCategory(t *testing.T) {
	for in, want := range map[string]string{"": "", "repository": "Repository", "TITLE": "Title", "Language": "Language"} {
		if got, err := ParseCategory(in); err != nil || got != want {
			t.Errorf("ParseCategory(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"author", "everything"} {
		if _, err := ParseCategory(bad); err == nil {
			t.Errorf("ParseCategory(%q): want an error", bad)
		}
	}
}
