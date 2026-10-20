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
