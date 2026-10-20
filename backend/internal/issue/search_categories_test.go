package issue

import (
	"context"
	"fmt"
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
	authorIssue := insertIssue(other, 3, "unrelated", "dev-"+word)
	langIssue := insertIssue(inLang, 1, "unrelated", "someone")

	svc := New(pool, func() int { return 25 })
	search := func(term string, limit, page int) []Issue {
		t.Helper()
		got, err := svc.SearchIssues(ctx, Filters{Search: term, Limit: limit, Page: page, SearchSample: 2000, SearchPerRepo: 3}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	got := search(word, 50, 1)
	want := []struct {
		id  int
		cat string
	}{
		// Rank 1 of each category, then rank 2. The two title matches share a
		// repository and a difficulty, so the more recently updated (the
		// later insert) ranks first.
		{repoIssue, "Repository"}, {titleIssue2, "Title"}, {authorIssue, "Author"},
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

	if got := search(lang, 50, 1); len(got) != 1 || got[0].ID != langIssue || got[0].MatchCategory != "Language" {
		t.Errorf("language search = %v, want #%d Language", summary(got), langIssue)
	}

	// Paging slices the same interleaved list.
	if p2 := search(word, 2, 2); len(p2) != 2 || p2[0].ID != authorIssue || p2[1].ID != titleIssue {
		t.Errorf("page 2 of 2 = %v", summary(p2))
	}
	if p9 := search(word, 2, 9); len(p9) != 0 {
		t.Errorf("past the end = %v, want none", summary(p9))
	}

	// % is literal: "zq%" must not match every zq-something repository.
	if got := search("zq%"+word[2:], 50, 1); len(got) != 0 {
		t.Errorf("a %% in the term acted as a wildcard: %v", summary(got))
	}
}

func summary(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, it := range issues {
		out[i] = fmt.Sprintf("#%d %s", it.ID, it.MatchCategory)
	}
	return out
}
