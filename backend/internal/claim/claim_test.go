package claim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
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

func TestClaimLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	scan := func(dst *int) func(pgx.Row) error { return func(r pgx.Row) error { return r.Scan(dst) } }
	user := func(tag string) int {
		var id int
		if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
			[]any{fmt.Sprintf("claim-%s-%d", tag, n)}, scan(&id)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
		return id
	}
	alice, bob := user("a"), user("b")
	var repo int
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`,
		[]any{fmt.Sprintf("claim-test/%d", n)}, scan(&repo)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repo); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	issue := func(num int, state string) int {
		var id int
		if err := pool.QueryRow(ctx, `INSERT INTO issues (repository_id, number, title, url, difficulty, state) VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED', $3) RETURNING id`,
			[]any{repo, num, state}, scan(&id)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	one, two, three, closed := issue(1, "OPEN"), issue(2, "OPEN"), issue(3, "OPEN"), issue(4, "CLOSED")
	s := New(pool)

	if _, err := s.Claim(ctx, alice, closed, 7, 2); !errors.Is(err, ErrClosed) {
		t.Errorf("closed issue: %v", err)
	}
	if _, err := s.Claim(ctx, alice, 999_999_999, 7, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing issue: %v", err)
	}
	c, err := s.Claim(ctx, alice, one, 7, 2)
	if err != nil || c.IssueID != one || c.ExpiresAt.Before(time.Now().Add(6*24*time.Hour)) {
		t.Fatalf("claim = %+v, %v", c, err)
	}
	if again, err := s.Claim(ctx, alice, one, 7, 2); err != nil || !again.ClaimedAt.Equal(c.ClaimedAt) {
		t.Errorf("claiming your own again = %+v, %v; want the same claim", again, err)
	}
	if _, err := s.Claim(ctx, bob, one, 7, 2); !errors.Is(err, ErrTaken) {
		t.Errorf("bob on alice's issue: %v", err)
	}
	if _, err := s.Claim(ctx, alice, two, 7, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(ctx, alice, three, 7, 2); !errors.Is(err, ErrTooMany) {
		t.Errorf("third claim with a cap of two: %v", err)
	}

	st, err := s.Of(ctx, bob, one)
	if err != nil || !st.Claimed || st.Yours != nil {
		t.Errorf("bob's view = %+v, %v", st, err)
	}
	if st, err := s.Of(ctx, alice, one); err != nil || st.Yours == nil || st.Yours.Extended {
		t.Errorf("alice's view = %+v, %v", st, err)
	}
	ids, err := s.ActiveIssues(ctx)
	if err != nil || !ids[one] || !ids[two] || ids[three] {
		t.Errorf("active = %v, %v", ids, err)
	}

	// Past its days, a claim stays while alice has an open pull request that
	// closes the issue, and lapses once it doesn't.
	if err := pool.Exec(ctx, `UPDATE issue_claims SET expires_at = now() - interval '1 day' WHERE issue_id = $1`, one); err != nil {
		t.Fatal(err)
	}
	var pr int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO pull_requests (github_id, user_id, repository_id, number, title, url, state, created_at, updated_at, author_association)
		 VALUES ($1, $2, $3, 9, 't', 'u', 'OPEN', now(), now(), 'NONE') RETURNING id`,
		[]any{n % 1_000_000_000, alice, repo}, func(r pgx.Row) error { return r.Scan(&pr) }); err != nil {
		t.Fatal(err)
	}
	if err := pool.Exec(ctx, `INSERT INTO pull_request_issues (pull_request_id, issue_id) VALUES ($1, $2)`, pr, one); err != nil {
		t.Fatal(err)
	}
	if st, err := s.Of(ctx, alice, one); err != nil || st.Yours == nil || !st.Yours.Extended {
		t.Errorf("kept by an open pull request: %+v, %v", st, err)
	}
	if err := pool.Exec(ctx, `UPDATE pull_requests SET state = 'CLOSED' WHERE id = $1`, pr); err != nil {
		t.Fatal(err)
	}
	if st, err := s.Of(ctx, bob, one); err != nil || st.Claimed {
		t.Errorf("lapsed claim still shows: %+v, %v", st, err)
	}
	// Bob can take it now; alice's lapsed claim is retired as expired.
	if _, err := s.Claim(ctx, bob, one, 7, 2); err != nil {
		t.Fatalf("bob after alice's lapsed: %v", err)
	}
	var reason string
	if err := pool.QueryRow(ctx, `SELECT release_reason FROM issue_claims WHERE issue_id = $1 AND user_id = $2`, []any{one, alice},
		func(r pgx.Row) error { return r.Scan(&reason) }); err != nil || reason != "expired" {
		t.Errorf("alice's old claim: %q, %v", reason, err)
	}

	// Releasing frees a place under the cap.
	if err := s.Release(ctx, alice, two); err != nil {
		t.Fatal(err)
	}
	if err := s.Release(ctx, alice, two); !errors.Is(err, ErrNotYours) {
		t.Errorf("releasing twice: %v", err)
	}
	if _, err := s.Claim(ctx, alice, three, 7, 2); err != nil {
		t.Errorf("after a release: %v", err)
	}
	mine, err := s.Mine(ctx, alice)
	if err != nil || len(mine) != 1 || mine[0].IssueID != three {
		t.Errorf("alice's claims = %+v, %v", mine, err)
	}
}
