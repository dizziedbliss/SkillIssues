-- +goose Up
-- Pull request tracking and the XP ledger. ingestd reads each person's public
-- pull requests with the server's credential (no user token), keeps the ones
-- to catalog repositories, and pays XP for merges into an append-only ledger
-- that levels, badges, streaks and boards are computed from.

CREATE TABLE IF NOT EXISTS pull_requests (
  id                 BIGSERIAL PRIMARY KEY,
  github_id          BIGINT      NOT NULL UNIQUE,
  user_id            INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  repository_id      INTEGER     NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  number             INTEGER     NOT NULL,
  title              TEXT        NOT NULL,
  url                TEXT        NOT NULL,
  state              TEXT        NOT NULL CHECK (state IN ('OPEN', 'CLOSED', 'MERGED')),
  created_at         TIMESTAMPTZ NOT NULL,
  updated_at         TIMESTAMPTZ NOT NULL,
  merged_at          TIMESTAMPTZ,
  closed_at          TIMESTAMPTZ,
  -- GitHub's authorAssociation when last read: OWNER, MEMBER, COLLABORATOR,
  -- CONTRIBUTOR, FIRST_TIME_CONTRIBUTOR, FIRST_TIMER, NONE.
  author_association TEXT        NOT NULL,
  additions          INTEGER     NOT NULL DEFAULT 0,
  deletions          INTEGER     NOT NULL DEFAULT 0,
  -- The last commit's check rollup (SUCCESS, FAILURE, PENDING, ERROR,
  -- EXPECTED); NULL when the repository runs no checks on it.
  checks             TEXT,
  approvals          INTEGER     NOT NULL DEFAULT 0,
  changes_requested  INTEGER     NOT NULL DEFAULT 0,
  -- Why this pull request earns nothing; NULL when it can.
  no_xp_reason       TEXT        CHECK (no_xp_reason IN ('own_repository', 'organisation_member')),
  first_seen_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_synced_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS pull_requests_user_idx ON pull_requests (user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS pull_requests_user_open_idx ON pull_requests (user_id, updated_at) WHERE state = 'OPEN';
CREATE INDEX IF NOT EXISTS pull_requests_repository_idx ON pull_requests (repository_id);

-- The catalog issues a pull request says it closes (closingIssuesReferences).
CREATE TABLE IF NOT EXISTS pull_request_issues (
  pull_request_id BIGINT  NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
  issue_id        INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  PRIMARY KEY (pull_request_id, issue_id)
);
CREATE INDEX IF NOT EXISTS pull_request_issues_issue_idx ON pull_request_issues (issue_id);

-- One row per person: where their pull requests have been read up to.
CREATE TABLE IF NOT EXISTS pr_sync (
  user_id         INTEGER     PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  -- Newest updatedAt of the last completed pass; NULL = never completed, so
  -- the next pass reads everything (the backfill).
  synced_through  TIMESTAMPTZ,
  -- A pass in flight: the cursor of its next page, and the newest updatedAt
  -- it has seen, which becomes synced_through when it completes.
  pass_cursor     TEXT,
  pass_high       TIMESTAMPTZ,
  next_check_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_checked_at TIMESTAMPTZ,
  attempts        INTEGER     NOT NULL DEFAULT 0,
  last_error      TEXT,
  -- The last "check my pull requests now", which api.prs.refresh_min_seconds
  -- spaces out.
  requested_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS pr_sync_due_idx ON pr_sync (next_check_at);

-- The ledger. Never updated: a correction is another row.
CREATE TABLE IF NOT EXISTS xp_events (
  id              BIGSERIAL PRIMARY KEY,
  user_id         INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source          TEXT        NOT NULL CHECK (source IN ('merged', 'backfill', 'opened', 'opened_reversed', 'quest', 'path')),
  pull_request_id BIGINT      REFERENCES pull_requests(id) ON DELETE CASCADE,
  key             TEXT,
  amount          INTEGER     NOT NULL,
  -- How the amount was reached (base, bonuses, caps) and what it was for
  -- (difficulty, language, backlog), so badges never re-read pruned issues.
  detail          JSONB       NOT NULL DEFAULT '{}',
  -- When the thing it pays for happened (a merge, an opening, a quest done):
  -- streaks and monthly boards count by this, not by when it was written.
  earned_at       TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (pull_request_id IS NOT NULL OR key IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS xp_events_pr_once ON xp_events (user_id, source, pull_request_id) WHERE pull_request_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS xp_events_key_once ON xp_events (user_id, source, key) WHERE key IS NOT NULL;
CREATE INDEX IF NOT EXISTS xp_events_user_idx ON xp_events (user_id, earned_at);
CREATE INDEX IF NOT EXISTS xp_events_earned_idx ON xp_events (earned_at);

-- A merge notifies the person, written by the database like issue_closed.
ALTER TABLE notifications ALTER COLUMN issue_id DROP NOT NULL;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS pull_request_id BIGINT REFERENCES pull_requests(id) ON DELETE CASCADE;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (
  (kind = 'issue_closed' AND issue_id IS NOT NULL) OR
  (kind = 'pr_merged' AND pull_request_id IS NOT NULL));
CREATE UNIQUE INDEX IF NOT EXISTS notifications_pr_once ON notifications (user_id, kind, pull_request_id) WHERE pull_request_id IS NOT NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_pr_merged() RETURNS trigger AS $$
BEGIN
  INSERT INTO notifications (user_id, kind, pull_request_id)
  VALUES (NEW.user_id, 'pr_merged', NEW.pull_request_id)
  ON CONFLICT (user_id, kind, pull_request_id) WHERE pull_request_id IS NOT NULL DO NOTHING;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS xp_events_notify_merged ON xp_events;
CREATE TRIGGER xp_events_notify_merged
  AFTER INSERT ON xp_events
  FOR EACH ROW
  WHEN (NEW.source = 'merged')
  EXECUTE FUNCTION notify_pr_merged();

-- ingestd reads who to track (only what it needs) and writes the rest.
GRANT SELECT (id, github_id, username, created_at) ON users TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE         ON pull_requests       TO skillissues_ingest;
GRANT SELECT, INSERT, DELETE         ON pull_request_issues TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE         ON pr_sync             TO skillissues_ingest;
GRANT SELECT, INSERT                 ON xp_events           TO skillissues_ingest;
GRANT USAGE ON SEQUENCE pull_requests_id_seq, xp_events_id_seq TO skillissues_ingest;
-- go-api shows them, and makes a person due now on request.
GRANT SELECT                         ON pull_requests, pull_request_issues, xp_events TO skillissues_api;
GRANT SELECT, INSERT, UPDATE         ON pr_sync             TO skillissues_api;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.prs.enabled', 'true', 'bool', NULL, NULL,
   'Read signed-up people''s public pull requests to catalog repositories and pay XP for merges (interactive lane).'),
  ('ingest.prs.users_per_cycle', '50', 'int', 1, 1000,
   'People whose pull requests are read per coordinator cycle.'),
  ('ingest.prs.users_per_query', '10', 'int', 1, 50,
   'People read in one GraphQL request (one alias each).'),
  ('ingest.prs.page_size', '20', 'int', 1, 100,
   'Pull requests per person per request, newest-updated first.'),
  ('ingest.prs.hot_interval_seconds', '900', 'int', 60, 86400,
   'How often someone with an open pull request updated within ingest.prs.hot_window_days is read.'),
  ('ingest.prs.warm_interval_seconds', '3600', 'int', 60, 604800,
   'How often someone with an older open pull request is read.'),
  ('ingest.prs.cold_interval_seconds', '86400', 'int', 300, 2592000,
   'How often someone with no open pull request is read.'),
  ('ingest.prs.hot_window_days', '7', 'int', 1, 90,
   'An open pull request updated this recently makes its author hot.'),
  ('ingest.prs.retry_base_seconds', '300', 'int', 30, 86400,
   'First wait after a failed read; doubles per attempt, up to ingest.prs.cold_interval_seconds.'),
  ('api.prs.refresh_min_seconds', '300', 'int', 0, 86400,
   'Shortest gap between two "check my pull requests now" requests from one person.'),
  ('api.prs.page_size', '50', 'int', 1, 500,
   'Tracked pull requests GET /me/pulls returns.'),
  ('api.xp.recent_limit', '20', 'int', 1, 200,
   'Latest XP events GET /me/xp returns.'),
  ('xp.base_beginner', '75', 'int', 0, 10000, 'XP for a merge that closes a beginner catalog issue.'),
  ('xp.base_intermediate', '140', 'int', 0, 10000, 'XP for a merge that closes an intermediate catalog issue.'),
  ('xp.base_advanced', '250', 'int', 0, 10000, 'XP for a merge that closes an advanced catalog issue.'),
  ('xp.base_unclassified', '100', 'int', 0, 10000, 'XP for a merge that closes a catalog issue of unknown difficulty.'),
  ('xp.base_unlinked', '50', 'int', 0, 10000, 'XP for a merge to a catalog repository that closes no catalog issue.'),
  ('xp.first_merge_repo_bonus', '15', 'int', 0, 10000, 'Extra XP for someone''s first merge in a repository.'),
  ('xp.backlog_bonus', '50', 'int', 0, 10000, 'Extra XP for closing a catalog issue that had been open xp.backlog_days or more when it was fixed.'),
  ('xp.backlog_days', '180', 'int', 1, 3650, 'Age at which an issue counts as neglected backlog.'),
  ('xp.ci_bonus', '15', 'int', 0, 10000, 'Extra XP when the merged pull request''s checks passed (repositories with checks).'),
  ('xp.approval_bonus', '10', 'int', 0, 10000, 'Extra XP when a maintainer approved it, in a repository with checks.'),
  ('xp.approval_bonus_no_ci', '20', 'int', 0, 10000, 'Extra XP when a maintainer approved it, in a repository without checks.'),
  ('xp.clean_merge_bonus', '5', 'int', 0, 10000, 'Extra XP when it merged with no changes requested, in a repository with checks.'),
  ('xp.clean_merge_bonus_no_ci', '10', 'int', 0, 10000, 'Extra XP when it merged with no changes requested, in a repository without checks.'),
  ('xp.backfill_percent', '50', 'int', 0, 100, 'Share of the XP paid for merges from before someone signed up.'),
  ('xp.repo_daily_cap', '400', 'int', 0, 100000, 'Most merge XP one person earns in one repository in one day (UTC); 0 = no cap.'),
  ('xp.opened', '5', 'int', 0, 1000, 'XP for opening a pull request that closes a catalog issue, once per issue; reversed if it closes unmerged.'),
  ('xp.opened_weekly_cap', '3', 'int', 0, 100, 'Most opened-pull-request rewards per person per week (UTC ISO week).'),
  ('xp.level_step', '100', 'int', 1, 100000, 'Level L starts at xp.level_step x L(L-1)/2 XP: 100, 300, 600, 1000 ...')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key LIKE 'xp.%' OR key LIKE 'ingest.prs.%' OR key IN ('api.prs.refresh_min_seconds', 'api.prs.page_size', 'api.xp.recent_limit');
DROP TRIGGER IF EXISTS xp_events_notify_merged ON xp_events;
DROP FUNCTION IF EXISTS notify_pr_merged();
DELETE FROM notifications WHERE kind = 'pr_merged';
DROP INDEX IF EXISTS notifications_pr_once;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
ALTER TABLE notifications DROP COLUMN IF EXISTS pull_request_id;
ALTER TABLE notifications ALTER COLUMN issue_id SET NOT NULL;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind = 'issue_closed');
REVOKE SELECT (id, github_id, username, created_at) ON users FROM skillissues_ingest;
DROP TABLE IF EXISTS xp_events;
DROP TABLE IF EXISTS pr_sync;
DROP TABLE IF EXISTS pull_request_issues;
DROP TABLE IF EXISTS pull_requests;
