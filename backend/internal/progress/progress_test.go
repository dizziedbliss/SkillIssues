package progress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/xp"
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

func TestProgressReadsWhatTheTrackerWrote(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	scan := func(dst any) func(pgx.Row) error { return func(r pgx.Row) error { return r.Scan(dst) } }
	var userID, repoID int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("progress-%d", n)}, scan(&userID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url, language) VALUES ($1, 'u', 'Go') RETURNING id`,
		[]any{fmt.Sprintf("progress-test/%d", n)}, scan(&repoID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	s := New(pool)

	// Before ingestd enrols them: not tracked, an empty ledger.
	if st, err := s.SyncOf(ctx, userID); err != nil || st.Tracked {
		t.Fatalf("SyncOf before enrolment = %+v, %v", st, err)
	}

	merged := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var prID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO pull_requests (github_id, user_id, repository_id, number, title, url, state, created_at, updated_at, merged_at, author_association)
		 VALUES ($1, $2, $3, 4, 'Fix it', 'https://github.com/o/r/pull/4', 'MERGED', $4, $4, $4, 'CONTRIBUTOR') RETURNING id`,
		[]any{n % 1_000_000_000, userID, repoID, merged}, scan(&prID)); err != nil {
		t.Fatal(err)
	}
	if err := pool.Exec(ctx,
		`INSERT INTO xp_events (user_id, source, pull_request_id, amount, detail, earned_at) VALUES
		   ($1, 'merged', $2, 125, '{"base":75,"backlog":50,"language":"Go","repository_id":1}', $3),
		   ($1, 'opened', $2, 5, '{"issue_id":9}', $3 - interval '1 day')`, userID, prID, merged); err != nil {
		t.Fatal(err)
	}

	events, err := s.Events(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	sum := xp.Summarise(events, 100, merged)
	if sum.Total != 130 || sum.Merges != 1 || sum.Level.Level != 2 || len(sum.Badges) < 2 {
		t.Errorf("summary = %+v", sum)
	}
	recent, err := s.Recent(ctx, userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].Source != "merged" || recent[0].Pull == nil || recent[0].Pull.Number != 4 || recent[0].Detail.Backlog != 50 {
		t.Errorf("recent = %+v", recent)
	}
	pulls, err := s.Pulls(ctx, userID, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulls) != 1 || pulls[0].XP != 130 || pulls[0].State != "MERGED" || pulls[0].Issues == nil {
		t.Errorf("pulls = %+v", pulls)
	}
	if open, err := s.Pulls(ctx, userID, true, 10); err != nil || len(open) != 0 {
		t.Errorf("open pulls = %+v, %v", open, err)
	}

	// "Check now" enrols them, then refuses until the gap has passed.
	if _, err := s.RequestCheck(ctx, userID, time.Hour); err != nil {
		t.Fatalf("first check: %v", err)
	}
	st, err := s.SyncOf(ctx, userID)
	if err != nil || !st.Tracked || !st.Backfilling || st.RequestedAt == nil {
		t.Fatalf("SyncOf after a request = %+v, %v", st, err)
	}
	next, err := s.RequestCheck(ctx, userID, time.Hour)
	if !errors.Is(err, ErrTooSoon) || next.Before(time.Now().Add(50*time.Minute)) {
		t.Errorf("second check = %v, %v; want ErrTooSoon about an hour out", next, err)
	}
	if _, err := s.RequestCheck(ctx, userID, 0); err != nil {
		t.Errorf("with no gap: %v", err)
	}
}
