-- Discovery Sweeper: find repositories the dataset does not contain.
--
-- Until now a repository reached `repositories` only through a dataset import.
-- The sweeper runs GitHub repository Search over pushed-date windows with the
-- same thresholds eligibility uses (stars:>=min_stars fork:false
-- archived:false), and stores what passes `ingest.Eligible`. A new eligible
-- row is picked up by EnqueueInitialJobs like an imported one.
--
--   repositories.github_database_id
--       GitHub's numeric repository id. Stable across renames and transfers,
--       unlike full_name. Filled by the sweeper and by the crawl's first page
--       of every pass; NULL = not seen by either yet (every imported row,
--       until then). Not called github_id: 00021's Down re-adds a column of
--       that name.
--
--   discovery_sweeps   one pass over a pushed-date range. 'full' covers the
--                      whole ingest.eligibility.pushed_within_days range;
--                      'incremental' covers from the previous completed
--                      sweep's pushed_to. pushed_to of the latest 'done' sweep
--                      is the discovery watermark; a sweep with a failed
--                      window ends 'failed' and does not move it.
--   discovery_windows  the sweep's queue: one search time range each, leased
--                      and fenced like ingest_jobs. A window GitHub reports
--                      more than 1,000 matches for is replaced by its two
--                      halves (search returns at most 1,000 results).
--
-- 'duplicate' joins 'not_found' as an eligibility_reason RecomputeEligibility
-- leaves alone: a row whose GitHub id another row already holds (a dataset row
-- for a repository that was since renamed) is not crawled a second time.
--
-- +goose Up

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS github_database_id BIGINT;
CREATE UNIQUE INDEX IF NOT EXISTS repositories_github_database_id_idx
  ON repositories (github_database_id);

CREATE TABLE IF NOT EXISTS discovery_sweeps (
  id           BIGSERIAL   PRIMARY KEY,
  kind         TEXT        NOT NULL CHECK (kind IN ('full', 'incremental')),
  pushed_from  TIMESTAMPTZ NOT NULL,
  pushed_to    TIMESTAMPTZ NOT NULL,
  min_stars    INTEGER     NOT NULL,
  status       TEXT        NOT NULL DEFAULT 'running'
    CHECK (status IN ('running', 'done', 'failed')),
  started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  CHECK (pushed_from < pushed_to)
);

-- One sweep at a time.
CREATE UNIQUE INDEX IF NOT EXISTS discovery_sweeps_one_running_idx
  ON discovery_sweeps ((true)) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS discovery_sweeps_kind_done_idx
  ON discovery_sweeps (kind, completed_at) WHERE status = 'done';

CREATE TABLE IF NOT EXISTS discovery_windows (
  id                BIGSERIAL   PRIMARY KEY,
  sweep_id          BIGINT      NOT NULL REFERENCES discovery_sweeps(id) ON DELETE CASCADE,
  pushed_from       TIMESTAMPTZ NOT NULL,
  pushed_to         TIMESTAMPTZ NOT NULL,
  status            TEXT        NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'running', 'done', 'failed')),
  cursor            TEXT,
  pages_done        INTEGER     NOT NULL DEFAULT 0,
  repository_count  INTEGER,
  repos_seen        INTEGER     NOT NULL DEFAULT 0,
  split             BOOLEAN     NOT NULL DEFAULT false,
  truncated         BOOLEAN     NOT NULL DEFAULT false,
  attempt_count     INTEGER     NOT NULL DEFAULT 0,
  lease_owner       TEXT,
  lease_expires_at  TIMESTAMPTZ,
  next_retry_at     TIMESTAMPTZ,
  last_error_class  TEXT,
  last_error_detail TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (pushed_from < pushed_to)
);

CREATE INDEX IF NOT EXISTS discovery_windows_claim_idx ON discovery_windows (status, next_retry_at);
CREATE INDEX IF NOT EXISTS discovery_windows_sweep_idx ON discovery_windows (sweep_id, status);

GRANT SELECT, INSERT, UPDATE ON discovery_sweeps, discovery_windows TO skillissues_ingest;
GRANT USAGE ON SEQUENCE discovery_sweeps_id_seq, discovery_windows_id_seq TO skillissues_ingest;
GRANT SELECT ON discovery_sweeps, discovery_windows TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.discovery.enabled', 'false', 'bool', NULL, NULL,
   'Run the Discovery Sweeper: GitHub repository Search over pushed-date windows for repositories the dataset does not contain.'),
  ('ingest.discovery_workers', '1', 'int', 1, 16,
   'Concurrent Discovery Sweeper workers (one search page each).'),
  ('ingest.discovery.window_hours', '24', 'int', 1, 720,
   'Pushed-date span of each search window a new sweep starts with; denser windows are halved.'),
  ('ingest.discovery.min_window_seconds', '60', 'int', 1, 86400,
   'Smallest window a dense one is halved down to. Past it, only GitHub''s first 1,000 results are read and the window is marked truncated.'),
  ('ingest.discovery.page_size', '100', 'int', 1, 100,
   'Repositories requested per search page. GitHub caps this at 100.'),
  ('ingest.discovery.rate_limit_floor', '1000', 'int', 0, 12500,
   'Discovery stops searching when the shared crawler budget falls below this; kept above ingest.rate_limit_floor so the issue crawl keeps going after discovery yields.'),
  ('ingest.discovery.incremental_interval_seconds', '86400', 'int', 3600, 2592000,
   'How often an incremental sweep covers repositories pushed since the last completed sweep.'),
  ('ingest.discovery.full_interval_seconds', '2592000', 'int', 86400, 31536000,
   'How often a full sweep covers the whole pushed-within range, catching repositories that crossed the star floor without a new push.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.discovery.enabled',
  'ingest.discovery_workers',
  'ingest.discovery.window_hours',
  'ingest.discovery.min_window_seconds',
  'ingest.discovery.page_size',
  'ingest.discovery.rate_limit_floor',
  'ingest.discovery.incremental_interval_seconds',
  'ingest.discovery.full_interval_seconds'
);

DROP TABLE IF EXISTS discovery_windows;
DROP TABLE IF EXISTS discovery_sweeps;

-- A 'duplicate' row goes back to being re-judged by the thresholds.
UPDATE repositories SET eligibility_reason = NULL WHERE eligibility_reason = 'duplicate';

DROP INDEX IF EXISTS repositories_github_database_id_idx;
ALTER TABLE repositories DROP COLUMN IF EXISTS github_database_id;
