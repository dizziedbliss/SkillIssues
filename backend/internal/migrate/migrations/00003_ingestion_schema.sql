-- Ingestion schema: repository/issue metadata needed for broad crawling, the
-- job queue the ingestion workers claim leases from, shared GitHub rate-limit
-- budget tracking, and a bounded on-demand issue-body cache.
--
-- Additive and backward compatible — existing code keeps working unchanged
-- until the ingestion workers (a later phase) are wired up to use these.
--
-- +goose Up

-- ---------------------------------------------------------------------------
-- 1. Issues: broad crawling needs raw labels and an "unknown difficulty"
--    state, since absence of a difficulty label is not the same as BEGINNER.
-- ---------------------------------------------------------------------------

ALTER TABLE issues ADD COLUMN IF NOT EXISTS labels TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE issues ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS classification_rules_version INTEGER NOT NULL DEFAULT 0;

-- difficulty_score has no meaning for an issue that hasn't been classified
-- yet; broad ingestion inserts issues long before classification runs.
ALTER TABLE issues ALTER COLUMN difficulty_score DROP NOT NULL;

ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_difficulty_check;
ALTER TABLE issues ADD CONSTRAINT issues_difficulty_check
  CHECK (difficulty IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED', 'UNCLASSIFIED'));

CREATE INDEX IF NOT EXISTS issues_labels_idx ON issues USING GIN(labels);

-- ---------------------------------------------------------------------------
-- 2. Repositories: metadata needed to decide crawl eligibility and to
--    discover renamed/case-changed duplicates.
-- ---------------------------------------------------------------------------

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS forks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS watchers INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS pushed_at TIMESTAMPTZ;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS topics TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS languages JSONB;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS forking_allowed BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS parent_full_name TEXT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS default_branch TEXT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS is_eligible BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS eligibility_reason TEXT;

-- Guards against two repositories differing only by case (a rename GitHub
-- treats as the same repo); full_name is already uniquely indexed exactly.
CREATE UNIQUE INDEX IF NOT EXISTS repositories_full_name_lower_idx ON repositories (lower(full_name));

-- ---------------------------------------------------------------------------
-- 3. Ingestion job queue: one row per unit of crawl work, claimed via a
--    leased row rather than an in-process queue, so any worker instance can
--    pick up where another left off or crashed.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest_jobs (
  id                SERIAL      PRIMARY KEY,
  repository_id     INTEGER     REFERENCES repositories(id) ON DELETE CASCADE,
  job_type          TEXT        NOT NULL
    CHECK (job_type IN ('initial', 'freshness', 'deep', 'discovery')),
  status            TEXT        NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'running', 'done', 'failed')),
  attempt_count     INTEGER     NOT NULL DEFAULT 0,
  lease_owner       TEXT,
  lease_expires_at  TIMESTAMPTZ,
  next_retry_at     TIMESTAMPTZ,
  cursor            TEXT,
  pages_done        INTEGER     NOT NULL DEFAULT 0,
  has_next_page     BOOLEAN     NOT NULL DEFAULT false,
  last_error_class  TEXT,
  last_error_detail TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ingest_jobs_claim_idx ON ingest_jobs (status, next_retry_at);

-- One active (not yet done/failed) pagination job per repository, so two
-- workers can never claim overlapping pages of the same repository.
CREATE UNIQUE INDEX IF NOT EXISTS ingest_jobs_active_repo_idx
  ON ingest_jobs (repository_id)
  WHERE status IN ('pending', 'running');

-- ---------------------------------------------------------------------------
-- 4. Shared GitHub rate-limit budget: lets the crawler (installation token)
--    and interactive on-demand reads (a user's own token) hold separate
--    reservations instead of contending over one process-local counter.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS github_budget (
  identity        TEXT        NOT NULL,
  lane            TEXT        NOT NULL CHECK (lane IN ('crawler', 'interactive')),
  rate_limit      INTEGER,
  remaining       INTEGER,
  reset_at        TIMESTAMPTZ,
  secondary_until TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (identity, lane)
);

-- ---------------------------------------------------------------------------
-- 5. On-demand issue body cache: bounded LRU, pruned by last_accessed_at.
--    There is deliberately no unbounded body storage anywhere else.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS issue_body_cache (
  issue_id         INTEGER     PRIMARY KEY REFERENCES issues(id) ON DELETE CASCADE,
  body             TEXT,
  fetched_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS issue_body_cache_lru_idx ON issue_body_cache (last_accessed_at);

-- ---------------------------------------------------------------------------
-- 6. Skip ledger for the repository metadata importer: why a candidate
--    repository was not imported, so a re-run doesn't silently retry it.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS import_skipped (
  full_name  TEXT        PRIMARY KEY,
  reason     TEXT        NOT NULL,
  skipped_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 7. Co-contribution trailer preference: default false is the decision, not
--    just a column default — nobody is opted in without asking.
-- ---------------------------------------------------------------------------

ALTER TABLE developer_preferences ADD COLUMN IF NOT EXISTS co_contribution_enabled BOOLEAN NOT NULL DEFAULT false;

-- +goose Down

ALTER TABLE developer_preferences DROP COLUMN IF EXISTS co_contribution_enabled;

DROP TABLE IF EXISTS import_skipped;
DROP TABLE IF EXISTS issue_body_cache;
DROP TABLE IF EXISTS github_budget;
DROP TABLE IF EXISTS ingest_jobs;

DROP INDEX IF EXISTS repositories_full_name_lower_idx;
ALTER TABLE repositories DROP COLUMN IF EXISTS eligibility_reason;
ALTER TABLE repositories DROP COLUMN IF EXISTS is_eligible;
ALTER TABLE repositories DROP COLUMN IF EXISTS default_branch;
ALTER TABLE repositories DROP COLUMN IF EXISTS parent_full_name;
ALTER TABLE repositories DROP COLUMN IF EXISTS forking_allowed;
ALTER TABLE repositories DROP COLUMN IF EXISTS languages;
ALTER TABLE repositories DROP COLUMN IF EXISTS topics;
ALTER TABLE repositories DROP COLUMN IF EXISTS pushed_at;
ALTER TABLE repositories DROP COLUMN IF EXISTS created_at;
ALTER TABLE repositories DROP COLUMN IF EXISTS is_archived;
ALTER TABLE repositories DROP COLUMN IF EXISTS watchers;
ALTER TABLE repositories DROP COLUMN IF EXISTS forks;

DROP INDEX IF EXISTS issues_labels_idx;
ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_difficulty_check;
ALTER TABLE issues ADD CONSTRAINT issues_difficulty_check
  CHECK (difficulty IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED'));
-- difficulty_score is intentionally left nullable on rollback: forcing NOT
-- NULL back would fail if any row was inserted as UNCLASSIFIED (score NULL)
-- while this migration was applied.
ALTER TABLE issues DROP COLUMN IF EXISTS classification_rules_version;
ALTER TABLE issues DROP COLUMN IF EXISTS created_at;
ALTER TABLE issues DROP COLUMN IF EXISTS labels;
