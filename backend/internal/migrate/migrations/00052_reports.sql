-- Reports (internal/report): a signed-in person flags an issue, a repository
-- or a public theme (spam, abuse, already fixed, malware, ...). Reports are
-- private: only the admin endpoints read them. Enough people reporting the
-- same target (api.reports.hide_after) takes it out of lists until it is
-- reviewed. Upheld, a repository is excluded (excluded_repositories), a theme
-- made private, and an issue stays hidden. Dismissed, it comes back.
--
-- One open report per person per target. A report goes with the reporter's
-- account (ON DELETE CASCADE), like everything else tied to it.

-- +goose Up

CREATE TABLE IF NOT EXISTS reports (
  id          BIGSERIAL   PRIMARY KEY,
  user_id     INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_type TEXT        NOT NULL CHECK (target_type IN ('issue', 'repository', 'theme')),
  target_id   TEXT        NOT NULL,
  reason      TEXT        NOT NULL,
  note        TEXT        NOT NULL DEFAULT '',
  status      TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'upheld', 'dismissed')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS reports_one_open_idx
  ON reports (user_id, target_type, target_id) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS reports_target_idx ON reports (target_type, target_id, status);
CREATE INDEX IF NOT EXISTS reports_user_recent_idx ON reports (user_id, created_at);

GRANT SELECT, INSERT, UPDATE ON reports TO skillissues_api;
GRANT USAGE ON SEQUENCE reports_id_seq TO skillissues_api;
-- Upholding a repository report excludes it, as the operator would by hand.
GRANT INSERT ON excluded_repositories TO skillissues_api;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.reports.hide_after', '3', 'int', 1, 100,
   'Different people reporting the same issue, repository or theme before it leaves lists until reviewed.'),
  ('api.reports.per_user_per_day', '20', 'int', 1, 1000,
   'Reports one person may make in 24 hours.'),
  ('api.reports.hidden_refresh_seconds', '60', 'int', 5, 3600,
   'How often go-api re-reads which reported targets are hidden.'),
  ('api.themes.public_page_size', '24', 'int', 1, 100,
   'Public themes per page of the community listing.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN ('api.reports.hide_after', 'api.reports.per_user_per_day',
  'api.reports.hidden_refresh_seconds', 'api.themes.public_page_size');
REVOKE INSERT ON excluded_repositories FROM skillissues_api;
DROP TABLE IF EXISTS reports;
