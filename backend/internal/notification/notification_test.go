package notification

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

func testPool(t *testing.T) *db.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := db.New(context.Background(), db.DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func insertID(t *testing.T, pool *db.Pool, sql string, args ...any) int {
	t.Helper()
	var id int
	if err := pool.QueryRow(context.Background(), sql, args, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

// Closing an issue notifies everyone who saved it, once, and nobody else;
// reopening and closing again makes it unread again rather than adding a
// second row.
func TestIssueClosedNotifiesTrackingUsers(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()

	repoID := insertID(t, pool, `INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`,
		fmt.Sprintf("notify-test/%d", stamp))
	issueID := insertID(t, pool,
		`INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES ($1, 7, 'title', 'issue-url', 'UNCLASSIFIED') RETURNING id`,
		repoID)
	newUser := func(name string) int {
		login := fmt.Sprintf("notify-%s-%d", name, stamp)
		return insertID(t, pool, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`, login)
	}
	saver, saver2, bystander := newUser("saver"), newUser("saver2"), newUser("bystander")
	t.Cleanup(func() {
		bg := context.Background()
		if err := pool.Exec(bg, `DELETE FROM repositories WHERE id = (SELECT repository_id FROM issues WHERE id = $1)`, issueID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if err := pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1)`, []int{saver, saver2, bystander}); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
	})

	must := func(sql string, args ...any) {
		t.Helper()
		if err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	must(`INSERT INTO saved_issues (user_id, issue_id) VALUES ($1, $2)`, saver, issueID)
	must(`INSERT INTO saved_issues (user_id, issue_id) VALUES ($1, $2)`, saver2, issueID)

	svc := NewService(pool)
	must(`UPDATE issues SET state = 'CLOSED' WHERE id = $1`, issueID)

	for _, u := range []int{saver, saver2} {
		items, unread, err := svc.List(ctx, u, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || unread != 1 || items[0].Kind != "issue_closed" || items[0].Issue.ID != issueID ||
			items[0].Issue.State != "CLOSED" || items[0].Issue.URL != "issue-url" || items[0].ReadAt != nil {
			t.Fatalf("user %d: items %+v unread %d", u, items, unread)
		}
	}
	if items, _, err := svc.List(ctx, bystander, false, 50); err != nil || len(items) != 0 {
		t.Errorf("user who saved nothing was notified: %+v %v", items, err)
	}

	items, _, err := svc.List(ctx, saver, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := svc.MarkRead(ctx, saver, items[0].ID); err != nil || !found {
		t.Fatalf("MarkRead = %v, %v", found, err)
	}
	if found, err := svc.MarkRead(ctx, saver2, items[0].ID); err != nil || found {
		t.Errorf("another user's notification: MarkRead = %v, %v; want not found", found, err)
	}
	if unreadOnly, unread, err := svc.List(ctx, saver, true, 50); err != nil || len(unreadOnly) != 0 || unread != 0 {
		t.Errorf("after MarkRead: unread list %+v count %d err %v", unreadOnly, unread, err)
	}

	// Reopened, then closed again: same row, unread again.
	must(`UPDATE issues SET state = 'OPEN' WHERE id = $1`, issueID)
	must(`UPDATE issues SET state = 'CLOSED' WHERE id = $1`, issueID)
	again, unread, err := svc.List(ctx, saver, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || unread != 1 || again[0].ID != items[0].ID || again[0].ReadAt != nil {
		t.Errorf("after re-close: %+v unread %d", again, unread)
	}

	// An update that is not OPEN -> CLOSED notifies nobody.
	must(`UPDATE issues SET title = 'changed' WHERE id = $1`, issueID)
	if n, err := svc.MarkAllRead(ctx, saver2); err != nil || n != 1 {
		t.Errorf("MarkAllRead = %d, %v; want 1", n, err)
	}
	if _, unread, err := svc.List(ctx, saver2, false, 50); err != nil || unread != 0 {
		t.Errorf("after MarkAllRead: unread %d err %v", unread, err)
	}
}
