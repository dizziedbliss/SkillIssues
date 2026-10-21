package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

// testRepo inserts a repository with open issues #1 and #2, removed (with its
// issues and jobs) when the test ends.
func testRepo(t *testing.T, pool *db.Pool) (int, string) {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("freshness-test/%d", time.Now().UnixNano())
	var repoID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url) VALUES ($1, $2) RETURNING id`,
		[]any{name, "https://github.com/" + name},
		func(row pgx.Row) error { return row.Scan(&repoID) },
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{1, 2} {
		node := github.GraphQLIssueNode{Number: n, Title: "t", URL: fmt.Sprintf("u/%d", n), State: "OPEN", CreatedAt: created, UpdatedAt: created}
		if err := pool.InTx(ctx, func(tx pgx.Tx) error { return UpsertIssueTx(ctx, tx, repoID, node, "Go") }); err != nil {
			t.Fatalf("insert issue #%d: %v", n, err)
		}
	}
	return repoID, name
}

// fakeCrawlAPI answers every GraphQL request with response and records the
// variables of the last one.
func fakeCrawlAPI(t *testing.T, response string) (*github.Client, *map[string]any) {
	t.Helper()
	var vars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		vars = req.Variables
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return github.NewClient(srv.URL, func() time.Duration { return 10 * time.Second }), &vars
}

// runOneJob queues a single job for repoID and processes it with the worker.
// 'discovery' jobs are never claimed by a running coordinator, so this cannot
// race a live ingestd on the same database.
func runOneJob(t *testing.T, pool *db.Pool, gh *github.Client, repoID int, since, passStarted *time.Time) int {
	t.Helper()
	ctx := context.Background()
	var jobID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at, since, pass_started_at)
		 VALUES ($1, 'discovery', 'pending', now(), $2, $3) RETURNING id`,
		[]any{repoID, since, passStarted},
		func(row pgx.Row) error { return row.Scan(&jobID) },
	); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	jobs, err := ClaimJobs(ctx, pool, JobType("discovery"), 1000, "freshness-test", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	snap := settings.Defaults()
	w := NewWorker(pool, gh, func() string { return "test-token" }, func() *settings.Snapshot { return snap })
	for _, j := range jobs {
		if j.ID == jobID {
			if err := w.ProcessJob(ctx, j); err != nil {
				t.Fatalf("process job: %v", err)
			}
			return jobID
		}
	}
	t.Fatal("claim did not return the test job")
	return 0
}

type issueRow struct {
	State     string
	UpdatedAt time.Time
}

func readIssues(t *testing.T, pool *db.Pool, repoID int) map[int]issueRow {
	t.Helper()
	out := map[int]issueRow{}
	if err := pool.QueryRows(context.Background(),
		`SELECT number, state, updated_at FROM issues WHERE repository_id = $1`, []any{repoID},
		func(rows pgx.Rows) error {
			var n int
			var r issueRow
			if err := rows.Scan(&n, &r.State, &r.UpdatedAt); err != nil {
				return err
			}
			out[n] = r
			return nil
		},
	); err != nil {
		t.Fatalf("read issues: %v", err)
	}
	return out
}

func readPass(t *testing.T, pool *db.Pool, repoID, jobID int) (watermark *time.Time, status string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `SELECT issues_synced_through FROM repositories WHERE id = $1`, []any{repoID},
		func(row pgx.Row) error { return row.Scan(&watermark) }); err != nil {
		t.Fatalf("read watermark: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM ingest_jobs WHERE id = $1`, []any{jobID},
		func(row pgx.Row) error { return row.Scan(&status) }); err != nil {
		t.Fatalf("read job: %v", err)
	}
	return watermark, status
}

const fakeRateLimit = `"rateLimit":{"limit":5000,"remaining":4900,"used":100,"resetAt":"2099-01-01T00:00:00Z","cost":1}`

// An incremental pass upserts the open issues GitHub reports changed, marks
// the ones it reports closed CLOSED (with GitHub's time), leaves the rest
// alone, and on completion moves the watermark to the pass's start.
func TestIncrementalPassClosesIssuesAndAdvancesWatermark(t *testing.T) {
	pool := testPool(t)
	repoID, _ := testRepo(t, pool)
	gh, vars := fakeCrawlAPI(t, `{"data":{"repository":{
		"issues":{"nodes":[{"databaseId":3,"number":3,"title":"new","url":"u/3","state":"OPEN","createdAt":"2026-10-15T00:00:00Z","updatedAt":"2026-10-15T00:00:00Z","author":{"login":"a"},"comments":{"totalCount":0},"labels":{"nodes":[]}}],"pageInfo":{"hasNextPage":false,"endCursor":"o1"}},
		"closed":{"nodes":[{"number":1,"updatedAt":"2026-10-14T12:00:00Z"},{"number":99,"updatedAt":"2026-10-14T13:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}},
		"stargazerCount":1},`+fakeRateLimit+`}}`)

	since := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	passStarted := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)
	jobID := runOneJob(t, pool, gh, repoID, &since, &passStarted)

	if (*vars)["withClosed"] != true || (*vars)["since"] != "2026-10-13T00:00:00Z" {
		t.Errorf("request variables = %v, want withClosed and since", *vars)
	}
	issues := readIssues(t, pool, repoID)
	if got := issues[1]; got.State != "CLOSED" || !got.UpdatedAt.Equal(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("issue #1 = %+v, want CLOSED at GitHub's updatedAt", got)
	}
	if issues[2].State != "OPEN" {
		t.Errorf("issue #2 = %+v, want untouched OPEN", issues[2])
	}
	if issues[3].State != "OPEN" {
		t.Errorf("issue #3 = %+v, want inserted OPEN", issues[3])
	}
	if _, ok := issues[99]; ok {
		t.Error("closed issue #99, never stored, was inserted")
	}
	watermark, status := readPass(t, pool, repoID, jobID)
	if status != "done" || watermark == nil || !watermark.Equal(passStarted) {
		t.Errorf("job %s, watermark %v; want done, %v", status, watermark, passStarted)
	}
}

// A full crawl (no watermark yet) never asks for closed issues, and its
// completion sets the first watermark.
func TestFullCrawlSetsFirstWatermark(t *testing.T) {
	pool := testPool(t)
	repoID, _ := testRepo(t, pool)
	gh, vars := fakeCrawlAPI(t, `{"data":{"repository":{
		"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}},
		"stargazerCount":1},`+fakeRateLimit+`}}`)

	passStarted := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)
	jobID := runOneJob(t, pool, gh, repoID, nil, &passStarted)

	if (*vars)["withClosed"] != false {
		t.Errorf("full crawl asked for closed issues: %v", *vars)
	}
	if issues := readIssues(t, pool, repoID); issues[1].State != "OPEN" || issues[2].State != "OPEN" {
		t.Errorf("full crawl changed stored issues: %+v", issues)
	}
	watermark, status := readPass(t, pool, repoID, jobID)
	if status != "done" || watermark == nil || !watermark.Equal(passStarted) {
		t.Errorf("job %s, watermark %v; want done, %v", status, watermark, passStarted)
	}
}

// A saved issue goes due once stale; GitHub reporting it closed, or no longer
// having it, marks it CLOSED.
func TestTrackedIssuesAreCheckedAndClosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := testRepo(t, pool)

	var userID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("tracked-test-%d", time.Now().UnixNano())},
		func(row pgx.Row) error { return row.Scan(&userID) },
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	if err := pool.Exec(ctx,
		`INSERT INTO saved_issues (user_id, issue_id) SELECT $1, id FROM issues WHERE repository_id = $2`,
		userID, repoID,
	); err != nil {
		t.Fatalf("save issues: %v", err)
	}

	all, err := DueTrackedIssues(ctx, pool, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var due []TrackedIssue
	for _, d := range all {
		if d.Number == 1 || d.Number == 2 {
			var r int
			if err := pool.QueryRow(ctx, `SELECT repository_id FROM issues WHERE id = $1`, []any{d.ID},
				func(row pgx.Row) error { return row.Scan(&r) }); err != nil {
				t.Fatal(err)
			}
			if r == repoID {
				due = append(due, d)
			}
		}
	}
	if len(due) != 2 {
		t.Fatalf("due tracked issues for the test repo = %d, want 2", len(due))
	}
	if stale, err := DueTrackedIssues(ctx, pool, time.Hour, 1000); err != nil {
		t.Fatal(err)
	} else {
		for _, d := range stale {
			if d.ID == due[0].ID || d.ID == due[1].ID {
				t.Errorf("issue synced just now is due under a 1h interval: %+v", d)
			}
		}
	}

	closedAt := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	states := []github.TrackedIssueState{{Found: true, State: "CLOSED", UpdatedAt: closedAt}, {Found: false}}
	closed, err := ApplyTrackedStates(ctx, pool, due, states)
	if err != nil {
		t.Fatal(err)
	}
	if closed != 2 {
		t.Errorf("closed = %d, want 2", closed)
	}
	issues := readIssues(t, pool, repoID)
	if got := issues[due[0].Number]; got.State != "CLOSED" || !got.UpdatedAt.Equal(closedAt) {
		t.Errorf("found-closed issue = %+v", got)
	}
	if got := issues[due[1].Number]; got.State != "CLOSED" || got.UpdatedAt.Equal(closedAt) {
		t.Errorf("not-found issue = %+v, want CLOSED with its old updated_at", got)
	}
}

// When open issues run out before closed ones, the pass keeps paging the
// closed connection alone, from its own cursor, and completes only then.
func TestPassPagesClosedConnectionAfterOpenIsExhausted(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := testRepo(t, pool)

	responses := []string{
		`{"data":{"repository":{
			"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}},
			"closed":{"nodes":[{"number":1,"updatedAt":"2026-10-14T12:00:00Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}},
			"stargazerCount":1},` + fakeRateLimit + `}}`,
		`{"data":{"repository":{
			"closed":{"nodes":[{"number":2,"updatedAt":"2026-10-14T13:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":"c2"}},
			"stargazerCount":1},` + fakeRateLimit + `}}`,
	}
	var seen []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		seen = append(seen, req.Variables)
		if len(seen) > len(responses) {
			t.Errorf("unexpected request %d", len(seen))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(responses[len(seen)-1])); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	gh := github.NewClient(srv.URL, func() time.Duration { return 10 * time.Second })

	since := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	passStarted := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)
	var jobID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at, since, pass_started_at)
		 VALUES ($1, 'discovery', 'pending', now(), $2, $3) RETURNING id`,
		[]any{repoID, since, passStarted},
		func(row pgx.Row) error { return row.Scan(&jobID) },
	); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	snap := settings.Defaults()
	w := NewWorker(pool, gh, func() string { return "test-token" }, func() *settings.Snapshot { return snap })
	for page := 1; page <= 2; page++ {
		jobs, err := ClaimJobs(ctx, pool, JobType("discovery"), 1000, "freshness-test", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		found := false
		for _, j := range jobs {
			if j.ID != jobID {
				continue
			}
			found = true
			// Treat it as a deep job so a page with more to fetch is
			// requeued in place, not handed to a new 'deep' row that a
			// live coordinator could claim.
			j.JobType = JobDeep
			if err := w.ProcessJob(ctx, j); err != nil {
				t.Fatalf("page %d: %v", page, err)
			}
		}
		if !found {
			t.Fatalf("page %d: test job not claimable", page)
		}
		if page == 1 {
			if wm, status := readPass(t, pool, repoID, jobID); status != "pending" || wm != nil {
				t.Fatalf("after page 1: job %s, watermark %v; want pending, nil", status, wm)
			}
		}
	}

	if len(seen) != 2 {
		t.Fatalf("requests = %d, want 2", len(seen))
	}
	if seen[1]["withOpen"] != false || seen[1]["withClosed"] != true || seen[1]["closedAfter"] != "c1" {
		t.Errorf("page 2 variables = %v, want closed only, after c1", seen[1])
	}
	issues := readIssues(t, pool, repoID)
	if issues[1].State != "CLOSED" || issues[2].State != "CLOSED" {
		t.Errorf("issues = %+v, want both CLOSED", issues)
	}
	if wm, status := readPass(t, pool, repoID, jobID); status != "done" || wm == nil || !wm.Equal(passStarted) {
		t.Errorf("job %s, watermark %v; want done, %v", status, wm, passStarted)
	}
}

// The first page of a pass writes the repository's live stats over the
// dataset snapshot; later pages do not ask for them.
func TestFirstPageRefreshesRepositoryStats(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := testRepo(t, pool)
	gh, vars := fakeCrawlAPI(t, `{"data":{"repository":{
		"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}},
		"stargazerCount":315,"forkCount":12,"watchers":{"totalCount":7},"openIssues":{"totalCount":42},
		"isArchived":true,"pushedAt":"2026-10-10T10:00:00Z",
		"contributingGuidelines":{"url":"https://github.com/o/n/blob/main/CONTRIBUTING.md"}},`+fakeRateLimit+`}}`)

	passStarted := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)
	runOneJob(t, pool, gh, repoID, nil, &passStarted)

	if (*vars)["withStats"] != true {
		t.Errorf("first page did not request stats: %v", *vars)
	}
	var stars, forks, watchers, open int
	var archived bool
	var pushed, refreshed *time.Time
	var contributing *string
	if err := pool.QueryRow(ctx,
		`SELECT stars, forks, watchers, open_issue_count, is_archived, pushed_at, metadata_refreshed_at, contributing_url FROM repositories WHERE id = $1`,
		[]any{repoID},
		func(row pgx.Row) error {
			return row.Scan(&stars, &forks, &watchers, &open, &archived, &pushed, &refreshed, &contributing)
		},
	); err != nil {
		t.Fatal(err)
	}
	if stars != 315 || forks != 12 || watchers != 7 || open != 42 || !archived ||
		pushed == nil || !pushed.Equal(time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)) || refreshed == nil {
		t.Errorf("stats = stars %d forks %d watchers %d open %d archived %v pushed %v refreshed %v",
			stars, forks, watchers, open, archived, pushed, refreshed)
	}
	if contributing == nil || *contributing != "https://github.com/o/n/blob/main/CONTRIBUTING.md" {
		t.Errorf("contributing_url = %v", contributing)
	}
}

// Closed issues past retention are deleted unless a user saved them; open and
// recently closed ones stay.
func TestPruneClosedIssuesKeepsWhatUsersHold(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := testRepo(t, pool)

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{3, 4} {
		node := github.GraphQLIssueNode{Number: n, Title: "t", URL: fmt.Sprintf("u/%d", n), State: "OPEN", CreatedAt: created, UpdatedAt: created}
		if err := pool.InTx(ctx, func(tx pgx.Tx) error { return UpsertIssueTx(ctx, tx, repoID, node, "Go") }); err != nil {
			t.Fatalf("insert issue #%d: %v", n, err)
		}
	}
	// #1 open; #2 closed recently; #3 closed long ago (prune); #4 closed long
	// ago but saved.
	if err := pool.Exec(ctx,
		`UPDATE issues SET state = 'CLOSED',
		    updated_at = CASE number WHEN 2 THEN now() - interval '1 day' ELSE now() - interval '30 days' END
		 WHERE repository_id = $1 AND number IN (2, 3, 4)`, repoID,
	); err != nil {
		t.Fatal(err)
	}
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("prune-test-%d", time.Now().UnixNano())},
		func(row pgx.Row) error { return row.Scan(&userID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	if err := pool.Exec(ctx,
		`INSERT INTO saved_issues (user_id, issue_id) SELECT $1, id FROM issues WHERE repository_id = $2 AND number = 4`,
		userID, repoID); err != nil {
		t.Fatal(err)
	}

	// Batch of 100 over the whole test database: other rows may be pruned
	// too, so only the test repository's survivors are asserted.
	for {
		n, err := PruneClosedIssues(ctx, pool, 7*24*time.Hour, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n < 100 {
			break
		}
	}
	issues := readIssues(t, pool, repoID)
	for _, n := range []int{1, 2, 4} {
		if _, ok := issues[n]; !ok {
			t.Errorf("issue #%d was pruned", n)
		}
	}
	if _, ok := issues[3]; ok {
		t.Error("issue #3, closed 30 days ago and unheld, survived")
	}
}
