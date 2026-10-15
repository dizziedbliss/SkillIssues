-- Incremental freshness and closed-issue detection.
--
-- Until now a freshness re-check re-crawled every page of a repository, and
-- it asked GitHub only for open issues, so an issue that closed simply
-- stopped coming back and stayed OPEN here forever. A freshness pass now
-- fetches only what changed since the repository's last completed pass:
-- open issues updated since then (upserted) and issues closed since then
-- (marked CLOSED).
--
--   repositories.issues_synced_through
--       Every open/closed change up to this time is reflected in issues.
--       It is the enqueue time of the last pass that ran to completion, so
--       it can only ever be behind the truth, never ahead. NULL = no pass
--       has completed; the next one is a full crawl.
--
--   ingest_jobs.since            the watermark this pass fetches changes
--                                after; NULL for a full crawl
--   ingest_jobs.pass_started_at  enqueue time of the pass's first job;
--                                becomes the new watermark on completion
--   ingest_jobs.closed_cursor    cursor of the closed-issues connection
--   ingest_jobs.open_done        open connection exhausted
--   ingest_jobs.closed_done      closed connection exhausted
--
-- +goose Up

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS issues_synced_through TIMESTAMPTZ;

ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS since           TIMESTAMPTZ;
ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS pass_started_at TIMESTAMPTZ;
ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS closed_cursor   TEXT;
ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS open_done       BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS closed_done     BOOLEAN NOT NULL DEFAULT false;

-- Passes already in flight are full crawls (since NULL). Their start is the
-- creation of the repository's latest initial job; a deep job started later
-- inherits it.
UPDATE ingest_jobs j
SET pass_started_at = (
  SELECT max(i.created_at) FROM ingest_jobs i
  WHERE i.repository_id = j.repository_id AND i.job_type = 'initial' AND i.created_at <= j.created_at
)
WHERE j.status IN ('pending', 'running');
UPDATE ingest_jobs SET pass_started_at = created_at
WHERE status IN ('pending', 'running') AND pass_started_at IS NULL;

ALTER TABLE ingest_jobs ALTER COLUMN pass_started_at SET DEFAULT now();

-- Repositories with no pass in flight whose latest pass completed: the
-- watermark is that pass's start. A pass whose deep job failed did not
-- complete, so those stay NULL and get a full crawl next time.
UPDATE repositories r
SET issues_synced_through = last_pass.created_at
FROM (
  SELECT DISTINCT ON (repository_id) repository_id, created_at
  FROM ingest_jobs
  WHERE job_type = 'initial' AND status = 'done'
  ORDER BY repository_id, created_at DESC
) last_pass
WHERE r.id = last_pass.repository_id
  AND NOT EXISTS (
    SELECT 1 FROM ingest_jobs j
    WHERE j.repository_id = r.id
      AND (j.status IN ('pending', 'running')
           OR (j.status = 'failed' AND j.created_at >= last_pass.created_at))
  );

-- Queued freshness passes that have not fetched anything yet become
-- incremental too: their since is the start of the repository's previous
-- completed pass. Without this, the backlog queued before this migration
-- would still re-crawl every page.
UPDATE ingest_jobs j
SET since = prev.created_at
FROM ingest_jobs prev
WHERE j.status = 'pending' AND j.job_type = 'initial' AND j.pages_done = 0
  AND prev.id = (
    SELECT p.id FROM ingest_jobs p
    WHERE p.repository_id = j.repository_id AND p.job_type = 'initial'
      AND p.status = 'done' AND p.created_at < j.created_at
    ORDER BY p.created_at DESC LIMIT 1
  )
  AND NOT EXISTS (
    SELECT 1 FROM ingest_jobs f
    WHERE f.repository_id = j.repository_id AND f.status = 'failed'
      AND f.created_at >= prev.created_at AND f.created_at < j.created_at
  );

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.closed_page_size', '40', 'int', 1, 100,
   'Issues closed since a repository''s last completed pass, fetched alongside each page of a freshness pass.'),
  ('ingest.tracked_issue_check_seconds', '3600', 'int', 60, 86400,
   'How stale an open issue a user saved or is working on may get before its state is re-checked on GitHub.'),
  ('ingest.tracked_issue_batch_size', '50', 'int', 1, 100,
   'Tracked issues checked per GitHub request.')
ON CONFLICT (key) DO NOTHING;

UPDATE settings
SET description = 'How old a repository''s last sync must be before a freshness pass (every change since its last completed pass) is queued.'
WHERE key = 'ingest.freshness_interval_seconds';

-- +goose Down

UPDATE settings
SET description = 'How old a repository''s last successful sync must be before its first issue page is re-fetched.'
WHERE key = 'ingest.freshness_interval_seconds';

DELETE FROM settings WHERE key IN (
  'ingest.closed_page_size',
  'ingest.tracked_issue_check_seconds',
  'ingest.tracked_issue_batch_size'
);

ALTER TABLE ingest_jobs DROP COLUMN IF EXISTS closed_done;
ALTER TABLE ingest_jobs DROP COLUMN IF EXISTS open_done;
ALTER TABLE ingest_jobs DROP COLUMN IF EXISTS closed_cursor;
ALTER TABLE ingest_jobs DROP COLUMN IF EXISTS pass_started_at;
ALTER TABLE ingest_jobs DROP COLUMN IF EXISTS since;

ALTER TABLE repositories DROP COLUMN IF EXISTS issues_synced_through;
