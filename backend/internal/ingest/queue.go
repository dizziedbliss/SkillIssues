package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// JobType distinguishes the two pagination roles. Initial and Freshness
// share the same handling — both fetch exactly one page — so they share
// the "initial" job type; a freshness re-check is just a new "initial" job
// enqueued for a repository that already completed one.
type JobType string

const (
	JobInitial JobType = "initial"
	JobDeep    JobType = "deep"
)

// Job is one claimed row from ingest_jobs.
type Job struct {
	ID            int
	RepositoryID  int
	FullName      string
	Language      *string
	JobType       JobType
	Cursor        *string
	PagesDone     int
	AttemptCount  int
	LastSyncSince *time.Time

	// LeaseOwner is the coordinator that claimed this job. Every write that
	// finishes the job is fenced on it: once the lease has expired and been
	// reclaimed, a late write from the old owner changes nothing and returns
	// ErrLeaseLost instead of overwriting the new owner's cursor.
	LeaseOwner string
}

// ErrLeaseLost means the job's lease expired and was reclaimed (or the job
// was otherwise finished) before this worker wrote its result. The write was
// not applied; within a transaction the caller must roll back.
var ErrLeaseLost = errors.New("ingest: job lease lost")

// fenceJob is the WHERE clause every job-finishing write shares: the job is
// still running under the lease this worker was given.
const fenceJob = `id = $1 AND status = 'running' AND lease_owner = $2`

// execFenced runs a fenced job update and turns "no row matched" into
// ErrLeaseLost.
func execFenced(ctx context.Context, exec func(context.Context, string, ...any) (pgconn.CommandTag, error), sql string, args ...any) error {
	tag, err := exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	return nil
}

// ClaimJobs atomically claims up to limit pending-or-due jobs of the given
// type via SELECT ... FOR UPDATE SKIP LOCKED, so two workers (in this
// process or another) can never claim the same job. FIFO on next_retry_at —
// no priority score (plan §7, D1/D9).
//
// lease bounds how long the job may sit queued and run before
// ReclaimExpiredLeases may hand it to someone else (ingest.lease_seconds).
func ClaimJobs(ctx context.Context, pool *db.Pool, jobType JobType, limit int, owner string, lease time.Duration) ([]Job, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := pool.Raw().Query(ctx,
		`UPDATE ingest_jobs j
		 SET status = 'running', lease_owner = $1, lease_expires_at = now() + ($2 * interval '1 second')
		 FROM (
			SELECT id FROM ingest_jobs
			WHERE job_type = $3
			  AND status = 'pending'
			  AND (next_retry_at IS NULL OR next_retry_at <= now())
			ORDER BY next_retry_at NULLS FIRST, id
			LIMIT $4
			FOR UPDATE SKIP LOCKED
		 ) claimed
		 WHERE j.id = claimed.id
		 RETURNING j.id, j.repository_id, j.cursor, j.pages_done, j.attempt_count, j.job_type`,
		owner, lease.Seconds(), string(jobType), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ingest: claim jobs: %w", err)
	}
	defer rows.Close()

	var ids []int
	byID := map[int]*Job{}
	for rows.Next() {
		j := &Job{LeaseOwner: owner}
		var jt string
		if err := rows.Scan(&j.ID, &j.RepositoryID, &j.Cursor, &j.PagesDone, &j.AttemptCount, &jt); err != nil {
			return nil, fmt.Errorf("ingest: scan claimed job: %w", err)
		}
		j.JobType = JobType(jt)
		ids = append(ids, j.ID)
		byID[j.ID] = j
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Fill in repository full_name/language (not worth a JOIN above given
	// SKIP LOCKED already does the hard part).
	err = pool.QueryRows(ctx,
		`SELECT id, full_name, language FROM repositories WHERE id = ANY($1)`,
		[]any{repoIDs(byID)},
		func(rows pgx.Rows) error {
			var id int
			var fullName string
			var lang *string
			if err := rows.Scan(&id, &fullName, &lang); err != nil {
				return err
			}
			for _, j := range byID {
				if j.RepositoryID == id {
					j.FullName = fullName
					j.Language = lang
				}
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ingest: load repository details for claimed jobs: %w", err)
	}

	jobs := make([]Job, 0, len(ids))
	for _, id := range ids {
		jobs = append(jobs, *byID[id])
	}
	return jobs, nil
}

func repoIDs(byID map[int]*Job) []int {
	ids := make([]int, 0, len(byID))
	for _, j := range byID {
		ids = append(ids, j.RepositoryID)
	}
	return ids
}

// CompleteOrRequeue finishes a page within an already-open transaction:
// caller is responsible for upserting the page's issues in the same tx
// first, so the cursor only ever advances alongside committed rows (plan
// §7 "atomic progress" / "never advance a cursor before its rows are
// committed").
//
// If hasNextPage, the job (or a new "deep" job, if this was "initial")
// goes back to pending with the new cursor. Otherwise it's marked done.
//
// maxPages is the disk guardrail (ingest.max_pages_per_repo, D6): once
// pagesDone reaches it, the job is marked done regardless of what GitHub's
// hasNextPage actually says, capping how much of the disk one
// mega-repository's issue tracker can consume. 0 means unlimited.
func CompleteOrRequeue(ctx context.Context, tx pgx.Tx, job Job, newCursor string, hasNextPage bool, pagesDone int, maxPages int) error {
	if maxPages > 0 && pagesDone >= maxPages {
		hasNextPage = false
	}
	// "Last successful sync time, separate from the cursor" (ingestion design
	// §5). Written in the same transaction as the page's rows, so it can
	// never claim a sync that was rolled back — and it is what
	// EnqueueFreshnessJobs orders the cycle by.
	if _, err := tx.Exec(ctx,
		`UPDATE repositories SET last_synced_at = now() WHERE id = $1`,
		job.RepositoryID,
	); err != nil {
		return fmt.Errorf("ingest: mark repository synced: %w", err)
	}

	if !hasNextPage {
		return execFenced(ctx, tx.Exec,
			`UPDATE ingest_jobs SET status = 'done', cursor = $3, pages_done = $4,
			    has_next_page = false, lease_owner = NULL, lease_expires_at = NULL,
			    next_retry_at = NULL, updated_at = now()
			 WHERE `+fenceJob,
			job.ID, job.LeaseOwner, newCursor, pagesDone,
		)
	}

	if job.JobType == JobDeep {
		return execFenced(ctx, tx.Exec,
			`UPDATE ingest_jobs SET status = 'pending', cursor = $3, pages_done = $4,
			    has_next_page = true, lease_owner = NULL, lease_expires_at = NULL,
			    next_retry_at = now(), attempt_count = 0, updated_at = now()
			 WHERE `+fenceJob,
			job.ID, job.LeaseOwner, newCursor, pagesDone,
		)
	}

	// An "initial" job's role ends after its one page: mark it done and
	// hand off pagination to a new "deep" job, per the Initial/Freshness
	// vs. Deep Diver split.
	if err := execFenced(ctx, tx.Exec,
		`UPDATE ingest_jobs SET status = 'done', cursor = $3, pages_done = $4,
		    has_next_page = true, lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = NULL, updated_at = now()
		 WHERE `+fenceJob,
		job.ID, job.LeaseOwner, newCursor, pagesDone,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, cursor, pages_done, has_next_page, next_retry_at)
		 VALUES ($1, 'deep', 'pending', $2, $3, true, now())
		 ON CONFLICT (repository_id) WHERE status IN ('pending', 'running') DO NOTHING`,
		job.RepositoryID, newCursor, pagesDone,
	)
	return err
}

// ReleaseJob returns a claimed job to pending without counting it as a
// failed attempt — used when the worker never actually tried GitHub (e.g.
// the shared budget was exhausted before the call was made).
func ReleaseJob(ctx context.Context, pool *db.Pool, job Job, retryAfter time.Duration) error {
	return execFenced(ctx, pool.Raw().Exec,
		`UPDATE ingest_jobs SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = now() + ($3 * interval '1 second'), updated_at = now()
		 WHERE `+fenceJob,
		job.ID, job.LeaseOwner, retryAfter.Seconds(),
	)
}

// FailJob records an error and schedules a retry with exponential backoff,
// or — once attempt_count exceeds maxAttempts — leaves the job in 'failed'
// so a persistently expensive repository defers itself instead of
// blocking unrelated work (plan §7 "poison isolation").
func FailJob(ctx context.Context, pool *db.Pool, job Job, errClass, errDetail string, backoffBase time.Duration, maxAttempts int) error {
	attempt := job.AttemptCount + 1
	if attempt >= maxAttempts {
		return execFenced(ctx, pool.Raw().Exec,
			`UPDATE ingest_jobs SET status = 'failed', attempt_count = $3,
			    last_error_class = $4, last_error_detail = $5,
			    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
			 WHERE `+fenceJob,
			job.ID, job.LeaseOwner, attempt, errClass, errDetail,
		)
	}
	backoff := backoffBase * time.Duration(1<<uint(attempt))
	return execFenced(ctx, pool.Raw().Exec,
		`UPDATE ingest_jobs SET status = 'pending', attempt_count = $3,
		    last_error_class = $4, last_error_detail = $5,
		    lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = now() + ($6 * interval '1 second'), updated_at = now()
		 WHERE `+fenceJob,
		job.ID, job.LeaseOwner, attempt, errClass, errDetail, backoff.Seconds(),
	)
}

// MarkUnavailable isolates a poisoned repository (e.g. GitHub returns
// NOT_FOUND) permanently rather than retrying it forever, and flags the
// repository itself so other code stops treating it as ingestible.
//
// Both updates commit together: a job marked failed always has its
// repository flagged, and a lost lease changes neither.
func MarkUnavailable(ctx context.Context, pool *db.Pool, job Job, reason string) error {
	return pool.InTx(ctx, func(tx pgx.Tx) error {
		if err := execFenced(ctx, tx.Exec,
			`UPDATE ingest_jobs SET status = 'failed', last_error_class = 'not_found', last_error_detail = $3,
			    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
			 WHERE `+fenceJob,
			job.ID, job.LeaseOwner, reason,
		); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE repositories SET is_eligible = false, eligibility_reason = $2 WHERE id = $1`,
			job.RepositoryID, reason,
		)
		return err
	})
}

// EnqueueInitialJobs inserts a pending "initial" job for every eligible
// repository that doesn't already have any ingest_jobs row at all — new
// repositories (freshly imported, or newly discovered) enter the pipeline
// this way. Idempotent: already-queued or already-processed repositories
// are untouched.
func EnqueueInitialJobs(ctx context.Context, pool *db.Pool, limit int) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at)
		 SELECT r.id, 'initial', 'pending', now()
		 FROM repositories r
		 WHERE r.is_eligible
		   AND NOT EXISTS (SELECT 1 FROM ingest_jobs j WHERE j.repository_id = r.id)
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: enqueue initial jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// EnqueueFreshnessJobs re-queues the first page of repositories whose last
// successful sync is older than olderThan — the Freshness half of the
// Initial/Freshness worker. Without this the catalog goes quiet once every
// repository has been seeded once: EnqueueInitialJobs only ever picks up
// repositories that have no ingest_jobs row at all.
//
// Still FIFO and still no score: "oldest successful sync first" is cycle
// ordering, not a ranking of repository value. The partial unique index on
// (repository_id) WHERE status IN ('pending','running') makes this safe to
// run on every cycle — a repository with work already queued is skipped.
func EnqueueFreshnessJobs(ctx context.Context, pool *db.Pool, limit int, olderThan time.Duration) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	tag, err := pool.Raw().Exec(ctx,
		`WITH due AS (
			SELECT r.id
			FROM repositories r
			WHERE r.is_eligible
			  AND r.last_synced_at IS NOT NULL
			  AND r.last_synced_at < now() - ($2 * interval '1 second')
			  AND NOT EXISTS (
				SELECT 1 FROM ingest_jobs j
				WHERE j.repository_id = r.id AND j.status IN ('pending', 'running')
			  )
			ORDER BY r.last_synced_at
			LIMIT $1
		 )
		 INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at)
		 SELECT id, 'initial', 'pending', now() FROM due
		 ON CONFLICT (repository_id) WHERE status IN ('pending', 'running') DO NOTHING`,
		limit, olderThan.Seconds(),
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: enqueue freshness jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RecomputeEligibility re-tests every stored repository against the current
// thresholds and rewrites is_eligible/eligibility_reason. The importer
// stores every record in the dataset, eligible or not, precisely so this is
// a pure SQL recompute: lowering the star floor pulls repositories into the
// crawl without re-reading the multi-GB source file, and raising it stops
// them being crawled.
//
// The rule mirrors cmd/repoimport's eligible() exactly — if one changes,
// both change. Repositories GitHub itself rejected (MarkUnavailable writes
// eligibility_reason = 'not_found') are left alone: that is a fact about
// the repository, not a threshold decision, and re-enabling them would
// re-queue work that is known to fail.
func RecomputeEligibility(ctx context.Context, pool *db.Pool, minStars, pushedWithinDays int) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`WITH judged AS (
			SELECT id,
			       array_to_string(array_remove(ARRAY[
			         CASE WHEN is_fork THEN 'is_fork' END,
			         CASE WHEN is_archived THEN 'archived' END,
			         -- NULL means "never imported with this field", not
			         -- "has no issues". Absence of data is unknown, never a
			         -- negative judgement — reading it as 0 here would
			         -- declare the entire pre-existing catalog ineligible
			         -- and stop the crawl dead.
			         CASE WHEN issue_count IS NOT NULL AND issue_count <= 0 THEN 'no_issues' END,
			         CASE WHEN language IS NULL OR language = '' THEN 'no_primary_language' END,
			         CASE WHEN pushed_at IS NULL
			                OR pushed_at < now() - ($2 * interval '1 day')
			              THEN 'stale' END,
			         CASE WHEN coalesce(stars, 0) < $1 THEN 'below_star_floor' END
			       ], NULL), ';') AS reason
			FROM repositories
			WHERE eligibility_reason IS DISTINCT FROM 'not_found'
		 )
		 UPDATE repositories r
		 SET is_eligible = (judged.reason = ''),
		     eligibility_reason = judged.reason
		 FROM judged
		 WHERE r.id = judged.id
		   AND (r.is_eligible IS DISTINCT FROM (judged.reason = '')
		        OR r.eligibility_reason IS DISTINCT FROM judged.reason)`,
		minStars, pushedWithinDays,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: recompute eligibility: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ReclaimExpiredLeases returns running jobs whose lease has lapsed (a
// crashed worker) back to pending, so they self-heal instead of being
// stuck forever (plan §7 "leases expire, so a crash self-heals").
func ReclaimExpiredLeases(ctx context.Context, pool *db.Pool) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`UPDATE ingest_jobs SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL
		 WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < now()`,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: reclaim expired leases: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
