package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func setExcluded(t *testing.T, repoName string) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	if err := pool.Exec(ctx,
		`INSERT INTO excluded_repositories (full_name, reason) VALUES ($1, 'test')`, repoName); err != nil {
		t.Fatalf("exclude: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(),
			`DELETE FROM excluded_repositories WHERE lower(full_name) = lower($1)`, repoName); err != nil {
			t.Errorf("cleanup exclusion: %v", err)
		}
	})
}

func repoState(t *testing.T, repoID int) (eligible bool, reason string) {
	t.Helper()
	pool := testPool(t)
	if err := pool.QueryRow(context.Background(),
		`SELECT is_eligible, coalesce(eligibility_reason, '') FROM repositories WHERE id = $1`, []any{repoID},
		func(row pgx.Row) error { return row.Scan(&eligible, &reason) }); err != nil {
		t.Fatal(err)
	}
	return eligible, reason
}

// makeHealthy gives a test repository everything the eligibility rule asks
// for, so a later exclusion is the only thing that can fail it.
func makeHealthy(t *testing.T, repoID int) {
	t.Helper()
	if err := testPool(t).Exec(context.Background(),
		`UPDATE repositories SET language = 'Go', stars = 1000000, pushed_at = now(), is_archived = false,
		        issue_count = 10, is_eligible = true, eligibility_reason = '' WHERE id = $1`, repoID); err != nil {
		t.Fatal(err)
	}
}

// A listed repository goes ineligible however healthy it is, matched
// case-insensitively, and comes back when the entry is removed.
func TestRecomputeEligibilityHonoursExclusions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, name := testRepo(t, pool)
	makeHealthy(t, repoID)

	if _, err := RecomputeEligibility(ctx, pool, 5, 3650); err != nil {
		t.Fatal(err)
	}
	if ok, reason := repoState(t, repoID); !ok {
		t.Fatalf("healthy repository ineligible before exclusion: %q", reason)
	}

	setExcluded(t, upper(name))
	if _, err := RecomputeEligibility(ctx, pool, 5, 3650); err != nil {
		t.Fatal(err)
	}
	if ok, reason := repoState(t, repoID); ok || reason != "excluded" {
		t.Fatalf("after exclusion: eligible=%v reason=%q, want false/excluded", ok, reason)
	}

	if err := pool.Exec(ctx, `DELETE FROM excluded_repositories WHERE lower(full_name) = lower($1)`, name); err != nil {
		t.Fatal(err)
	}
	if _, err := RecomputeEligibility(ctx, pool, 5, 3650); err != nil {
		t.Fatal(err)
	}
	if ok, reason := repoState(t, repoID); !ok {
		t.Fatalf("after un-excluding: still ineligible: %q", reason)
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// A job queued before its repository became ineligible is not claimed, and is
// claimed again once the repository is eligible.
func TestClaimJobsSkipsIneligibleRepositories(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repoID, _ := testRepo(t, pool)
	makeHealthy(t, repoID)

	var jobID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at)
		 VALUES ($1, 'discovery', 'pending', now()) RETURNING id`, []any{repoID},
		func(row pgx.Row) error { return row.Scan(&jobID) }); err != nil {
		t.Fatal(err)
	}
	claimed := func() bool {
		jobs, err := ClaimJobs(ctx, pool, JobType("discovery"), 1000, "claim-guard-test", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range jobs {
			if j.ID == jobID {
				return true
			}
		}
		return false
	}

	if err := pool.Exec(ctx, `UPDATE repositories SET is_eligible = false, eligibility_reason = 'excluded' WHERE id = $1`, repoID); err != nil {
		t.Fatal(err)
	}
	if claimed() {
		t.Fatal("job of an ineligible repository was claimed")
	}
	if err := pool.Exec(ctx, `UPDATE repositories SET is_eligible = true, eligibility_reason = '' WHERE id = $1`, repoID); err != nil {
		t.Fatal(err)
	}
	if !claimed() {
		t.Fatal("job was not claimed once the repository was eligible again")
	}
}

// Excluded and archived repositories lose their stored issues, except ones a
// user saved; a repository that is only below the star floor keeps them.
func TestPruneExcludedIssues(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	excludedRepo, _ := testRepo(t, pool)
	archivedRepo, _ := testRepo(t, pool)
	lowStarsRepo, _ := testRepo(t, pool)
	reasons := map[int]string{excludedRepo: "excluded", archivedRepo: "stale;archived", lowStarsRepo: "below_star_floor"}
	for id, reason := range reasons {
		if err := pool.Exec(ctx, `UPDATE repositories SET is_eligible = false, eligibility_reason = $2 WHERE id = $1`, id, reason); err != nil {
			t.Fatal(err)
		}
	}

	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("prune-excluded-%d", time.Now().UnixNano())},
		func(row pgx.Row) error { return row.Scan(&userID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	// Issue #2 of the excluded repository is saved.
	if err := pool.Exec(ctx,
		`INSERT INTO saved_issues (user_id, issue_id) SELECT $1, id FROM issues WHERE repository_id = $2 AND number = 2`,
		userID, excludedRepo); err != nil {
		t.Fatal(err)
	}

	for {
		n, err := PruneExcludedIssues(ctx, pool, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n < 100 {
			break
		}
	}

	if got := readIssues(t, pool, excludedRepo); len(got) != 1 || got[2].State == "" {
		t.Errorf("excluded repository issues = %v, want only the saved #2", got)
	}
	if got := readIssues(t, pool, archivedRepo); len(got) != 0 {
		t.Errorf("archived repository kept %d issues", len(got))
	}
	if got := readIssues(t, pool, lowStarsRepo); len(got) != 2 {
		t.Errorf("below-star-floor repository has %d issues, want 2 kept", len(got))
	}
}
