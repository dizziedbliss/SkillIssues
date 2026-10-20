package issue

import (
	"context"
	"slices"
	"testing"

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
