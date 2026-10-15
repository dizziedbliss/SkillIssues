-- Removes what nothing reads (checked against prod on 2026-10-16, 287k rows):
--
--   github_id         NULL on every row; the import never sets it.
--   forkable          identical to forking_allowed on every row (both are
--                     written from the dataset's forkingAllowed); never read.
--   last_activity_at  identical to pushed_at on every row (the import writes
--                     pushedAt into both); never read.
--   parent_full_name  NULL on every row: only forks have a parent, and forks
--                     are filtered out at import.
--   is_fork           false on every row, for the same reason. The import
--                     still rejects forks from the dataset's isFork field;
--                     nothing stored can be one.
--   default_branch    NULL on every row; the desktop app reads it live from
--                     GitHub.
--   sync_state        the pre-ingestd sync table. Empty; nothing reads or
--                     writes it.
--
-- +goose Up

ALTER TABLE repositories DROP COLUMN IF EXISTS github_id;
ALTER TABLE repositories DROP COLUMN IF EXISTS forkable;
ALTER TABLE repositories DROP COLUMN IF EXISTS last_activity_at;
ALTER TABLE repositories DROP COLUMN IF EXISTS parent_full_name;
ALTER TABLE repositories DROP COLUMN IF EXISTS is_fork;
ALTER TABLE repositories DROP COLUMN IF EXISTS default_branch;

DROP TABLE IF EXISTS sync_state;

-- +goose Down

CREATE TABLE IF NOT EXISTS sync_state (
  repository_id          INTEGER     PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  last_synced_at         TIMESTAMPTZ,
  last_issue_sync_at     TIMESTAMPTZ,
  last_issue_update_seen TIMESTAMPTZ,
  priority               TEXT        NOT NULL DEFAULT 'MEDIUM'
    CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
  sync_status            TEXT        NOT NULL DEFAULT 'PENDING'
    CHECK (sync_status IN ('PENDING', 'RUNNING', 'DONE', 'FAILED')),
  failure_count          INTEGER     NOT NULL DEFAULT 0,
  next_sync_at           TIMESTAMPTZ,
  rate_limit_limit       INTEGER,
  rate_limit_remaining   INTEGER,
  rate_limit_used        INTEGER,
  rate_limit_reset_at    TIMESTAMPTZ,
  rate_limit_cost        INTEGER
);
CREATE INDEX IF NOT EXISTS sync_state_priority_idx  ON sync_state(priority, last_synced_at NULLS FIRST);
CREATE INDEX IF NOT EXISTS sync_state_next_sync_idx ON sync_state(next_sync_at);
GRANT SELECT ON sync_state TO metrics_ro;

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS default_branch   TEXT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS is_fork          BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS parent_full_name TEXT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS forkable         BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS github_id        BIGINT UNIQUE;
UPDATE repositories SET last_activity_at = pushed_at, forkable = forking_allowed;
