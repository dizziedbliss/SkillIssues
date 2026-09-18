-- Migration 004: Add missing indexes and new sync_state columns
-- These columns were referenced in Go sync coordinator but missing from v1 schema.

ALTER TABLE sync_state
  ADD COLUMN IF NOT EXISTS last_issue_sync_at      TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_issue_update_seen  TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS sync_status             TEXT NOT NULL DEFAULT 'PENDING',
  ADD COLUMN IF NOT EXISTS failure_count           INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS next_sync_at            TIMESTAMPTZ;

-- New audit log table for anti-exploit and compliance tracking
CREATE TABLE IF NOT EXISTS contribution_audit_log (
  id              SERIAL      PRIMARY KEY,
  contribution_id INTEGER     NOT NULL REFERENCES contributions(id) ON DELETE CASCADE,
  user_id         INTEGER     NOT NULL REFERENCES users(id),
  from_state      TEXT,
  to_state        TEXT        NOT NULL,
  event           TEXT        NOT NULL,
  metadata        JSONB,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- New performance indexes
CREATE INDEX IF NOT EXISTS repositories_github_id_idx ON repositories(github_id);
CREATE INDEX IF NOT EXISTS repositories_stars_idx     ON repositories(stars DESC);
CREATE INDEX IF NOT EXISTS issues_github_id_idx       ON issues(github_id);
CREATE INDEX IF NOT EXISTS issues_learning_tags_idx   ON issues USING GIN(learning_tags);
CREATE INDEX IF NOT EXISTS contributions_issue_idx    ON contributions(issue_id);
CREATE INDEX IF NOT EXISTS sync_state_next_sync_idx   ON sync_state(next_sync_at);
CREATE INDEX IF NOT EXISTS audit_log_contribution_idx ON contribution_audit_log(contribution_id, created_at DESC);
