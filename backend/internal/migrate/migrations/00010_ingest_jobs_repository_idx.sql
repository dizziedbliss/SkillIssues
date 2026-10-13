-- ingest_jobs.repository_id is a foreign key with ON DELETE CASCADE, but the
-- only index covering that column was the *partial* unique index
-- ingest_jobs_active_repo_idx (WHERE status IN ('pending','running')).
-- Postgres cannot use a partial index to satisfy an FK cascade lookup, so
-- every delete of a repositories row sequentially scanned the whole
-- ingest_jobs table — at 227k jobs that made a bulk repository delete take
-- hours instead of minutes.
--
-- The same missing index also forces a scan for the NOT EXISTS probe in
-- ingest.EnqueueFreshnessJobs whenever a job's status has moved past
-- pending/running.
--
-- +goose Up

CREATE INDEX IF NOT EXISTS ingest_jobs_repository_idx ON ingest_jobs (repository_id);

-- +goose Down

DROP INDEX IF EXISTS ingest_jobs_repository_idx;
