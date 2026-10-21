package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/jackc/pgx/v5"
)

// The assignee count is written when a source says it, and kept when a
// source doesn't (an archive event that carries no assignees).
func TestWriteIssueAssignees(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	var repoID int
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`,
		[]any{fmt.Sprintf("assignees-test/r-%d", time.Now().UnixNano())}, func(row pgx.Row) error { return row.Scan(&repoID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	write := func(at time.Time, assignees *int, mode issueWrite) {
		t.Helper()
		node := github.GraphQLIssueNode{DatabaseID: 1, Number: 7, Title: "t", URL: "u", State: "OPEN", UpdatedAt: at, Assignees: assignees}
		if err := pool.InTx(ctx, func(tx pgx.Tx) error {
			_, err := writeIssueTx(ctx, tx, repoID, node, "Go", mode)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() *int {
		t.Helper()
		var n *int
		if err := pool.QueryRow(ctx, `SELECT assignees FROM issues WHERE repository_id = $1 AND number = 7`, []any{repoID},
			func(row pgx.Row) error { return row.Scan(&n) }); err != nil {
			t.Fatal(err)
		}
		return n
	}
	two, base := 2, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	write(base, nil, writeIfNewer)
	if n := read(); n != nil {
		t.Errorf("unknown written as %d, want NULL", *n)
	}
	write(base.Add(time.Hour), &two, writeIfNewer)
	if n := read(); n == nil || *n != 2 {
		t.Errorf("after a crawl: %v, want 2", n)
	}
	write(base.Add(2*time.Hour), nil, writeIfNewer)
	write(base.Add(3*time.Hour), nil, updateIfNewer)
	if n := read(); n == nil || *n != 2 {
		t.Errorf("after sources that didn't say: %v, want 2 kept", n)
	}
}
