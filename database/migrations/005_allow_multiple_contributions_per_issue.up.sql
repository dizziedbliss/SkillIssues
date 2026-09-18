ALTER TABLE contributions DROP CONSTRAINT IF EXISTS contributions_user_id_issue_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contributions_user_issue_active ON contributions (user_id, issue_id) WHERE state = 'STARTED';
