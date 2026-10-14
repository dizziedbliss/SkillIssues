package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/jackc/pgx/v5"
)

// first_seen_at is set once, when the issue is first stored; last_synced_at
// moves on every later upsert. Neither is GitHub's created_at/updated_at.
func TestUpsertIssueRecordsFetchTimes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	name := fmt.Sprintf("upsert-test/%d", time.Now().UnixNano())
	var repoID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url) VALUES ($1, $2) RETURNING id`,
		[]any{name, "https://github.com/" + name},
		func(row pgx.Row) error { return row.Scan(&repoID) },
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	ghTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	node := github.GraphQLIssueNode{
		Number: 1, Title: "t", URL: "https://github.com/" + name + "/issues/1", State: "OPEN",
		CreatedAt: ghTime, UpdatedAt: ghTime, AuthorLogin: "someone",
	}
	upsert := func() {
		t.Helper()
		if err := pool.InTx(ctx, func(tx pgx.Tx) error {
			return UpsertIssueTx(ctx, tx, repoID, node, "Go")
		}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	times := func() (firstSeen, lastSynced, updated time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx,
			`SELECT first_seen_at, last_synced_at, updated_at FROM issues WHERE repository_id = $1 AND number = 1`,
			[]any{repoID},
			func(row pgx.Row) error { return row.Scan(&firstSeen, &lastSynced, &updated) },
		); err != nil {
			t.Fatalf("read issue: %v", err)
		}
		return
	}

	upsert()
	seen1, synced1, updated1 := times()
	if !updated1.Equal(ghTime) {
		t.Errorf("updated_at = %v, want GitHub's %v", updated1, ghTime)
	}
	if time.Since(seen1) > time.Minute || !seen1.Equal(synced1) {
		t.Errorf("after insert: first_seen_at %v, last_synced_at %v; want both ≈ now and equal", seen1, synced1)
	}

	time.Sleep(10 * time.Millisecond)
	upsert()
	seen2, synced2, _ := times()
	if !seen2.Equal(seen1) {
		t.Errorf("first_seen_at changed on re-sync: %v → %v", seen1, seen2)
	}
	if !synced2.After(synced1) {
		t.Errorf("last_synced_at did not advance on re-sync: %v → %v", synced1, synced2)
	}
}
