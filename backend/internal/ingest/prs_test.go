package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/xp"
)

var testRules = xp.Rules{
	BaseBeginner: 75, BaseIntermediate: 140, BaseAdvanced: 250, BaseUnclassified: 100, BaseUnlinked: 50,
	FirstMergeRepoBonus: 15, BacklogBonus: 50, BacklogDays: 180, BackfillPercent: 50, Opened: 5, OpenedWeeklyCap: 3,
	LevelStep: 100,
}

var testIntervals = PRIntervals{Hot: 15 * time.Minute, Warm: time.Hour, Cold: 24 * time.Hour, HotWindow: 7 * 24 * time.Hour, RetryBase: 5 * time.Minute}

// prTestWorld is one person, two catalog repositories with GitHub ids, and a
// beginner issue open long enough to be backlog.
type prTestWorld struct {
	userID       int
	ghID         int64
	login        string
	joined       time.Time
	repoA, repoB int
	ghA, ghB     int64
	nameA        string
	issueID      int
}

func newPRTestWorld(t *testing.T, pool *db.Pool) prTestWorld {
	t.Helper()
	ctx := context.Background()
	n := time.Now().UnixNano()
	w := prTestWorld{ghID: n % 1_000_000_000, login: fmt.Sprintf("pr-test-%d", n), joined: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		ghA: n%1_000_000 + 7_000_000_000, ghB: n%1_000_000 + 8_000_000_000}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (github_id, username, created_at) VALUES ($1, $2, $3) RETURNING id`,
		[]any{fmt.Sprint(w.ghID), w.login, w.joined}, func(row pgx.Row) error { return row.Scan(&w.userID) }); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, w.userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	repo := func(gh int64) (int, string) {
		name := fmt.Sprintf("pr-test/%d", gh)
		var id int
		if err := pool.QueryRow(ctx,
			`INSERT INTO repositories (full_name, url, language, github_database_id) VALUES ($1, 'u', 'Go', $2) RETURNING id`,
			[]any{name, gh}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
			t.Fatalf("insert repository: %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup repository: %v", err)
			}
		})
		return id, name
	}
	w.repoA, w.nameA = repo(w.ghA)
	w.repoB, _ = repo(w.ghB)
	if err := pool.QueryRow(ctx,
		`INSERT INTO issues (repository_id, number, title, url, difficulty, labels, created_at, updated_at)
		 VALUES ($1, 3, 't', 'u', 'BEGINNER', '{docs}', $2, $2) RETURNING id`,
		[]any{w.repoA, w.joined.AddDate(-1, 0, 0)}, func(row pgx.Row) error { return row.Scan(&w.issueID) }); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	return w
}

func (w prTestWorld) state(t *testing.T, pool *db.Pool) PRSyncState {
	t.Helper()
	due, err := DuePRSync(context.Background(), pool, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range due {
		if st.UserID == w.userID {
			return st
		}
	}
	t.Fatalf("user %d not due", w.userID)
	return PRSyncState{}
}

func day(base time.Time, d int) *time.Time {
	t := base.AddDate(0, 0, d)
	return &t
}

func (w prTestWorld) pull(id int64, gh int64, state string, merged *time.Time, updated time.Time) github.UserPullRequest {
	return github.UserPullRequest{
		DatabaseID: id, Number: int(id % 1000), Title: "t", URL: "u", State: state, CreatedAt: updated.Add(-time.Hour),
		UpdatedAt: updated, MergedAt: merged, AuthorAssociation: "CONTRIBUTOR", ChangesRequested: 1,
		RepositoryID: gh, Repository: fmt.Sprintf("pr-test/%d", gh),
	}
}

func TestPullRequestPassBackfillsAwardsAndAdvances(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	w := newPRTestWorld(t, pool)
	if _, err := EnsurePRSync(ctx, pool); err != nil {
		t.Fatal(err)
	}
	base := w.joined.AddDate(0, 1, 0) // a month after signing up
	now := base.AddDate(0, 0, 30)
	id := w.ghID * 10

	// Page 1, newest first: a merge closing the backlog issue, and one to a
	// repository outside the catalog.
	recent := w.pull(id+1, w.ghA, "MERGED", day(base, 2), *day(base, 2))
	recent.Closes = []github.ClosingIssue{{RepositoryID: w.ghA, Repository: w.nameA, Number: 3}}
	outside := w.pull(id+2, 42, "MERGED", day(base, 1), *day(base, 1))
	out, err := ApplyPullsPage(ctx, pool, w.state(t, pool), github.UserPullsPage{
		Found: true, DatabaseID: w.ghID, HasNextPage: true, EndCursor: "c1",
		Pulls: []github.UserPullRequest{recent, outside},
	}, testRules, testIntervals, now)
	if err != nil {
		t.Fatal(err)
	}
	if out.Stored != 1 || out.Complete || len(out.Awards) != 0 {
		t.Fatalf("page 1 = %+v, want 1 stored, pass still running, nothing paid", out)
	}
	st := w.state(t, pool) // still due: the pass goes on next cycle
	if st.PassCursor == nil || *st.PassCursor != "c1" || st.SyncedThrough != nil {
		t.Fatalf("after page 1: %+v", st)
	}

	// Page 2, the last: an older merge in the same repository from before
	// signing up (backfill), and one to their own repository.
	old := w.pull(id+3, w.ghA, "MERGED", day(w.joined, -40), *day(w.joined, -40))
	own := w.pull(id+4, w.ghB, "MERGED", day(base, -5), *day(base, -5))
	own.OwnerID = &w.ghID
	out, err = ApplyPullsPage(ctx, pool, st, github.UserPullsPage{
		Found: true, DatabaseID: w.ghID, Pulls: []github.UserPullRequest{old, own},
	}, testRules, testIntervals, now)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Complete || out.Awards[xp.SourceMerged] != 1 || out.Awards[xp.SourceBackfill] != 1 || out.Awards[xp.SourceOpened] != 1 {
		t.Fatalf("page 2 = %+v, want the pass complete with a merge, a backfill and an opening paid", out)
	}

	type row struct {
		source string
		amount int
		gh     int64
	}
	var ledger []row
	if err := pool.QueryRows(ctx,
		`SELECT e.source, e.amount, p.github_id FROM xp_events e JOIN pull_requests p ON p.id = e.pull_request_id
		 WHERE e.user_id = $1 ORDER BY e.earned_at`, []any{w.userID},
		func(rows pgx.Rows) error {
			var r row
			if err := rows.Scan(&r.source, &r.amount, &r.gh); err != nil {
				return err
			}
			ledger = append(ledger, r)
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	// The backfilled merge is the first in the repository (unlinked 50 +
	// first 15, at half); the recent one closes the year-old beginner issue
	// (75 + backlog 50), without the first-merge bonus, and its opening
	// earned the small reward. Their own repository pays nothing.
	want := []row{{xp.SourceBackfill, 32, id + 3}, {xp.SourceOpened, 5, id + 1}, {xp.SourceMerged, 125, id + 1}}
	if len(ledger) != len(want) || ledger[0] != want[0] || ledger[1] != want[1] || ledger[2] != want[2] {
		t.Errorf("ledger = %+v, want %+v", ledger, want)
	}
	var reason *string
	if err := pool.QueryRow(ctx, `SELECT no_xp_reason FROM pull_requests WHERE github_id = $1`, []any{id + 4},
		func(r pgx.Row) error { return r.Scan(&reason) }); err != nil {
		t.Fatal(err)
	}
	if reason == nil || *reason != "own_repository" {
		t.Errorf("own repository: no_xp_reason = %v", reason)
	}
	// Only a merge since signing up notifies.
	var notes int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1 AND kind = 'pr_merged'`, []any{w.userID},
		func(r pgx.Row) error { return r.Scan(&notes) }); err != nil {
		t.Fatal(err)
	}
	if notes != 1 {
		t.Errorf("pr_merged notifications = %d, want 1", notes)
	}

	// The watermark is the newest updatedAt the pass saw, and the next read
	// is a day away: nothing is open.
	var through, next time.Time
	var cursor *string
	if err := pool.QueryRow(ctx, `SELECT synced_through, next_check_at, pass_cursor FROM pr_sync WHERE user_id = $1`, []any{w.userID},
		func(r pgx.Row) error { return r.Scan(&through, &next, &cursor) }); err != nil {
		t.Fatal(err)
	}
	if !through.Equal(*day(base, 2)) || cursor != nil || !next.Equal(now.Add(testIntervals.Cold)) {
		t.Errorf("pr_sync: through %v next %v cursor %v", through, next, cursor)
	}

	// The next pass stops at the watermark, and running the plan again pays
	// nothing twice.
	if err := RequestPRSync(ctx, pool, w.userID); err != nil {
		t.Fatal(err)
	}
	open := w.pull(id+5, w.ghA, "OPEN", nil, *day(base, 28))
	out, err = ApplyPullsPage(ctx, pool, w.state(t, pool), github.UserPullsPage{
		Found: true, DatabaseID: w.ghID, HasNextPage: true, EndCursor: "c9",
		Pulls: []github.UserPullRequest{open, recent, old},
	}, testRules, testIntervals, now)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Complete || out.Stored != 2 || len(out.Awards) != 0 {
		t.Errorf("incremental pass = %+v, want complete at the watermark, 2 re-read, nothing new paid", out)
	}
	if err := pool.QueryRow(ctx, `SELECT next_check_at FROM pr_sync WHERE user_id = $1`, []any{w.userID},
		func(r pgx.Row) error { return r.Scan(&next) }); err != nil {
		t.Fatal(err)
	}
	if !next.Equal(now.Add(testIntervals.Hot)) {
		t.Errorf("with a pull request open and moving, next read %v, want %v", next, now.Add(testIntervals.Hot))
	}
}

// A login that GitHub now gives to another account stores nothing.
func TestPullRequestPassRefusesAnotherAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	w := newPRTestWorld(t, pool)
	if _, err := EnsurePRSync(ctx, pool); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stray := w.pull(w.ghID*10+1, w.ghA, "MERGED", &now, now)
	out, err := ApplyPullsPage(ctx, pool, w.state(t, pool), github.UserPullsPage{
		Found: true, DatabaseID: w.ghID + 1, Pulls: []github.UserPullRequest{stray},
	}, testRules, testIntervals, now)
	if err != nil {
		t.Fatal(err)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pull_requests WHERE user_id = $1`, []any{w.userID},
		func(r pgx.Row) error { return r.Scan(&stored) }); err != nil {
		t.Fatal(err)
	}
	if !out.OtherAccount || stored != 0 {
		t.Errorf("outcome %+v, stored %d: want other_account and nothing stored", out, stored)
	}
}
