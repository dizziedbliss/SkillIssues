package profile

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

// GET /me says when the account was created, for the profile's "joined on".
func TestFullProfileJoinedAt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	login := fmt.Sprintf("profile-test-%d", time.Now().UnixNano())
	created := time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	var id int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username, created_at) VALUES ($1, $1, $2) RETURNING id`,
		[]any{login, created}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	p, err := New(pool).GetFullProfile(ctx, id)
	if err != nil {
		t.Fatalf("GetFullProfile: %v", err)
	}
	if p.JoinedAt != "2026-09-20T08:30:00Z" {
		t.Errorf("joined_at = %q, want 2026-09-20T08:30:00Z", p.JoinedAt)
	}
}
