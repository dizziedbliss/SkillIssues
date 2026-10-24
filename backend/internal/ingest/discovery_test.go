package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

func TestSplitPoint(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		span   time.Duration
		min    time.Duration
		ok     bool
		wantAt time.Time
	}{
		{"day halves at noon", 24 * time.Hour, time.Minute, true, from.Add(12 * time.Hour)},
		{"at the minimum span", time.Minute, time.Minute, false, time.Time{}},
		{"just over the minimum", 61 * time.Second, time.Minute, true, from.Add(30 * time.Second)},
		{"one second cannot halve on whole seconds", time.Second, 0, false, time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mid, ok := splitPoint(DiscoveryWindow{From: from, To: from.Add(c.span)}, c.min)
			if ok != c.ok || (ok && !mid.Equal(c.wantAt)) {
				t.Fatalf("splitPoint = %v, %v; want %v, %v", mid, ok, c.wantAt, c.ok)
			}
		})
	}
}

func TestSearchQuery(t *testing.T) {
	win := DiscoveryWindow{
		From:     time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC),
		To:       time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		MinStars: 25,
	}
	want := "stars:>=25 fork:false archived:false pushed:2026-10-01T06:00:00Z..2026-10-01T09:00:00Z"
	if got := searchQuery(win); got != want {
		t.Fatalf("searchQuery = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Database tests. Each builds its own sweep and windows and processes them by
// id, so it never claims (or is claimed by) anything else on the database.
// ---------------------------------------------------------------------------

// testSweep inserts a running sweep with one pending window over [from, to),
// removed when the test ends. Only one sweep may run at a time, so a sweep a
// previous test run left behind is closed first.
func testSweep(t *testing.T, pool *db.Pool, from, to time.Time) (sweepID, windowID int64) {
	t.Helper()
	ctx := context.Background()
	if err := pool.Exec(ctx, `UPDATE discovery_sweeps SET status = 'failed' WHERE status = 'running'`); err != nil {
		t.Fatalf("close stale sweeps: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO discovery_sweeps (kind, pushed_from, pushed_to, min_stars) VALUES ('full', $1, $2, 25) RETURNING id`,
		[]any{from, to}, func(row pgx.Row) error { return row.Scan(&sweepID) },
	); err != nil {
		t.Fatalf("insert sweep: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM discovery_sweeps WHERE id = $1`, sweepID); err != nil {
			t.Errorf("cleanup sweep: %v", err)
		}
	})
	if err := pool.QueryRow(ctx,
		`INSERT INTO discovery_windows (sweep_id, pushed_from, pushed_to, next_retry_at) VALUES ($1, $2, $3, now()) RETURNING id`,
		[]any{sweepID, from, to}, func(row pgx.Row) error { return row.Scan(&windowID) },
	); err != nil {
		t.Fatalf("insert window: %v", err)
	}
	return sweepID, windowID
}

// claimWindow claims one specific window, as ClaimDiscoveryWindows would.
func claimWindow(t *testing.T, pool *db.Pool, id int64, owner string) DiscoveryWindow {
	t.Helper()
	win := DiscoveryWindow{LeaseOwner: owner}
	if err := pool.QueryRow(context.Background(),
		`UPDATE discovery_windows w SET status = 'running', lease_owner = $2, lease_expires_at = now() + interval '1 minute'
		 FROM discovery_sweeps s WHERE w.id = $1 AND s.id = w.sweep_id
		 RETURNING w.id, w.sweep_id, w.pushed_from, w.pushed_to, w.cursor, w.pages_done, w.attempt_count, s.min_stars`,
		[]any{id, owner},
		func(row pgx.Row) error {
			return row.Scan(&win.ID, &win.SweepID, &win.From, &win.To, &win.Cursor, &win.PagesDone, &win.AttemptCount, &win.MinStars)
		},
	); err != nil {
		t.Fatalf("claim window %d: %v", id, err)
	}
	return win
}

type searchNode struct {
	ID       int64
	Name     string
	Stars    int
	Language string
	Issues   int
	Pushed   time.Time
	IsFork   bool
}

// searchResponse builds a GraphQL search response.
func searchResponse(t *testing.T, count int, hasNext bool, nodes ...searchNode) string {
	t.Helper()
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		var lang any
		if n.Language != "" {
			lang = map[string]any{"name": n.Language}
		}
		out = append(out, map[string]any{
			"databaseId": n.ID, "nameWithOwner": n.Name, "url": "https://github.com/" + n.Name,
			"description": "d", "primaryLanguage": lang,
			"languages":        map[string]any{"edges": []any{map[string]any{"size": 10, "node": map[string]any{"name": "Go"}}}},
			"repositoryTopics": map[string]any{"nodes": []any{map[string]any{"topic": map[string]any{"name": "cli"}}}},
			"licenseInfo":      map[string]any{"spdxId": "MIT"},
			"stargazerCount":   n.Stars, "forkCount": 3, "watchers": map[string]any{"totalCount": 4},
			"isFork": n.IsFork, "isArchived": false, "forkingAllowed": true,
			"issues": map[string]any{"totalCount": n.Issues}, "openIssues": map[string]any{"totalCount": 2},
			"createdAt": "2020-01-01T00:00:00Z", "pushedAt": n.Pushed.UTC().Format(time.RFC3339),
		})
	}
	body, err := json.Marshal(map[string]any{"data": map[string]any{
		"search": map[string]any{
			"repositoryCount": count,
			"pageInfo":        map[string]any{"hasNextPage": hasNext, "endCursor": "c1"},
			"nodes":           out,
		},
		"rateLimit": map[string]any{"limit": 5000, "remaining": 4900, "used": 100, "resetAt": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "cost": 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

type windowRow struct {
	Status    string
	Split     bool
	Truncated bool
	Cursor    *string
	ReposSeen int
}

func readWindow(t *testing.T, pool *db.Pool, id int64) windowRow {
	t.Helper()
	var w windowRow
	if err := pool.QueryRow(context.Background(),
		`SELECT status, split, truncated, cursor, repos_seen FROM discovery_windows WHERE id = $1`, []any{id},
		func(row pgx.Row) error { return row.Scan(&w.Status, &w.Split, &w.Truncated, &w.Cursor, &w.ReposSeen) },
	); err != nil {
		t.Fatalf("read window: %v", err)
	}
	return w
}

type repoRow struct {
	ID       int
	Name     string
	GitHubID *int64
	Eligible bool
	Reason   *string
	Stars    *int
}

// readRepo returns the repository row named name, or nil.
func readRepo(t *testing.T, pool *db.Pool, name string) *repoRow {
	t.Helper()
	var r repoRow
	err := pool.QueryRow(context.Background(),
		`SELECT id, full_name, github_database_id, is_eligible, eligibility_reason, stars
		 FROM repositories WHERE lower(full_name) = lower($1)`, []any{name},
		func(row pgx.Row) error {
			return row.Scan(&r.ID, &r.Name, &r.GitHubID, &r.Eligible, &r.Reason, &r.Stars)
		})
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}
	if err != nil {
		t.Fatalf("read repository %s: %v", name, err)
	}
	return &r
}

// cleanupRepos deletes repositories by name when the test ends.
func cleanupRepos(t *testing.T, pool *db.Pool, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, n := range names {
			if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE lower(full_name) = lower($1)`, n); err != nil {
				t.Errorf("cleanup %s: %v", n, err)
			}
		}
	})
}

func insertRepo(t *testing.T, pool *db.Pool, name string, githubID *int64, reason *string) int {
	t.Helper()
	var id int
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO repositories (full_name, url, github_database_id, eligibility_reason, is_eligible)
		 VALUES ($1, $2, $3, $4::text, $4::text IS NULL) RETURNING id`,
		[]any{name, "https://github.com/" + name, githubID, reason},
		func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert repository %s: %v", name, err)
	}
	cleanupRepos(t, pool, name)
	return id
}

func discoveryWorker(t *testing.T, pool *db.Pool, response string) *Worker {
	t.Helper()
	gh, _ := fakeCrawlAPI(t, response)
	snap := settings.Defaults()
	return NewWorker(pool, gh, func() string { return "test-token" }, func() *settings.Snapshot { return snap })
}

// uniqueID returns a GitHub id no other test row uses.
func uniqueID() int64 { return 9_000_000_000 + time.Now().UnixNano()%1_000_000_000 }

func ptr[T any](v T) *T { return &v }

func TestDiscoverySplitsADenseWindow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sweepID, winID := testSweep(t, pool, from, from.Add(24*time.Hour))

	w := discoveryWorker(t, pool, searchResponse(t, 3400, true))
	if err := w.ProcessDiscoveryWindow(ctx, claimWindow(t, pool, winID, "discovery-test")); err != nil {
		t.Fatalf("process: %v", err)
	}
	if got := readWindow(t, pool, winID); got.Status != "done" || !got.Split {
		t.Fatalf("dense window = %+v, want done and split", got)
	}
	var halves []string
	if err := pool.QueryRows(ctx,
		`SELECT pushed_from || '..' || pushed_to FROM discovery_windows
		 WHERE sweep_id = $1 AND id <> $2 AND status = 'pending' ORDER BY pushed_from`,
		[]any{sweepID, winID}, func(rows pgx.Rows) error {
			var s string
			if err := rows.Scan(&s); err != nil {
				return err
			}
			halves = append(halves, s)
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	if len(halves) != 2 {
		t.Fatalf("got %d pending halves (%v), want 2", len(halves), halves)
	}
}

func TestDiscoveryTruncatesAtTheMinimumWindow(t *testing.T) {
	pool := testPool(t)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	_, winID := testSweep(t, pool, from, from.Add(time.Minute))
	w := discoveryWorker(t, pool, searchResponse(t, 1500, true))
	if err := w.ProcessDiscoveryWindow(context.Background(), claimWindow(t, pool, winID, "discovery-test")); err != nil {
		t.Fatalf("process: %v", err)
	}
	got := readWindow(t, pool, winID)
	if got.Split || !got.Truncated || got.Status != "pending" || got.Cursor == nil || *got.Cursor != "c1" {
		t.Fatalf("minimum window = %+v, want pending at cursor c1, truncated, not split", got)
	}
}

func TestDiscoveryStoresOnlyEligibleNewRepositories(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	eligible := fmt.Sprintf("discovery-test/new-%d", stamp)
	lowStars := fmt.Sprintf("discovery-test/low-%d", stamp)
	fork := fmt.Sprintf("discovery-test/fork-%d", stamp)
	excluded := fmt.Sprintf("discovery-test/excluded-%d", stamp)
	cleanupRepos(t, pool, eligible, lowStars, fork, excluded)
	if err := pool.Exec(ctx, `INSERT INTO excluded_repositories (full_name, reason) VALUES ($1, 'test')`, excluded); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM excluded_repositories WHERE full_name = $1`, excluded); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	now := time.Now()
	from := now.Add(-time.Hour)
	_, winID := testSweep(t, pool, from, now)
	id := uniqueID()
	w := discoveryWorker(t, pool, searchResponse(t, 4, false,
		searchNode{ID: id, Name: eligible, Stars: 100, Language: "Go", Issues: 5, Pushed: now},
		searchNode{ID: id + 1, Name: lowStars, Stars: 3, Language: "Go", Issues: 5, Pushed: now},
		searchNode{ID: id + 2, Name: fork, Stars: 100, Language: "Go", Issues: 5, Pushed: now, IsFork: true},
		searchNode{ID: id + 3, Name: excluded, Stars: 100, Language: "Go", Issues: 5, Pushed: now},
	))
	if err := w.ProcessDiscoveryWindow(ctx, claimWindow(t, pool, winID, "discovery-test")); err != nil {
		t.Fatalf("process: %v", err)
	}

	r := readRepo(t, pool, eligible)
	if r == nil || !r.Eligible || r.GitHubID == nil || *r.GitHubID != id || r.Stars == nil || *r.Stars != 100 {
		t.Fatalf("eligible repository = %+v, want stored eligible with github id %d", r, id)
	}
	for _, n := range []string{lowStars, fork, excluded} {
		if r := readRepo(t, pool, n); r != nil {
			t.Errorf("%s was stored: %+v", n, r)
		}
	}
	if got := readWindow(t, pool, winID); got.Status != "done" || got.ReposSeen != 4 {
		t.Fatalf("window = %+v, want done with 4 repositories seen", got)
	}
}

func TestDiscoveryIdentity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	now := time.Now()
	node := func(id int64, name string) searchNode {
		return searchNode{ID: id, Name: name, Stars: 100, Language: "Go", Issues: 5, Pushed: now}
	}
	run := func(t *testing.T, nodes ...searchNode) {
		t.Helper()
		_, winID := testSweep(t, pool, now.Add(-time.Hour), now)
		w := discoveryWorker(t, pool, searchResponse(t, len(nodes), false, nodes...))
		if err := w.ProcessDiscoveryWindow(ctx, claimWindow(t, pool, winID, "discovery-test")); err != nil {
			t.Fatalf("process: %v", err)
		}
	}

	t.Run("known name gets its id", func(t *testing.T) {
		name := fmt.Sprintf("discovery-test/known-%d", stamp)
		insertRepo(t, pool, name, nil, nil)
		id := uniqueID()
		run(t, node(id, name))
		if r := readRepo(t, pool, name); r.GitHubID == nil || *r.GitHubID != id {
			t.Fatalf("row = %+v, want github id %d", r, id)
		}
	})

	t.Run("rename follows the id", func(t *testing.T) {
		oldName := fmt.Sprintf("discovery-test/old-%d", stamp)
		newName := fmt.Sprintf("discovery-test/renamed-%d", stamp)
		id := uniqueID()
		rowID := insertRepo(t, pool, oldName, &id, nil)
		cleanupRepos(t, pool, newName)
		run(t, node(id, newName))
		r := readRepo(t, pool, newName)
		if r == nil || r.ID != rowID {
			t.Fatalf("renamed row = %+v, want row %d under the new name", r, rowID)
		}
	})

	t.Run("id-less twin under the new name is marked duplicate", func(t *testing.T) {
		oldName := fmt.Sprintf("discovery-test/todo-%d", stamp)
		newName := fmt.Sprintf("discovery-test/tasks-%d", stamp)
		id := uniqueID()
		keeperID := insertRepo(t, pool, oldName, &id, nil)
		twinID := insertRepo(t, pool, newName, nil, nil)
		run(t, node(id, newName))
		keeper, twin := readRepo(t, pool, oldName), readRepo(t, pool, newName)
		if keeper == nil || keeper.ID != keeperID || !keeper.Eligible {
			t.Fatalf("row with the id = %+v, want unchanged under its old name", keeper)
		}
		if twin == nil || twin.ID != twinID || twin.Eligible || twin.Reason == nil || *twin.Reason != "duplicate" {
			t.Fatalf("twin = %+v, want ineligible 'duplicate'", twin)
		}
		// RecomputeEligibility must not undo it.
		if _, err := RecomputeEligibility(ctx, pool, 0, 3650); err != nil {
			t.Fatal(err)
		}
		if twin := readRepo(t, pool, newName); twin.Reason == nil || *twin.Reason != "duplicate" {
			t.Fatalf("after recompute twin = %+v, want still 'duplicate'", twin)
		}
	})

	t.Run("not_found is cleared", func(t *testing.T) {
		name := fmt.Sprintf("discovery-test/back-%d", stamp)
		insertRepo(t, pool, name, nil, ptr("not_found"))
		run(t, node(uniqueID(), name))
		if r := readRepo(t, pool, name); r.Reason != nil {
			t.Fatalf("row = %+v, want not_found cleared for the next recheck", r)
		}
	})

	t.Run("crawl marks an id-less twin duplicate", func(t *testing.T) {
		keeper := fmt.Sprintf("discovery-test/crawl-keeper-%d", stamp)
		twin := fmt.Sprintf("discovery-test/crawl-twin-%d", stamp)
		id := uniqueID()
		insertRepo(t, pool, keeper, &id, nil)
		twinID := insertRepo(t, pool, twin, nil, nil)
		var conflict string
		if err := pool.InTx(ctx, func(tx pgx.Tx) error {
			var err error
			conflict, err = RecordRepositoryIDTx(ctx, tx, twinID, id)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if r := readRepo(t, pool, twin); conflict != ConflictDuplicate || r.Reason == nil || *r.Reason != "duplicate" {
			t.Fatalf("conflict %q, twin %+v; want duplicate", conflict, r)
		}
	})
}

// A worker whose lease was reclaimed writes nothing: no repository, and the
// window keeps the new owner's state.
func TestDiscoveryLostLeaseWritesNothing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now()
	name := fmt.Sprintf("discovery-test/lease-%d", now.UnixNano())
	cleanupRepos(t, pool, name)
	_, winID := testSweep(t, pool, now.Add(-time.Hour), now)
	win := claimWindow(t, pool, winID, "old-owner")
	claimWindow(t, pool, winID, "new-owner")

	w := discoveryWorker(t, pool, searchResponse(t, 1, false,
		searchNode{ID: uniqueID(), Name: name, Stars: 100, Language: "Go", Issues: 5, Pushed: now}))
	if err := w.ProcessDiscoveryWindow(ctx, win); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("process with a lost lease: err = %v, want ErrLeaseLost", err)
	}
	if r := readRepo(t, pool, name); r != nil {
		t.Fatalf("repository written under a lost lease: %+v", r)
	}
	if got := readWindow(t, pool, winID); got.Status != "running" {
		t.Fatalf("window = %+v, want still running under the new owner", got)
	}
}

func TestCompleteSweeps(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now()
	status := func(id int64) string {
		var s string
		if err := pool.QueryRow(ctx, `SELECT status FROM discovery_sweeps WHERE id = $1`, []any{id},
			func(row pgx.Row) error { return row.Scan(&s) }); err != nil {
			t.Fatal(err)
		}
		return s
	}

	sweepID, winID := testSweep(t, pool, now.Add(-time.Hour), now)
	if _, err := CompleteSweeps(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if s := status(sweepID); s != "running" {
		t.Fatalf("sweep with a pending window = %s, want running", s)
	}

	if err := pool.Exec(ctx, `UPDATE discovery_windows SET status = 'failed' WHERE id = $1`, winID); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteSweeps(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if s := status(sweepID); s != "failed" {
		t.Fatalf("sweep with a failed window = %s, want failed", s)
	}

	sweepID, winID = testSweep(t, pool, now.Add(-time.Hour), now)
	if err := pool.Exec(ctx, `UPDATE discovery_windows SET status = 'done' WHERE id = $1`, winID); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteSweeps(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if s := status(sweepID); s != "done" {
		t.Fatalf("sweep with every window done = %s, want done", s)
	}
}

func TestStartSweep(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := pool.Exec(ctx, `UPDATE discovery_sweeps SET status = 'failed' WHERE status = 'running'`); err != nil {
		t.Fatal(err)
	}
	p := SweepParams{MinStars: 25, PushedWithinDays: 3, Window: 24 * time.Hour}
	id, kind, err := StartSweep(ctx, pool, SweepFull, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM discovery_sweeps WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if kind != SweepFull {
		t.Fatalf("kind = %s, want full", kind)
	}
	var windows int
	var covered time.Duration
	if err := pool.QueryRow(ctx,
		`SELECT count(*), coalesce(extract(epoch FROM sum(pushed_to - pushed_from)), 0)::bigint
		 FROM discovery_windows WHERE sweep_id = $1`, []any{id},
		func(row pgx.Row) error {
			var secs int64
			if err := row.Scan(&windows, &secs); err != nil {
				return err
			}
			covered = time.Duration(secs) * time.Second
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	if windows != 3 || covered != 72*time.Hour {
		t.Fatalf("got %d windows covering %s, want 3 covering 72h", windows, covered)
	}
	if _, _, err := StartSweep(ctx, pool, SweepIncremental, p); !errors.Is(err, ErrSweepRunning) {
		t.Fatalf("second sweep while one runs: err = %v, want ErrSweepRunning", err)
	}
}

// A sweep that ended failed is reopened once ingest.discovery.failed_retry_seconds
// has passed: only its failed windows go back to pending, with their attempts
// reset, and never while another sweep runs.
func TestRetryFailedSweep(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now()
	read := func(q string, id int64) (status string, attempts int) {
		t.Helper()
		if err := pool.QueryRow(ctx, q, []any{id}, func(row pgx.Row) error { return row.Scan(&status, &attempts) }); err != nil {
			t.Fatal(err)
		}
		return status, attempts
	}

	sweepID, failedWin := testSweep(t, pool, now.Add(-2*time.Hour), now.Add(-time.Hour))
	var doneWin int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO discovery_windows (sweep_id, pushed_from, pushed_to, status, next_retry_at)
		 VALUES ($1, $2, $3, 'done', now()) RETURNING id`,
		[]any{sweepID, now.Add(-time.Hour), now}, func(row pgx.Row) error { return row.Scan(&doneWin) },
	); err != nil {
		t.Fatal(err)
	}
	if err := pool.Exec(ctx,
		`UPDATE discovery_windows SET status = 'failed', attempt_count = 3, last_error_class = 'fetch_error' WHERE id = $1`, failedWin); err != nil {
		t.Fatal(err)
	}
	if err := pool.Exec(ctx,
		`UPDATE discovery_sweeps SET status = 'failed', completed_at = now() - interval '2 hours' WHERE id = $1`, sweepID); err != nil {
		t.Fatal(err)
	}

	// Not yet: it failed two hours ago and the retry waits three.
	if id, err := RetryFailedSweep(ctx, pool, 3*time.Hour); err != nil || id != 0 {
		t.Fatalf("before the retry interval: id = %d, err = %v; want 0, nil", id, err)
	}

	id, err := RetryFailedSweep(ctx, pool, time.Hour)
	if err != nil || id != sweepID {
		t.Fatalf("after the retry interval: id = %d, err = %v; want %d", id, err, sweepID)
	}
	if s, _ := read(`SELECT status, 0 FROM discovery_sweeps WHERE id = $1`, sweepID); s != "running" {
		t.Fatalf("sweep = %s, want running", s)
	}
	if s, n := read(`SELECT status, attempt_count FROM discovery_windows WHERE id = $1`, failedWin); s != "pending" || n != 0 {
		t.Fatalf("failed window = %s after %d attempts, want pending after 0", s, n)
	}
	if s, _ := read(`SELECT status, attempt_count FROM discovery_windows WHERE id = $1`, doneWin); s != "done" {
		t.Fatalf("done window = %s, want left done", s)
	}

	// It is running again, so a second call finds nothing to reopen.
	if id, err := RetryFailedSweep(ctx, pool, time.Hour); err != nil || id != 0 {
		t.Fatalf("while it runs: id = %d, err = %v; want 0, nil", id, err)
	}
}
