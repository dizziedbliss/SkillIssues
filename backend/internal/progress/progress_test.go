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

func TestPublicProfilesAndLeaderboard(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	scan := func(dst any) func(pgx.Row) error { return func(r pgx.Row) error { return r.Scan(dst) } }
	user := func(tag string, public bool) (int, string) {
		login := fmt.Sprintf("lb-%s-%d", tag, n)
		var id int
		if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username, public_profile) VALUES ($1, $1, $2) RETURNING id`,
			[]any{login, public}, scan(&id)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
		return id, login
	}
	alice, aliceLogin := user("alice", true)
	bob, bobLogin := user("bob", true)
	carol, carolLogin := user("carol", false)
	// A key per row keeps the ledger's uniqueness without pull requests.
	pay := func(id, amount int, lang string, at time.Time) {
		if err := pool.Exec(ctx,
			`INSERT INTO xp_events (user_id, source, key, amount, detail, earned_at) VALUES ($1, 'quest', $2, $3, jsonb_build_object('language', $4::text), $5)`,
			id, fmt.Sprintf("t-%d-%d", id, at.UnixNano()), amount, lang, at); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	lastYear := now.AddDate(-1, 0, 0)
	pay(alice, 300, "Go", lastYear)
	pay(alice, 100, "Go", now)
	pay(bob, 250, "Rust", now)
	pay(carol, 9999, "Go", now) // private: never listed

	s := New(pool)
	if _, err := s.PublicProfile(ctx, carolLogin, 100, 10, now); !errors.Is(err, ErrNotPublic) {
		t.Errorf("a private profile: %v", err)
	}
	if _, err := s.PublicProfile(ctx, "no-such-person-"+aliceLogin, 100, 10, now); !errors.Is(err, ErrNotPublic) {
		t.Errorf("no such person: %v", err)
	}
	p, err := s.PublicProfile(ctx, aliceLogin, 100, 10, now)
	if err != nil || p.Summary.Total != 400 || p.Username != aliceLogin {
		t.Fatalf("alice's profile = %+v, %v", p, err)
	}

	rank := func(since *time.Time, lang string) map[string]int {
		t.Helper()
		got, err := s.Leaderboard(ctx, since, lang, 100, 500)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		for _, e := range got {
			out[e.Username] = e.Rank
		}
		if _, listed := out[carolLogin]; listed {
			t.Errorf("a private profile is on the board")
		}
		return out
	}
	all := rank(nil, "")
	if all[aliceLogin] == 0 || all[bobLogin] == 0 || all[aliceLogin] > all[bobLogin] {
		t.Errorf("all time: alice %d, bob %d; want alice above bob", all[aliceLogin], all[bobLogin])
	}
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	thisMonth := rank(&month, "")
	if thisMonth[bobLogin] == 0 || thisMonth[aliceLogin] < thisMonth[bobLogin] {
		t.Errorf("this month: alice %d, bob %d; want bob above alice", thisMonth[aliceLogin], thisMonth[bobLogin])
	}
	if goOnly := rank(nil, "Go"); goOnly[bobLogin] != 0 || goOnly[aliceLogin] == 0 {
		t.Errorf("Go board = %v", goOnly)
	}
}
