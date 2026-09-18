DROP INDEX IF EXISTS idx_contributions_user_issue_active;
ALTER TABLE contributions ADD CONSTRAINT contributions_user_id_issue_id_key UNIQUE (user_id, issue_id);
