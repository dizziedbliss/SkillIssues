-- +goose Up
-- Claims: "I'm working on this", for other SkillIssues users to see. A claim
-- lives on SkillIssues only and never touches GitHub. It is active until it
-- is released, or until api.claims.days pass with no open pull request of the
-- claimer's that closes the issue (internal/claim states the rule once).

CREATE TABLE IF NOT EXISTS issue_claims (
  id             BIGSERIAL PRIMARY KEY,
  issue_id       INTEGER     NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  user_id        INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  claimed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at     TIMESTAMPTZ NOT NULL,
  released_at    TIMESTAMPTZ,
  release_reason TEXT        CHECK (release_reason IN ('released', 'expired')),
  CHECK ((released_at IS NULL) = (release_reason IS NULL))
);
-- One unreleased claim per issue: an expired one is released (as 'expired')
-- in the transaction that claims the issue again.
CREATE UNIQUE INDEX IF NOT EXISTS issue_claims_one_open ON issue_claims (issue_id) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS issue_claims_user_open ON issue_claims (user_id) WHERE released_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON issue_claims TO skillissues_api;
GRANT USAGE ON SEQUENCE issue_claims_id_seq TO skillissues_api;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.claims.days', '7', 'int', 1, 60,
   'Days a claim lasts without an open pull request of the claimer''s that closes the issue.'),
  ('api.claims.max_active', '2', 'int', 1, 20,
   'Claims one person may hold at once.'),
  ('api.claims.refresh_seconds', '30', 'int', 5, 3600,
   'How often go-api re-reads which issues are claimed, for lists.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN ('api.claims.days', 'api.claims.max_active', 'api.claims.refresh_seconds');
DROP TABLE IF EXISTS issue_claims;
