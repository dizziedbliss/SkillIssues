package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

func TestArchiveFileURL(t *testing.T) {
	hour := time.Date(2026, 10, 18, 5, 0, 0, 0, time.FixedZone("IST", 5*3600+1800))
	// 05:00 IST is 23:30 UTC the day before; the file is named in UTC, with
	// the hour not zero-padded.
	got := archiveFileURL("https://data.gharchive.org/", hour.Truncate(time.Hour))
	if got != "https://data.gharchive.org/2026-10-17-23.json.gz" {
		t.Errorf("url = %q", got)
	}
}

// archiveLine is one synthetic archive event, shaped as GH Archive writes them.
func archiveLine(t *testing.T, typ string, repoID int64, repoName string, issue map[string]any) string {
	t.Helper()
	ev := map[string]any{
		"id":   "1",
		"type": typ,
		"repo": map[string]any{"id": repoID, "name": repoName, "url": "u"},
		"payload": map[string]any{
			"action": "labeled",
			"issue":  issue,
		},
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func archiveIssueJSON(id int64, number int, state string, updated time.Time, labels ...string) map[string]any {
	ls := []map[string]any{}
	for _, l := range labels {
		ls = append(ls, map[string]any{"id": 1, "name": l, "color": "fff"})
	}
	return map[string]any{
		"id": id, "number": number, "title": fmt.Sprintf("issue %d", number),
		"html_url": fmt.Sprintf("https://github.com/o/r/issues/%d", number),
		"state":    state, "created_at": "2026-01-01T00:00:00Z", "updated_at": updated.Format(time.RFC3339),
		"comments": 2, "user": map[string]any{"login": "author", "id": 9}, "labels": ls,
		"body": "attacker-controlled text the reader never stores",
	}
}

func gzipLines(t *testing.T, lines ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The reader keeps the newest state of each issue, from issue and issue
// comment events alike, and nothing else: no pull requests, no other event
// types.
func TestReadArchiveHour(t *testing.T) {
	t0 := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	pr := archiveIssueJSON(30, 3, "open", t0.Add(5*time.Minute))
	pr["pull_request"] = map[string]any{"url": "u"}
	file := gzipLines(t,
		`{"id":"0","type":"PushEvent","repo":{"id":1,"name":"o/r"},"payload":{"commits":[]}}`,
		archiveLine(t, "IssuesEvent", 1, "o/r", archiveIssueJSON(10, 1, "open", t0.Add(10*time.Minute), "bug")),
		// An older state of the same issue, later in the file: ignored.
		archiveLine(t, "IssuesEvent", 1, "o/r", archiveIssueJSON(10, 1, "open", t0.Add(1*time.Minute))),
		archiveLine(t, "IssueCommentEvent", 1, "o/r", archiveIssueJSON(10, 1, "closed", t0.Add(20*time.Minute), "bug", "good first issue")),
		archiveLine(t, "IssueCommentEvent", 1, "o/r", pr),
		archiveLine(t, "IssuesEvent", 2, "other/repo", archiveIssueJSON(20, 1, "open", t0.Add(3*time.Minute))),
	)

	issues, events, err := readArchiveHour(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if events != 6 {
		t.Errorf("events = %d, want 6", events)
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %d, want 2 (one per repository; the PR is not an issue): %+v", len(issues), issues)
	}
	got := issues[archiveIssueKey{1, 1}]
	if got.node.State != "CLOSED" || !got.node.UpdatedAt.Equal(t0.Add(20*time.Minute)) ||
		len(got.node.Labels) != 2 || got.node.DatabaseID != 10 || got.node.AuthorLogin != "author" ||
		got.node.URL != "https://github.com/o/r/issues/1" || got.repoName != "o/r" {
		t.Errorf("o/r#1 = %+v, want the newest (closed, two labels) state", got)
	}
	if _, ok := issues[archiveIssueKey{2, 1}]; !ok {
		t.Error("other/repo#1 missing")
	}
}

func TestReadArchiveHourRejectsGarbage(t *testing.T) {
	if _, _, err := readArchiveHour(strings.NewReader("not gzip")); err == nil {
		t.Error("want an error for a file that is not gzip")
	}
	if _, _, err := readArchiveHour(bytes.NewReader(gzipLines(t, `{"type":`))); err == nil {
		t.Error("want an error for a truncated event")
	}
}

// archiveFixture is a stored repository with known issues, and a fake GH
// Archive serving files the test sets.
type archiveFixture struct {
	pool     *db.Pool
	worker   *Worker
	repoID   int
	ghID     int64
	name     string
	files    map[string][]byte
	hourBase time.Time
	queued   []time.Time // every hour the test queued, for cleanup
}

func newArchiveFixture(t *testing.T) *archiveFixture {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	f := &archiveFixture{
		pool:  pool,
		ghID:  stamp,
		name:  fmt.Sprintf("archive-test/r%d", stamp),
		files: map[string][]byte{},
		// Hours far in the past, so the test's rows never meet real ones.
		hourBase: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(stamp%100000) * time.Hour),
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url, github_database_id, language) VALUES ($1, 'u', $2, 'Go') RETURNING id`,
		[]any{f.name, f.ghID}, func(row pgx.Row) error { return row.Scan(&f.repoID) }); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		for _, sql := range []string{
			`DELETE FROM repositories WHERE id = $1`,
		} {
			if err := pool.Exec(bg, sql, f.repoID); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		if err := pool.Exec(bg, `DELETE FROM archive_hours WHERE hour = ANY($1)`, f.queued); err != nil {
			t.Errorf("cleanup hours: %v", err)
		}
		if err := pool.Exec(bg, `DELETE FROM repo_candidates WHERE full_name LIKE 'archive-test/%'`); err != nil {
			t.Errorf("cleanup candidates: %v", err)
		}
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(body); err != nil {
			t.Errorf("serve archive: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	snap := settings.Defaults()
	f.worker = NewWorker(pool, nil, func() string { return "" }, func() *settings.Snapshot { return snap })
	f.worker.archive = archiveSource{
		baseURL: srv.URL,
		client:  httpx.New(func() time.Duration { return 10 * time.Second }, nil),
	}
	return f
}

func (f *archiveFixture) hour(i int) time.Time { return f.hourBase.Add(time.Duration(i) * time.Hour) }

func (f *archiveFixture) serve(i int, file []byte) {
	f.files[archiveFileURL("", f.hour(i))] = file
}

// claim queues hour i and leases it, as the coordinator would.
func (f *archiveFixture) claim(t *testing.T, i int) ArchiveHour {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.Exec(ctx, `INSERT INTO archive_hours (hour) VALUES ($1)`, f.hour(i)); err != nil {
		t.Fatalf("queue hour: %v", err)
	}
	f.queued = append(f.queued, f.hour(i))
	owner := fmt.Sprintf("archive-test-%d", time.Now().UnixNano())
	var h ArchiveHour
	if err := f.pool.QueryRow(ctx,
		`UPDATE archive_hours SET status = 'running', lease_owner = $2, lease_expires_at = now() + interval '1 minute'
		 WHERE hour = $1 RETURNING hour, attempts, lease_owner`,
		[]any{f.hour(i), owner}, func(row pgx.Row) error { return row.Scan(&h.Hour, &h.Attempts, &h.LeaseOwner) }); err != nil {
		t.Fatalf("claim hour: %v", err)
	}
	return h
}

func (f *archiveFixture) hourStatus(t *testing.T, i int) string {
	t.Helper()
	var st string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM archive_hours WHERE hour = $1`,
		[]any{f.hour(i)}, func(row pgx.Row) error { return row.Scan(&st) }); err != nil {
		t.Fatalf("read hour status: %v", err)
	}
	return st
}

func (f *archiveFixture) insertIssue(t *testing.T, number int, state string, updated time.Time) {
	t.Helper()
	if err := f.pool.Exec(context.Background(),
		`INSERT INTO issues (repository_id, number, title, url, difficulty, state, updated_at)
		 VALUES ($1, $2, 'stored title', 'u', 'UNCLASSIFIED', $3, $4)`,
		f.repoID, number, state, updated); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
}

type storedIssue struct {
	title, state, difficulty string
	updated                  time.Time
}

func (f *archiveFixture) issue(t *testing.T, number int) (storedIssue, bool) {
	t.Helper()
	var s storedIssue
	err := f.pool.QueryRow(context.Background(),
		`SELECT title, state, difficulty, updated_at FROM issues WHERE repository_id = $1 AND number = $2`,
		[]any{f.repoID, number}, func(row pgx.Row) error { return row.Scan(&s.title, &s.state, &s.difficulty, &s.updated) })
	if err == db.ErrNotFound {
		return s, false
	}
	if err != nil {
		t.Fatalf("read issue: %v", err)
	}
	return s, true
}

// One hour, applied: newer states win, older ones do not, a closed issue
// nobody stored is not inserted, and a repository that is not stored becomes a
// candidate.
func TestProcessArchiveHourApplies(t *testing.T) {
	f := newArchiveFixture(t)
	ctx := context.Background()
	t0 := f.hour(0)

	f.insertIssue(t, 1, "OPEN", t0.Add(-time.Hour))   // the archive has a newer state
	f.insertIssue(t, 2, "OPEN", t0.Add(time.Hour))    // stored state is newer than the event
	f.insertIssue(t, 3, "OPEN", t0.Add(-2*time.Hour)) // closed in this hour

	f.serve(0, gzipLines(t,
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1001, 1, "open", t0.Add(10*time.Minute), "good first issue")),
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1002, 2, "closed", t0.Add(10*time.Minute))),
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1003, 3, "closed", t0.Add(30*time.Minute))),
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1004, 4, "open", t0.Add(40*time.Minute))),
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1005, 5, "closed", t0.Add(40*time.Minute))),
		archiveLine(t, "IssuesEvent", f.ghID+1, "archive-test/unknown", archiveIssueJSON(2001, 1, "open", t0.Add(5*time.Minute))),
	))

	h := f.claim(t, 0)
	if err := f.worker.ProcessArchiveHour(ctx, h, false); err != nil {
		t.Fatal(err)
	}
	if st := f.hourStatus(t, 0); st != "done" {
		t.Errorf("hour status = %s, want done", st)
	}

	if got, _ := f.issue(t, 1); got.title != "issue 1" || got.difficulty != "BEGINNER" || !got.updated.Equal(t0.Add(10*time.Minute)) {
		t.Errorf("#1 = %+v, want the archive's newer, labelled state", got)
	}
	if got, _ := f.issue(t, 2); got.state != "OPEN" || got.title != "stored title" {
		t.Errorf("#2 = %+v, want the stored, newer state untouched", got)
	}
	if got, _ := f.issue(t, 3); got.state != "CLOSED" {
		t.Errorf("#3 = %+v, want CLOSED", got)
	}
	if got, ok := f.issue(t, 4); !ok || got.state != "OPEN" {
		t.Errorf("#4 = %+v (stored %v), want a new open issue", got, ok)
	}
	if _, ok := f.issue(t, 5); ok {
		t.Error("#5, closed and never stored, was inserted")
	}

	var name string
	if err := f.pool.QueryRow(ctx, `SELECT full_name FROM repo_candidates WHERE github_database_id = $1`,
		[]any{f.ghID + 1}, func(row pgx.Row) error { return row.Scan(&name) }); err != nil {
		t.Fatalf("candidate not stored: %v", err)
	}

	// The same hour again changes nothing: every state is now not newer.
	if err := f.pool.Exec(ctx, `DELETE FROM archive_hours WHERE hour = $1`, f.hour(0)); err != nil {
		t.Fatal(err)
	}
	f.queued = f.queued[:len(f.queued)-1]
	h = f.claim(t, 0)
	if err := f.worker.ProcessArchiveHour(ctx, h, false); err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := f.pool.QueryRow(ctx, `SELECT issues_applied FROM archive_hours WHERE hour = $1`,
		[]any{f.hour(0)}, func(row pgx.Row) error { return row.Scan(&applied) }); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Errorf("re-reading the hour applied %d issues, want 0", applied)
	}
}

// With the disk guard on, stored issues are updated and none inserted.
func TestProcessArchiveHourUpdateOnly(t *testing.T) {
	f := newArchiveFixture(t)
	t0 := f.hour(1)
	f.insertIssue(t, 1, "OPEN", t0.Add(-time.Hour))
	f.serve(1, gzipLines(t,
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1001, 1, "open", t0.Add(time.Minute))),
		archiveLine(t, "IssuesEvent", f.ghID, f.name, archiveIssueJSON(1002, 2, "open", t0.Add(time.Minute))),
	))
	if err := f.worker.ProcessArchiveHour(context.Background(), f.claim(t, 1), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.issue(t, 1); got.title != "issue 1" {
		t.Errorf("#1 = %+v, want updated", got)
	}
	if _, ok := f.issue(t, 2); ok {
		t.Error("#2 was inserted while the disk guard was on")
	}
}

// An hour not published yet goes back to pending; one never published after
// ingest.archive.missing_after_hours is missing.
func TestProcessArchiveHourNotPublished(t *testing.T) {
	f := newArchiveFixture(t)
	ctx := context.Background()

	// hour(2) is decades old: past any missing_after_hours.
	if err := f.worker.ProcessArchiveHour(ctx, f.claim(t, 2), false); err != nil {
		t.Fatal(err)
	}
	if st := f.hourStatus(t, 2); st != "missing" {
		t.Errorf("old unpublished hour = %s, want missing", st)
	}

	// An hour that just ended is retried, not given up on.
	recent := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	f.hourBase = recent.Add(-3 * time.Hour)
	if err := f.worker.ProcessArchiveHour(ctx, f.claim(t, 3), false); err != nil {
		t.Fatal(err)
	}
	if st := f.hourStatus(t, 3); st != "pending" {
		t.Errorf("recent unpublished hour = %s, want pending", st)
	}
}

// A file that is not an archive fails its hour, and counts an attempt.
func TestProcessArchiveHourBadFile(t *testing.T) {
	f := newArchiveFixture(t)
	f.serve(4, []byte("<html>not an archive</html>"))
	if err := f.worker.ProcessArchiveHour(context.Background(), f.claim(t, 4), false); err == nil {
		t.Fatal("want an error")
	}
	var st string
	var attempts int
	if err := f.pool.QueryRow(context.Background(), `SELECT status, attempts FROM archive_hours WHERE hour = $1`,
		[]any{f.hour(4)}, func(row pgx.Row) error { return row.Scan(&st, &attempts) }); err != nil {
		t.Fatal(err)
	}
	if st != "pending" || attempts != 1 {
		t.Errorf("after one bad file: status %s, attempts %d; want pending, 1", st, attempts)
	}
}

// A worker whose lease was taken over cannot record the hour.
func TestArchiveHourWritesAreFenced(t *testing.T) {
	f := newArchiveFixture(t)
	h := f.claim(t, 5)
	h.LeaseOwner = "someone-else"
	if err := completeArchiveHour(context.Background(), f.pool, h, archiveHourResult{}); err != ErrLeaseLost {
		t.Errorf("complete with a lost lease = %v, want ErrLeaseLost", err)
	}
	if st := f.hourStatus(t, 5); st != "running" {
		t.Errorf("status = %s, want running (untouched)", st)
	}
}

// emptyArchiveHours skips a test that reads archive_hours as a whole unless the
// table is empty, and empties it again afterwards.
func emptyArchiveHours(t *testing.T, pool *db.Pool) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM archive_hours`, nil,
		func(row pgx.Row) error { return row.Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		t.Skip("archive_hours is not empty; this test owns the whole table")
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM archive_hours`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
}

// The first seed goes back no further than the backfill limit and stops at the
// last hour that has ended; the next adds only what ended since.
func TestSeedArchiveHours(t *testing.T) {
	pool := testPool(t)
	emptyArchiveHours(t, pool)
	ctx := context.Background()

	n, err := SeedArchiveHours(ctx, pool, 5*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Somewhere between 1 and 5 hours (the oldest pass may be more recent
	// than the limit), never more.
	if n < 1 || n > 5 {
		t.Fatalf("seeded %d hours, want 1 to 5", n)
	}
	var last time.Time
	if err := pool.QueryRow(ctx, `SELECT max(hour) FROM archive_hours`, nil,
		func(row pgx.Row) error { return row.Scan(&last) }); err != nil {
		t.Fatal(err)
	}
	if want := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour); !last.Equal(want) {
		t.Errorf("last seeded hour = %s, want %s", last, want)
	}
	if n, err := SeedArchiveHours(ctx, pool, 5*time.Hour); err != nil || n != 0 {
		t.Errorf("second seed added %d (err %v), want 0", n, err)
	}
}

// Coverage ends at the first hour that is not done or missing.
func TestArchiveCoveredUntil(t *testing.T) {
	pool := testPool(t)
	emptyArchiveHours(t, pool)
	ctx := context.Background()

	if _, ok, err := ArchiveCoveredUntil(ctx, pool); err != nil || ok {
		t.Fatalf("empty table: ok=%v err=%v, want not ok", ok, err)
	}
	base := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	for i, st := range []string{"done", "missing", "done", "failed", "done", "pending"} {
		if err := pool.Exec(ctx, `INSERT INTO archive_hours (hour, status) VALUES ($1, $2)`,
			base.Add(time.Duration(i)*time.Hour), st); err != nil {
			t.Fatal(err)
		}
	}
	until, ok, err := ArchiveCoveredUntil(ctx, pool)
	if err != nil || !ok || !until.Equal(base.Add(3*time.Hour)) {
		t.Errorf("covered until %s (ok %v, err %v), want the failed hour %s", until, ok, err, base.Add(3*time.Hour))
	}

	if err := pool.Exec(ctx, `UPDATE archive_hours SET status = 'done'`); err != nil {
		t.Fatal(err)
	}
	until, _, _ = ArchiveCoveredUntil(ctx, pool)
	if !until.Equal(base.Add(6 * time.Hour)) {
		t.Errorf("all done: covered until %s, want the end of the last hour", until)
	}
}

// A candidate is due when never looked up, or looked up long ago and active
// since; never once it is stored.
func TestDueRepoCandidates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	t.Cleanup(func() {
		bg := context.Background()
		if err := pool.Exec(bg, `DELETE FROM repo_candidates WHERE full_name LIKE 'due-test/%'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if err := pool.Exec(bg, `DELETE FROM repositories WHERE full_name LIKE 'due-test/%'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	insert := func(id int64, name string, checkedAgo *time.Duration, seenAgo time.Duration) {
		t.Helper()
		var checked any
		if checkedAgo != nil {
			checked = time.Now().Add(-*checkedAgo)
		}
		if err := pool.Exec(ctx,
			`INSERT INTO repo_candidates (github_database_id, full_name, first_seen_hour, last_seen_hour, checked_at)
			 VALUES ($1, $2, $3, $3, $4)`, id, name, time.Now().Add(-seenAgo), checked); err != nil {
			t.Fatal(err)
		}
	}
	day := 24 * time.Hour
	long, short := 40*day, 2*day
	insert(stamp+1, "due-test/never", nil, time.Hour)
	insert(stamp+2, "due-test/old-active", &long, time.Hour)
	insert(stamp+3, "due-test/old-quiet", &long, 41*day)
	insert(stamp+4, "due-test/recent", &short, time.Hour)
	insert(stamp+5, "due-test/stored", nil, time.Hour)
	if err := pool.Exec(ctx, `INSERT INTO repositories (full_name, url, github_database_id) VALUES ('due-test/stored', 'u', $1)`, stamp+5); err != nil {
		t.Fatal(err)
	}

	due, err := DueRepoCandidates(ctx, pool, 1000, 30*day)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range due {
		if strings.HasPrefix(c.FullName, "due-test/") {
			got[c.FullName] = true
		}
	}
	if !got["due-test/never"] || !got["due-test/old-active"] || len(got) != 2 {
		t.Errorf("due = %v, want never and old-active", got)
	}

	// Only old-quiet can never be due again; pruning removes it alone.
	if _, err := PruneRepoCandidates(ctx, pool, 30*day); err != nil {
		t.Fatal(err)
	}
	left := map[string]bool{}
	err = pool.QueryRows(ctx, `SELECT full_name FROM repo_candidates WHERE full_name LIKE 'due-test/%'`, nil,
		func(rows pgx.Rows) error {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			left[name] = true
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if left["due-test/old-quiet"] || len(left) != 4 {
		t.Errorf("after pruning: %v, want all but old-quiet", left)
	}
}

// Resolving looks candidates up and stores the eligible ones as the Discovery
// Sweeper would, recording every candidate's outcome so it is not looked up
// again until it is due.
func TestResolveRepoCandidates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	snap := settings.Defaults()
	if ok, err := CanProceed(ctx, pool, LaneCrawler, installationIdentity,
		snap.Int(settings.IngestDiscoveryRateLimitFloor), snap.Int(settings.IngestInteractiveReservePercent)); err != nil || !ok {
		t.Skipf("the installation budget is below discovery's floor here (err %v)", err)
	}
	stamp := time.Now().UnixNano()
	good := fmt.Sprintf("resolve-test/good-%d", stamp)
	small := fmt.Sprintf("resolve-test/small-%d", stamp)
	gone := fmt.Sprintf("resolve-test/gone-%d", stamp)
	t.Cleanup(func() {
		bg := context.Background()
		if err := pool.Exec(bg, `DELETE FROM repo_candidates WHERE full_name LIKE 'resolve-test/%'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if err := pool.Exec(bg, `DELETE FROM repositories WHERE full_name LIKE 'resolve-test/%'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	// The three are the oldest-unchecked candidates only if nothing else is
	// pending; make them the whole due set by checking everything else.
	var others int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM repo_candidates WHERE checked_at IS NULL`, nil,
		func(row pgx.Row) error { return row.Scan(&others) }); err != nil {
		t.Fatal(err)
	}
	if others > 0 {
		t.Skip("other unchecked candidates exist here")
	}
	for i, name := range []string{good, small, gone} {
		if err := pool.Exec(ctx,
			`INSERT INTO repo_candidates (github_database_id, full_name, first_seen_hour, last_seen_hour) VALUES ($1, $2, now(), now())`,
			stamp+int64(i), name); err != nil {
			t.Fatal(err)
		}
	}

	pushed := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	repo := func(id int64, name string, stars int) string {
		return fmt.Sprintf(`{"databaseId":%d,"nameWithOwner":%q,"url":"u","primaryLanguage":{"name":"Go"},
			"stargazerCount":%d,"forkCount":1,"watchers":{"totalCount":1},"isFork":false,"isArchived":false,
			"issues":{"totalCount":50},"openIssues":{"totalCount":10},"pushedAt":%q}`, id, name, stars, pushed)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		// Answer by the names asked, in whatever alias order they came.
		var parts []string
		for i := 0; ; i++ {
			o, ok := req.Variables[fmt.Sprintf("o%d", i)]
			if !ok {
				break
			}
			full := fmt.Sprintf("%s/%s", o, req.Variables[fmt.Sprintf("n%d", i)])
			switch full {
			case good:
				parts = append(parts, fmt.Sprintf(`"r%d":%s`, i, repo(stamp, good, 5000)))
			case small:
				parts = append(parts, fmt.Sprintf(`"r%d":%s`, i, repo(stamp+1, small, 1)))
			default:
				parts = append(parts, fmt.Sprintf(`"r%d":null`, i))
			}
		}
		parts = append(parts, `"rateLimit":{"limit":12500,"remaining":12000,"used":1,"resetAt":"2001-01-01T00:00:00Z","cost":1}`)
		fmt.Fprintf(w, `{"data":{%s},"errors":[{"type":"NOT_FOUND","message":"gone"}]}`, strings.Join(parts, ","))
	}))
	t.Cleanup(srv.Close)

	w := NewWorker(pool, github.NewClient(srv.URL, func() time.Duration { return 10 * time.Second }),
		func() string { return "test-token" }, func() *settings.Snapshot { return snap })
	n, err := w.ResolveRepoCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("looked up %d, want 3", n)
	}

	outcomes := map[string]string{}
	err = pool.QueryRows(ctx, `SELECT full_name, outcome FROM repo_candidates WHERE full_name LIKE 'resolve-test/%' AND checked_at IS NOT NULL`, nil,
		func(rows pgx.Rows) error {
			var name, outcome string
			if err := rows.Scan(&name, &outcome); err != nil {
				return err
			}
			outcomes[name] = outcome
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if outcomes[good] != discoveredNew || outcomes[small] != discoveredIneligible || outcomes[gone] != "not_found" {
		t.Errorf("outcomes = %v", outcomes)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM repositories WHERE full_name LIKE 'resolve-test/%'`, nil,
		func(row pgx.Row) error { return row.Scan(&stored) }); err != nil {
		t.Fatal(err)
	}
	if stored != 1 {
		t.Errorf("stored %d repositories, want only the eligible one", stored)
	}
	// The budget row this wrote belongs to the real installation; its reset
	// time is long past, so it constrains nothing.
}
