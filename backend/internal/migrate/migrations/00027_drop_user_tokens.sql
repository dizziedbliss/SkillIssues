-- The server holds no GitHub credential that can act for a user. Sign-in
-- reads a numeric GitHub id and discards the token; fork, push and pull
-- requests happen in the desktop client with a token that never leaves the
-- machine. Everything that stored, encrypted or spent a user token goes.
--
-- There are no real users yet, so accounts, sessions and contributions are
-- deleted rather than migrated: existing rows could have been matched by
-- username (the account-takeover path this redesign closes), and the stored
-- tokens must not outlive this migration in a backup either way.
--
-- Also removed with it, because they only existed for the token-holding
-- design:
--   * contributions and contribution_audit_log — gamification is deferred to
--     its redesign, and the old rows were awarded under the old rules.
--   * XP, level, badge and progress columns on developer_profiles.
--   * issue_body_cache and its settings — bodies are fetched by clients.
--   * settings for the endpoints that are gone.
--
-- The Down migration restores the schema, empty. The data is not recoverable.
--
-- +goose Up

DELETE FROM notifications;
DELETE FROM saved_issues;
DELETE FROM auth_sessions;
DELETE FROM oauth_states;
DELETE FROM users;

-- The closing trigger told people working on an issue (a contribution) as
-- well as people who saved it. Only saved issues remain.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_issue_closed() RETURNS trigger AS $$
BEGIN
  INSERT INTO notifications (user_id, kind, issue_id)
  SELECT user_id, 'issue_closed', NEW.id
  FROM saved_issues WHERE issue_id = NEW.id
  ON CONFLICT (user_id, kind, issue_id) DO UPDATE SET created_at = now(), read_at = NULL;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TABLE IF EXISTS contribution_audit_log;
DROP TABLE IF EXISTS contributions;
DROP TABLE IF EXISTS issue_body_cache;

ALTER TABLE users DROP COLUMN IF EXISTS github_access_token;
ALTER TABLE users DROP COLUMN IF EXISTS github_refresh_token;
ALTER TABLE users DROP COLUMN IF EXISTS github_token_expires_at;

-- github_id is the only identity. The login is display data that GitHub
-- lets another account take after a rename, so it may be cleared when a new
-- owner signs in (see auth.UpsertUser).
ALTER TABLE users ALTER COLUMN github_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN username DROP NOT NULL;

ALTER TABLE developer_profiles DROP COLUMN IF EXISTS progress_score;
ALTER TABLE developer_profiles DROP COLUMN IF EXISTS completed_count;
ALTER TABLE developer_profiles DROP COLUMN IF EXISTS total_xp;
ALTER TABLE developer_profiles DROP COLUMN IF EXISTS level;
ALTER TABLE developer_profiles DROP COLUMN IF EXISTS badges;

DELETE FROM settings WHERE key IN (
  'api.contribution.verify_per_hour',
  'api.issues.freshness_batch_cap',
  'api.issue_body.cache_max_rows',
  'api.issue_body.prune_interval_seconds',
  'api.issue_body.max_concurrent_fetches',
  'api.issue_body.installation_floor',
  'auth.pat_login_enabled'
);

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.session.prune_interval_seconds', '3600', 'int', 60, 86400,
   'How often expired sessions and abandoned OAuth handshakes are deleted.')
ON CONFLICT (key) DO NOTHING;

-- 00017 granted metrics_ro the contribution tables; dropping a table drops
-- its grants, so nothing to revoke. No credential column remains on users.

-- +goose Down

DELETE FROM settings WHERE key = 'api.session.prune_interval_seconds';

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.contribution.verify_per_hour', '5', 'int', 1, 1000,
   'Maximum PR verification attempts per user per contribution per hour.'),
  ('api.issues.freshness_batch_cap', '25', 'int', 1, 100,
   'Maximum issue ids accepted by one POST /issues/freshness call.'),
  ('api.issue_body.cache_max_rows', '50000', 'int', 100, 10000000,
   'Rows kept in issue_body_cache; the least recently read are pruned beyond this.'),
  ('api.issue_body.prune_interval_seconds', '600', 'int', 60, 86400,
   'How often issue_body_cache is pruned back to api.issue_body.cache_max_rows.'),
  ('api.issue_body.max_concurrent_fetches', '8', 'int', 1, 64,
   'Concurrent GitHub fetches for issue bodies across all users.'),
  ('api.issue_body.installation_floor', '100', 'int', 0, 5000,
   'Remaining GraphQL points below which the installation token stops serving issue-body reads.')
ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, value_type, description) VALUES
  ('auth.pat_login_enabled', 'false', 'bool',
   'Whether POST /auth/github/token is reachable.')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE developer_profiles ADD COLUMN IF NOT EXISTS progress_score  INTEGER NOT NULL DEFAULT 18;
ALTER TABLE developer_profiles ADD COLUMN IF NOT EXISTS completed_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE developer_profiles ADD COLUMN IF NOT EXISTS total_xp        INTEGER NOT NULL DEFAULT 0;
ALTER TABLE developer_profiles ADD COLUMN IF NOT EXISTS level           INTEGER NOT NULL DEFAULT 1;
ALTER TABLE developer_profiles ADD COLUMN IF NOT EXISTS badges          TEXT[]  NOT NULL DEFAULT '{}';

ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ALTER COLUMN github_id DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_access_token     TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_refresh_token    TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_token_expires_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS issue_body_cache (
  issue_id         INTEGER     PRIMARY KEY REFERENCES issues(id) ON DELETE CASCADE,
  body             TEXT,
  fetched_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS issue_body_cache_lru_idx ON issue_body_cache (last_accessed_at);

CREATE TABLE IF NOT EXISTS contributions (
  id           SERIAL      PRIMARY KEY,
  user_id      INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  issue_id     INTEGER     NOT NULL REFERENCES issues(id),
  pr_url       TEXT,
  pr_number    INTEGER,
  state        TEXT        NOT NULL DEFAULT 'STARTED'
    CHECK (state IN ('STARTED', 'PR_OPEN', 'PR_CLOSED', 'PR_MERGED', 'VERIFIED')),
  xp_awarded   BOOLEAN     NOT NULL DEFAULT false,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  merged_at    TIMESTAMPTZ,
  submitted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_contributions_user_issue_active
  ON contributions (user_id, issue_id) WHERE state = 'STARTED';
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
GRANT SELECT ON contributions, contribution_audit_log TO metrics_ro;

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
