-- In-app notifications. The first and only kind is issue_closed: an issue a
-- user saved, or is working on (contribution STARTED or PR_OPEN), closed on
-- GitHub.
--
-- They are written by a trigger on the OPEN -> CLOSED transition rather than
-- by Go code, because four separate paths close issues (a crawl freshness
-- pass, ingestd's tracked-issue check, the API's on-demand check, and the
-- webhook) and a notification must not depend on each remembering to.
--
-- One row per (user, kind, issue). An issue that is reopened and closes
-- again re-notifies: the row is bumped to unread rather than duplicated.
-- Rows go with their issue: an issue is only pruned once nobody saved or
-- worked on it, at which point its notifications have nobody to inform.
--
-- saved_issues is keyed (user_id, issue_id), so the trigger's lookup by
-- issue needs saved_issues_issue_idx; contributions already has one.
--
-- +goose Up

CREATE TABLE IF NOT EXISTS notifications (
  id         BIGSERIAL   PRIMARY KEY,
  user_id    INTEGER     NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  kind       TEXT        NOT NULL CHECK (kind IN ('issue_closed')),
  issue_id   INTEGER     NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  read_at    TIMESTAMPTZ,
  UNIQUE (user_id, kind, issue_id)
);

CREATE INDEX IF NOT EXISTS notifications_user_idx   ON notifications (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS saved_issues_issue_idx   ON saved_issues (issue_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_issue_closed() RETURNS trigger AS $$
BEGIN
  INSERT INTO notifications (user_id, kind, issue_id)
  SELECT u.user_id, 'issue_closed', NEW.id
  FROM (
    SELECT user_id FROM saved_issues WHERE issue_id = NEW.id
    UNION
    SELECT user_id FROM contributions WHERE issue_id = NEW.id AND state IN ('STARTED', 'PR_OPEN')
  ) u
  ON CONFLICT (user_id, kind, issue_id) DO UPDATE SET created_at = now(), read_at = NULL;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS issues_notify_closed ON issues;
CREATE TRIGGER issues_notify_closed
  AFTER UPDATE OF state ON issues
  FOR EACH ROW
  WHEN (OLD.state = 'OPEN' AND NEW.state = 'CLOSED')
  EXECUTE FUNCTION notify_issue_closed();

-- CLAUDE.md: every new table is granted to metrics_ro in its migration.
GRANT SELECT ON notifications TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.notifications.page_size', '50', 'int', 1, 200,
   'Notifications returned by GET /notifications.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'api.notifications.page_size';
DROP TRIGGER IF EXISTS issues_notify_closed ON issues;
DROP FUNCTION IF EXISTS notify_issue_closed();
DROP INDEX IF EXISTS saved_issues_issue_idx;
DROP TABLE IF EXISTS notifications;
