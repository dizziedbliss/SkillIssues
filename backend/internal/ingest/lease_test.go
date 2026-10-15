package ingest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// testPool connects to TEST_DATABASE_URL (a migrated database). Without it,
// or with -short, the test is skipped.
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

// A worker whose lease expired and was reclaimed must not be able to write
// the job's outcome over the new owner's: the write is rejected with
// ErrLeaseLost and changes nothing.
func TestFencedWritesRejectAReclaimedLease(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// 'discovery' jobs are never claimed by a running coordinator, so this
	// test cannot race a live ingestd on the same database.
	const jobType = JobType("discovery")
	name := fmt.Sprintf("lease-test/%d", time.Now().UnixNano())
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
	if err := pool.Exec(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at) VALUES ($1, $2, 'pending', now())`,
		repoID, string(jobType),
	); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	claim := func(owner string) Job {
		t.Helper()
		jobs, err := ClaimJobs(ctx, pool, jobType, 1000, owner, time.Minute)
		if err != nil {
			t.Fatalf("claim as %s: %v", owner, err)
		}
		for _, j := range jobs {
			if j.RepositoryID == repoID {
				return j
			}
		}
		t.Fatalf("claim as %s did not return the test job", owner)
		return Job{}
	}

	stale := claim("owner-a")
	if stale.LeaseOwner != "owner-a" {
		t.Fatalf("LeaseOwner = %q, want owner-a", stale.LeaseOwner)
	}

	// Owner A stalls past its lease; the job is reclaimed and claimed by B.
	if err := pool.Exec(ctx, `UPDATE ingest_jobs SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, stale.ID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	if _, err := ReclaimExpiredLeases(ctx, pool); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	current := claim("owner-b")

	// Every one of A's late writes is rejected.
	if err := FailJob(ctx, pool, stale, "fetch_error", "late", time.Second, 3); !errors.Is(err, ErrLeaseLost) {
		t.Errorf("FailJob with a reclaimed lease: err = %v, want ErrLeaseLost", err)
	}
	if err := ReleaseJob(ctx, pool, stale, time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Errorf("ReleaseJob with a reclaimed lease: err = %v, want ErrLeaseLost", err)
	}
	if err := MarkUnavailable(ctx, pool, stale, "late"); !errors.Is(err, ErrLeaseLost) {
		t.Errorf("MarkUnavailable with a reclaimed lease: err = %v, want ErrLeaseLost", err)
	}
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		return CompleteOrRequeue(ctx, tx, stale, PageProgress{OpenCursor: "stale-cursor", OpenDone: true, ClosedDone: true, PagesDone: 1}, 0)
	})
	if !errors.Is(err, ErrLeaseLost) {
		t.Errorf("CompleteOrRequeue with a reclaimed lease: err = %v, want ErrLeaseLost", err)
	}

	var status, owner string
	var eligible *bool
	if err := pool.QueryRow(ctx,
		`SELECT j.status, COALESCE(j.lease_owner, ''), r.is_eligible
		 FROM ingest_jobs j JOIN repositories r ON r.id = j.repository_id WHERE j.id = $1`,
		[]any{current.ID},
		func(row pgx.Row) error { return row.Scan(&status, &owner, &eligible) },
	); err != nil {
		t.Fatalf("read job: %v", err)
	}
	if status != "running" || owner != "owner-b" {
		t.Errorf("after stale writes: status=%s owner=%s, want running owner-b", status, owner)
	}
	if eligible != nil && !*eligible {
		t.Error("MarkUnavailable with a lost lease still flagged the repository ineligible")
	}

	// The current owner's write goes through.
	if err := FailJob(ctx, pool, current, "fetch_error", "current", time.Second, 3); err != nil {
		t.Errorf("FailJob by the current owner: %v", err)
	}
}
