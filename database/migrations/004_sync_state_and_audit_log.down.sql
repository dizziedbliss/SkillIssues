DROP TABLE IF EXISTS contribution_audit_log CASCADE;

DROP INDEX IF EXISTS repositories_github_id_idx;
DROP INDEX IF EXISTS repositories_stars_idx;
DROP INDEX IF EXISTS issues_github_id_idx;
DROP INDEX IF EXISTS issues_learning_tags_idx;
DROP INDEX IF EXISTS contributions_issue_idx;
DROP INDEX IF EXISTS sync_state_next_sync_idx;

ALTER TABLE sync_state
  DROP COLUMN IF EXISTS last_issue_sync_at,
  DROP COLUMN IF EXISTS last_issue_update_seen,
  DROP COLUMN IF EXISTS sync_status,
  DROP COLUMN IF EXISTS failure_count,
  DROP COLUMN IF EXISTS next_sync_at;
