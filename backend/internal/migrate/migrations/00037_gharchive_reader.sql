-- The GH Archive reader: issue freshness from GitHub's public event stream.
--
-- GH Archive publishes every public GitHub event as one gzipped JSON file per
-- hour. IssuesEvent and IssueCommentEvent carry the full issue as it stood
-- (labels, state, updated_at), so ingestd applies them to the issues of
-- eligible repositories without a single GitHub API call, and the per-
-- repository freshness crawl becomes a slow repair pass. Off by default
-- (ingest.archive.enabled).
--
--   archive_hours    one row per hour of the archive: a leased, fenced queue
--                    like ingest_jobs. 'pending' (not processed, or its file
--                    not published yet), 'running', 'done', 'failed' (gave up
--                    after ingest.max_attempts processing failures) and
--                    'missing' (never published, after
--                    ingest.archive.missing_after_hours).
--   repo_candidates  repositories the archive showed issue activity on that
--                    are not stored. Each is looked up on GitHub and stored if
--                    it is eligible, then not looked at again until
--                    ingest.archive.candidate_recheck_days have passed.
--
-- Settings:
--   ingest.archive.enabled                   read the archive at all
--   ingest.archive.workers                   hours processed at once
--   ingest.archive.poll_seconds              how often new hours are queued
--   ingest.archive.retry_seconds             wait before retrying an hour
--                                            that is not published yet or
--                                            failed
--   ingest.archive.missing_after_hours       an hour still unpublished this
--                                            long after it ended is missing
--   ingest.archive.max_backfill_hours        how far back the first run goes,
--                                            at most
--   ingest.archive.download_timeout_seconds  deadline for one hour's file
--   ingest.archive.max_file_mb               largest file accepted, compressed
--   ingest.archive.apply_batch_size          issues written per transaction
--   ingest.archive.max_lag_seconds           how far behind the archive may be
--                                            before the daily freshness crawl
--                                            takes over again
--   ingest.archive.repair_interval_seconds   the freshness crawl's interval
--                                            while the archive keeps up
--   ingest.archive.candidate_batch_size      candidates looked up per call
--   ingest.archive.candidate_recheck_days    when a candidate is looked at
--                                            again

-- +goose Up

CREATE TABLE IF NOT EXISTS archive_hours (
  hour             TIMESTAMPTZ PRIMARY KEY,
  status           TEXT        NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'running', 'done', 'failed', 'missing')),
  attempts         INTEGER     NOT NULL DEFAULT 0,
  next_retry_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  lease_owner      TEXT,
  lease_expires_at TIMESTAMPTZ,
  events_read      INTEGER,
  issues_applied   INTEGER,
  candidates_found INTEGER,
  last_error       TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at      TIMESTAMPTZ,
  CHECK (hour = date_trunc('hour', hour))
);

CREATE INDEX IF NOT EXISTS archive_hours_due_idx
  ON archive_hours (next_retry_at, hour) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS repo_candidates (
  github_database_id BIGINT      PRIMARY KEY,
  full_name          TEXT        NOT NULL,
  first_seen_hour    TIMESTAMPTZ NOT NULL,
  last_seen_hour     TIMESTAMPTZ NOT NULL,
  checked_at         TIMESTAMPTZ,
  outcome            TEXT
);

CREATE INDEX IF NOT EXISTS repo_candidates_due_idx
  ON repo_candidates (checked_at NULLS FIRST, github_database_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON archive_hours, repo_candidates TO skillissues_ingest;
GRANT SELECT ON archive_hours, repo_candidates TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.archive.enabled', 'false', 'bool', NULL, NULL,
   'Read GH Archive''s hourly public event files and apply their issue events to stored repositories.'),
  ('ingest.archive.workers', '1', 'int', 1, 8,
   'Archive hours processed at once.'),
  ('ingest.archive.poll_seconds', '300', 'int', 30, 3600,
   'How often newly ended hours are queued for the archive reader.'),
  ('ingest.archive.retry_seconds', '600', 'int', 60, 86400,
   'Wait before retrying an archive hour whose file is not published yet, or whose processing failed.'),
  ('ingest.archive.missing_after_hours', '24', 'int', 1, 720,
   'An archive hour whose file is still not published this many hours after it ended is marked missing.'),
  ('ingest.archive.max_backfill_hours', '168', 'int', 1, 2160,
   'On first start, the archive reader goes back to the oldest completed crawl pass, but no further than this.'),
  ('ingest.archive.download_timeout_seconds', '300', 'int', 10, 3600,
   'Deadline for downloading and reading one archive hour.'),
  ('ingest.archive.max_file_mb', '512', 'int', 1, 4096,
   'Largest compressed archive file accepted; a larger one fails its hour.'),
  ('ingest.archive.apply_batch_size', '500', 'int', 1, 10000,
   'Issues from the archive written per transaction.'),
  ('ingest.archive.max_lag_seconds', '21600', 'int', 3600, 604800,
   'If the archive falls further behind than this, the freshness crawl returns to ingest.freshness_interval_seconds and the incremental discovery sweep resumes.'),
  ('ingest.archive.repair_interval_seconds', '2592000', 'int', 86400, 31536000,
   'While the archive keeps up, the freshness crawl re-checks each repository this rarely, to catch what events miss (deleted or transferred issues).'),
  ('ingest.archive.candidate_batch_size', '50', 'int', 1, 100,
   'Repositories seen in the archive but not stored, looked up on GitHub per call.'),
  ('ingest.archive.candidate_recheck_days', '30', 'int', 1, 3650,
   'A looked-up candidate that was not stored is looked up again after this many days, if it is still active.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.archive.enabled',
  'ingest.archive.workers',
  'ingest.archive.poll_seconds',
  'ingest.archive.retry_seconds',
  'ingest.archive.missing_after_hours',
  'ingest.archive.max_backfill_hours',
  'ingest.archive.download_timeout_seconds',
  'ingest.archive.max_file_mb',
  'ingest.archive.apply_batch_size',
  'ingest.archive.max_lag_seconds',
  'ingest.archive.repair_interval_seconds',
  'ingest.archive.candidate_batch_size',
  'ingest.archive.candidate_recheck_days'
);

DROP TABLE IF EXISTS repo_candidates;
DROP TABLE IF EXISTS archive_hours;
