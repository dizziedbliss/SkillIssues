package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

// bodyFixture is a stored repository with one issue, and a fake GitHub that
// answers body queries with whatever the test sets.
type bodyFixture struct {
	pool    *db.Pool
	repoID  int
	issueID int
	answer  string
}

func newBodyFixture(t *testing.T) *bodyFixture {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	f := &bodyFixture{pool: pool}
	name := fmt.Sprintf("body-test/r%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`,
		[]any{name}, func(row pgx.Row) error { return row.Scan(&f.repoID) }); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES ($1, 7, 't', 'u', 'UNCLASSIFIED') RETURNING id`,
		[]any{f.repoID}, func(row pgx.Row) error { return row.Scan(&f.issueID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, f.repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

func (f *bodyFixture) worker(t *testing.T, token string) *Worker {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, f.answer)
	}))
	t.Cleanup(srv.Close)
	snap := settings.Defaults()
	return NewWorker(f.pool, github.NewClient(srv.URL, func() time.Duration { return 10 * time.Second }),
		func() string { return token }, func() *settings.Snapshot { return snap })
}

func (f *bodyFixture) cached(t *testing.T) (string, string, bool) {
	t.Helper()
	var body, source string
	err := f.pool.QueryRow(context.Background(), `SELECT body, source FROM issue_bodies WHERE issue_id = $1`,
		[]any{f.issueID}, func(row pgx.Row) error { return row.Scan(&body, &source) })
	if err == db.ErrNotFound {
		return "", "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return body, source, true
}

func skipUnlessInteractiveBudget(t *testing.T, pool *db.Pool) {
	t.Helper()
	snap := settings.Defaults()
	if ok, err := CanProceed(context.Background(), pool, LaneInteractive, installationIdentity,
		snap.Int(settings.IngestRateLimitFloor), snap.Int(settings.IngestInteractiveReservePercent)); err != nil || !ok {
		t.Skipf("the installation budget is spent here (err %v)", err)
	}
}

const bodyRateLimit = `"rateLimit":{"limit":12500,"remaining":12000,"used":1,"resetAt":"2001-01-01T00:00:00Z","cost":1}`

func TestFetchIssueBodyStoresIt(t *testing.T) {
	f := newBodyFixture(t)
	skipUnlessInteractiveBudget(t, f.pool)
	f.answer = `{"data":{"repository":{"issue":{"body":"Steps to reproduce","updatedAt":"2026-10-19T08:00:00Z"}},` + bodyRateLimit + `}}`

	body, outcome, err := f.worker(t, "token").FetchIssueBody(context.Background(), f.issueID)
	if err != nil || outcome != BodyFetched || body != "Steps to reproduce" {
		t.Fatalf("got %q, %s, %v", body, outcome, err)
	}
	if got, source, ok := f.cached(t); !ok || got != body || source != "fetch" {
		t.Errorf("cached %q (%s, %v)", got, source, ok)
	}
}

func TestFetchIssueBodyOutcomes(t *testing.T) {
	f := newBodyFixture(t)
	ctx := context.Background()

	if _, outcome, err := f.worker(t, "token").FetchIssueBody(ctx, -1); err != nil || outcome != BodyNotStored {
		t.Errorf("unknown issue: %s, %v; want not-stored", outcome, err)
	}
	if _, outcome, err := f.worker(t, "").FetchIssueBody(ctx, f.issueID); err != nil || outcome != BodyNoCredential {
		t.Errorf("no token: %s, %v; want no-credential", outcome, err)
	}

	skipUnlessInteractiveBudget(t, f.pool)
	f.answer = `{"data":{"repository":null,` + bodyRateLimit + `},"errors":[{"type":"NOT_FOUND","message":"gone"}]}`
	if _, outcome, err := f.worker(t, "token").FetchIssueBody(ctx, f.issueID); err != nil || outcome != BodyGone {
		t.Errorf("deleted on GitHub: %s, %v; want gone", outcome, err)
	}
	if _, _, ok := f.cached(t); ok {
		t.Error("a body was cached for an issue GitHub no longer has")
	}
}

// The archive refreshes a cached body, and never adds one.
func TestArchiveRefreshesCachedBody(t *testing.T) {
	f := newBodyFixture(t)
	ctx := context.Background()

	write := func(body string) {
		t.Helper()
		err := f.pool.InTx(ctx, func(tx pgx.Tx) error { return refreshCachedBodyTx(ctx, tx, f.repoID, 7, body) })
		if err != nil {
			t.Fatal(err)
		}
	}
	write("from the archive")
	if _, _, ok := f.cached(t); ok {
		t.Fatal("the archive cached a body nobody asked for")
	}
	if err := f.pool.Exec(ctx, `INSERT INTO issue_bodies (issue_id, body, source) VALUES ($1, 'old', 'fetch')`, f.issueID); err != nil {
		t.Fatal(err)
	}
	write("edited")
	if got, source, _ := f.cached(t); got != "edited" || source != "archive" {
		t.Errorf("cached %q (%s), want the archive's edit", got, source)
	}
}

// Over the cap, the least recently read bodies go first; a saved issue's
// body never does.
func TestPruneIssueBodies(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	var total int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(octet_length(body)), 0) FROM issue_bodies`, nil,
		func(row pgx.Row) error { return row.Scan(&total) }); err != nil {
		t.Fatal(err)
	}
	if total > 0 {
		t.Skip("issue_bodies is not empty; this test trims the whole table")
	}

	f := newBodyFixture(t)
	var ids []int
	for n := 1; n <= 4; n++ {
		var id int
		if err := pool.QueryRow(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED') RETURNING id`,
			[]any{f.repoID, 100 + n}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
			t.Fatal(err)
		}
		// 100 bytes each; n=1 read longest ago.
		if err := pool.Exec(ctx, `INSERT INTO issue_bodies (issue_id, body, source, last_read_at) VALUES ($1, $2, 'fetch', now() - $3::interval)`,
			id, strings.Repeat("x", 100), time.Duration(10-n)*time.Hour); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	var user int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("prune-%d", time.Now().UnixNano())}, func(row pgx.Row) error { return row.Scan(&user) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	// The oldest is saved: kept whatever its age.
	if err := pool.Exec(ctx, `INSERT INTO saved_issues (user_id, issue_id) VALUES ($1, $2)`, user, ids[0]); err != nil {
		t.Fatal(err)
	}

	// Room for 300 bytes: the saved 100, then the two most recently read.
	n, err := PruneIssueBodies(ctx, pool, 300)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}
	left := map[int]bool{}
	err = pool.QueryRows(ctx, `SELECT issue_id FROM issue_bodies WHERE issue_id = ANY($1)`, []any{ids},
		func(rows pgx.Rows) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}
			left[id] = true
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !left[ids[0]] || left[ids[1]] || !left[ids[2]] || !left[ids[3]] {
		t.Errorf("left %v of %v: want the saved one and the two newest", left, ids)
	}
}
