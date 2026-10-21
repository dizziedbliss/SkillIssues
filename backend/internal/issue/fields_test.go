package issue

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// An issue carries its GitHub labels as they are, and its repository's three
// largest languages by name, largest first.
func TestIssueLabelsAndTech(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := seedBrowseRepo(t, pool, "fields")
	if err := pool.Exec(ctx, `UPDATE repositories SET languages = $2 WHERE id = $1`, repoID,
		`[{"name":"Go","size":10},{"name":"Jupyter Notebook","size":50},{"name":"","size":99},{"name":"MDX","size":5},{"name":"Shell","size":1}]`); err != nil {
		t.Fatalf("set languages: %v", err)
	}
	var id int
	if err := pool.QueryRow(ctx,
		`INSERT INTO issues (repository_id, number, title, url, difficulty, labels)
		 VALUES ($1, 1, 't', 'u', 'UNCLASSIFIED', $2) RETURNING id`,
		[]any{repoID, []string{"good first issue", "Docs"}}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	got, err := New(pool, nil).GetIssue(ctx, id)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if want := []string{"good first issue", "Docs"}; !slices.Equal(got.Labels, want) {
		t.Errorf("labels = %q, want %q", got.Labels, want)
	}
	if want := []string{"Jupyter Notebook", "Go", "MDX"}; !slices.Equal(got.Tech, want) {
		t.Errorf("tech = %q, want %q", got.Tech, want)
	}

	// No languages and no labels are empty lists, never null.
	if err := pool.Exec(ctx, `UPDATE repositories SET languages = NULL WHERE id = $1`, repoID); err != nil {
		t.Fatalf("clear languages: %v", err)
	}
	if err := pool.Exec(ctx, `UPDATE issues SET labels = '{}' WHERE id = $1`, id); err != nil {
		t.Fatalf("clear labels: %v", err)
	}
	got, err = New(pool, nil).GetIssue(ctx, id)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if got.Labels == nil || got.Tech == nil || len(got.Labels)+len(got.Tech) != 0 {
		t.Errorf("empty: labels = %#v, tech = %#v, want two empty lists", got.Labels, got.Tech)
	}
}

// repo and owner narrow the catalog to one repository or one owner's,
// whatever the case, both ways of reading.
func TestBrowseIssuesRepoAndOwnerFilters(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	owner := fmt.Sprintf("Owner%d", time.Now().UnixNano())
	repo := func(name string) {
		var id int
		if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url, language) VALUES ($1, 'u', 'Go') RETURNING id`,
			[]any{owner + "/" + name}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
			t.Fatalf("insert repository: %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
		seedBrowseIssues(t, pool, id, 3)
	}
	repo("Alpha")
	repo("beta")
	other, _ := seedBrowseRepo(t, pool, "not-this-owner")
	seedBrowseIssues(t, pool, other, 3)

	svc := New(pool, nil)
	reposOf := func(f Filters) map[string]int {
		got := map[string]int{}
		for _, id := range walkCatalog(t, svc, f, nil, nil) {
			it, err := svc.GetIssue(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			got[it.Repository]++
		}
		return got
	}
	for _, rareBelow := range rareThresholds {
		base := Filters{Sort: SortEasiest, Limit: 2, RareLanguageBelow: rareBelow}

		f := base
		f.Repos = []string{strings.ToLower(owner) + "/ALPHA"}
		if got := reposOf(f); len(got) != 1 || got[owner+"/Alpha"] != 3 {
			t.Errorf("rare below %d: repo filter listed %v, want the 3 issues of %s/Alpha", rareBelow, got, owner)
		}

		f = base
		f.Owners = []string{strings.ToUpper(owner)}
		if got := reposOf(f); len(got) != 2 || got[owner+"/Alpha"] != 3 || got[owner+"/beta"] != 3 {
			t.Errorf("rare below %d: owner filter listed %v, want both of %s's repositories", rareBelow, got, owner)
		}

		// The shuffled feed still gives each repository one slot.
		f.Sort = SortShuffle
		if got := reposOf(f); len(got) != 2 || got[owner+"/Alpha"] != 1 || got[owner+"/beta"] != 1 {
			t.Errorf("rare below %d: shuffled owner filter listed %v, want one issue from each repository", rareBelow, got)
		}
	}
}

// Hidden issues and repositories leave the lists, and the cursor still
// walks past a page that loses every issue to hiding.
func TestBrowseLeavesHiddenOut(t *testing.T) {
	pool := testPool(t)
	repoID, lang := seedBrowseRepo(t, pool, "hidden")
	seedBrowseIssues(t, pool, repoID, 6)
	svc := New(pool, nil)
	all := walkCatalog(t, svc, Filters{Languages: []string{lang}, Sort: SortEasiest, Limit: 2, RareLanguageBelow: 1000}, nil, nil)
	if len(all) != 6 {
		t.Fatalf("walked %d issues, want 6", len(all))
	}

	// The first page's two issues are hidden: the walk carries on to the rest.
	svc.SetHidden(func() Hidden { return Hidden{Issues: map[int]bool{all[0]: true, all[1]: true}} })
	if got := walkCatalog(t, svc, Filters{Languages: []string{lang}, Sort: SortEasiest, Limit: 2, RareLanguageBelow: 1000}, nil, nil); !slices.Equal(got, all[2:]) {
		t.Errorf("with two hidden, walked %v, want %v", got, all[2:])
	}

	// A hidden repository takes all of its issues out.
	svc.SetHidden(func() Hidden { return Hidden{Repositories: map[int]bool{repoID: true}} })
	if got := walkCatalog(t, svc, Filters{Languages: []string{lang}, Sort: SortEasiest, Limit: 2, RareLanguageBelow: 1000}, nil, nil); len(got) != 0 {
		t.Errorf("with the repository hidden, walked %v, want none", got)
	}
}

// ?unassigned leaves out issues someone is assigned to, keeps the ones never
// read (NULL is unknown, not assigned), and still walks every page.
func TestBrowseUnassigned(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, lang := seedBrowseRepo(t, pool, "assigned")
	seedBrowseIssues(t, pool, repoID, 6)
	if err := pool.Exec(ctx, `UPDATE issues SET assignees = CASE number WHEN 1 THEN 1 WHEN 2 THEN 0 ELSE NULL END WHERE repository_id = $1`, repoID); err != nil {
		t.Fatal(err)
	}
	svc := New(pool, nil)
	for _, sort := range []Sort{SortEasiest, SortNewest, SortShuffle} {
		for _, rareBelow := range rareThresholds {
			got := walkCatalog(t, svc, Filters{Languages: []string{lang}, Sort: sort, Unassigned: true, Limit: 2, RareLanguageBelow: rareBelow}, nil, nil)
			for _, id := range got {
				it, err := svc.GetIssue(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				if it.Assignees != nil && *it.Assignees > 0 {
					t.Errorf("%s, rare below %d: listed assigned issue #%d", sort, rareBelow, it.Number)
				}
			}
			if sort == SortEasiest && len(got) != 5 {
				t.Errorf("rare below %d: walked %d issues, want the 5 not assigned", rareBelow, len(got))
			}
		}
	}
}

// A webhook closes or reopens an issue only when it is newer than what is
// stored: a delivery that arrives late, or a redelivered old one, never
// undoes a later change.
func TestSetStateByRepoAndNumberIgnoresStaleEvents(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, lang := seedBrowseRepo(t, pool, "webhook")
	stored := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var id int
	if err := pool.QueryRow(ctx,
		`INSERT INTO issues (repository_id, number, title, url, difficulty, labels, state, updated_at)
		 VALUES ($1, 7, 't', 'u', 'UNCLASSIFIED', '{}', 'OPEN', $2) RETURNING id`,
		[]any{repoID, stored}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	svc := New(pool, nil)
	read := func() (string, time.Time) {
		t.Helper()
		var state string
		var updated time.Time
		if err := pool.QueryRow(ctx, `SELECT state, updated_at FROM issues WHERE id = $1`, []any{id},
			func(row pgx.Row) error { return row.Scan(&state, &updated) }); err != nil {
			t.Fatalf("read issue: %v", err)
		}
		return state, updated
	}
	set := func(state string, at time.Time) {
		t.Helper()
		if err := svc.SetStateByRepoAndNumber(ctx, "browse-test/"+lang, 7, state, at); err != nil {
			t.Fatalf("SetStateByRepoAndNumber(%s, %s): %v", state, at, err)
		}
	}

	set("CLOSED", stored.Add(-time.Hour)) // older than the stored issue
	if state, updated := read(); state != "OPEN" || !updated.Equal(stored) {
		t.Fatalf("stale event: state %s at %s, want OPEN at %s", state, updated, stored)
	}
	set("CLOSED", stored) // the same moment is not newer either
	if state, _ := read(); state != "OPEN" {
		t.Fatalf("same-time event: state %s, want OPEN", state)
	}

	closed := stored.Add(time.Hour)
	set("CLOSED", closed)
	if state, updated := read(); state != "CLOSED" || !updated.Equal(closed) {
		t.Fatalf("newer event: state %s at %s, want CLOSED at %s", state, updated, closed)
	}
	set("OPEN", stored.Add(30*time.Minute)) // a reopen from before the close
	if state, _ := read(); state != "CLOSED" {
		t.Fatalf("late reopen: state %s, want CLOSED", state)
	}
}
