-- ==============================================================================
-- SkillIssues PostgreSQL Database Schema — Production v2
--
-- Canonical schema used by Go-based golang-migrate migrations.
-- All ALTER TABLE retrofits from v1 have been collapsed into this definition.
-- New in v2:
--   - author TEXT column on issues (was missing in v1, causing upsert errors)
--   - CHECK constraints on difficulty, state, and level enum columns
--   - Indexes on github_id columns for upsert performance
-- ==============================================================================

-- ------------------------------------------------------------------------------
-- 1. Authentication & Users
-- ------------------------------------------------------------------------------

-- Core user account record linked to GitHub identity
CREATE TABLE IF NOT EXISTS users (
  id                  SERIAL       PRIMARY KEY,
  github_id           TEXT         UNIQUE,
  username            TEXT         NOT NULL,
  avatar_url          TEXT,
  github_access_token TEXT,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Active user sessions — token is a SHA-256 hash of the raw bearer token
CREATE TABLE IF NOT EXISTS auth_sessions (
  token      TEXT        PRIMARY KEY,
  user_id    INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider   TEXT        NOT NULL DEFAULT 'demo',
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '7 days'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ephemeral PKCE state for GitHub OAuth (expires in 10 minutes)
CREATE TABLE IF NOT EXISTS oauth_states (
  state        TEXT        PRIMARY KEY,
  code_verifier TEXT       NOT NULL,
  redirect_uri  TEXT       NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '10 minutes'),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ------------------------------------------------------------------------------
-- 2. Developer Profiles & Preferences
-- ------------------------------------------------------------------------------

-- Gamification state: XP, level, badges, skills
CREATE TABLE IF NOT EXISTS developer_profiles (
  user_id             INTEGER  PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  bio                 TEXT     DEFAULT '',
  experience_level    TEXT     NOT NULL DEFAULT 'BEGINNER'
    CHECK (experience_level IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED')),
  target_level        TEXT     NOT NULL DEFAULT 'INTERMEDIATE'
    CHECK (target_level IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED')),
  progress_score      INTEGER  NOT NULL DEFAULT 18,
  completed_count     INTEGER  NOT NULL DEFAULT 0,
  demonstrated_skills TEXT[]   NOT NULL DEFAULT '{}',
  total_xp            INTEGER  NOT NULL DEFAULT 0,
  level               INTEGER  NOT NULL DEFAULT 1,
  badges              TEXT[]   NOT NULL DEFAULT '{}'
);

-- Recommendation filters: interests and preferred programming languages
CREATE TABLE IF NOT EXISTS developer_preferences (
  user_id              INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  interests            TEXT[]  NOT NULL DEFAULT '{}',
  preferred_languages  TEXT[]  NOT NULL DEFAULT '{}'
);

-- ------------------------------------------------------------------------------
-- 3. Repositories & Issue Catalog
-- ------------------------------------------------------------------------------

-- Tracked public GitHub repositories sourced from Kaggle ETL and live GraphQL sync
CREATE TABLE IF NOT EXISTS repositories (
  id               SERIAL      PRIMARY KEY,
  github_id        BIGINT      UNIQUE,
  full_name        TEXT        NOT NULL UNIQUE,
  url              TEXT        NOT NULL,
  description      TEXT        DEFAULT '',
  language         TEXT,
  stars            INTEGER     NOT NULL DEFAULT 0,
  license          TEXT,
  forkable         BOOLEAN     NOT NULL DEFAULT true,
  last_activity_at TIMESTAMPTZ,
  last_synced_at   TIMESTAMPTZ
);

-- Curated open-source issues with difficulty scoring and derived skill tags
-- NOTE: issues are kept even when closed (closed state = excluded from recommendations)
CREATE TABLE IF NOT EXISTS issues (
  id                  SERIAL      PRIMARY KEY,
  github_id           BIGINT      UNIQUE,
  repository_id       INTEGER     NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  number              INTEGER     NOT NULL,
  title               TEXT        NOT NULL,
  body                TEXT        DEFAULT '',
  url                 TEXT        NOT NULL,
  state               TEXT        NOT NULL DEFAULT 'OPEN'
    CHECK (state IN ('OPEN', 'CLOSED')),
  difficulty_score    INTEGER     NOT NULL,
  difficulty          TEXT        NOT NULL
    CHECK (difficulty IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED')),
  required_skills     TEXT[]      NOT NULL DEFAULT '{}',
  technologies        TEXT[]      NOT NULL DEFAULT '{}',
  learning_tags       TEXT[]      NOT NULL DEFAULT '{}',
  comments            INTEGER     NOT NULL DEFAULT 0,
  has_difficulty_label BOOLEAN    NOT NULL DEFAULT false,
  author              TEXT,       -- GitHub login of issue author; nullable (deleted/unavailable accounts)
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(repository_id, number)
);

-- User bookmarked challenges
CREATE TABLE IF NOT EXISTS saved_issues (
  user_id    INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  issue_id   INTEGER     NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, issue_id)
);

-- ------------------------------------------------------------------------------
-- 4. Contributions & PR Verification
-- ------------------------------------------------------------------------------

-- Active and completed contributions linked to GitHub PRs
-- issue_id FK has no CASCADE: preserve history even if issue is removed
CREATE TABLE IF NOT EXISTS contributions (
  id          SERIAL      PRIMARY KEY,
  user_id     INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  issue_id    INTEGER     NOT NULL REFERENCES issues(id),
  pr_url      TEXT,
  pr_number   INTEGER,
  state       TEXT        NOT NULL DEFAULT 'STARTED'
    CHECK (state IN ('STARTED', 'PR_OPEN', 'PR_CLOSED', 'PR_MERGED', 'VERIFIED')),
  xp_awarded  BOOLEAN     NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  merged_at   TIMESTAMPTZ,
  UNIQUE(user_id, issue_id)
);

-- Immutable audit log: every contribution state transition recorded
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

-- ------------------------------------------------------------------------------
-- 5. Background Sync Pipeline & GitHub Rate Limit Tracking
-- ------------------------------------------------------------------------------

-- Per-repository sync state, scheduling priority, and GitHub rate limit data
CREATE TABLE IF NOT EXISTS sync_state (
  repository_id          INTEGER     PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  last_synced_at         TIMESTAMPTZ,
  last_issue_sync_at     TIMESTAMPTZ,   -- timestamp of last successful issue sync
  last_issue_update_seen TIMESTAMPTZ,   -- most recent updatedAt seen from GitHub
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

-- Deduplication log for GitHub webhook deliveries (prevents double-processing)
CREATE TABLE IF NOT EXISTS webhook_events (
  delivery_id  TEXT        PRIMARY KEY,
  event_name   TEXT        NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ------------------------------------------------------------------------------
-- 6. Performance Indexes
-- ------------------------------------------------------------------------------

-- Repository lookups
CREATE INDEX IF NOT EXISTS repositories_github_id_idx ON repositories(github_id);
CREATE INDEX IF NOT EXISTS repositories_language_idx  ON repositories(language);
CREATE INDEX IF NOT EXISTS repositories_stars_idx     ON repositories(stars DESC);

-- Issue catalog queries
CREATE INDEX IF NOT EXISTS issues_github_id_idx        ON issues(github_id);
CREATE INDEX IF NOT EXISTS issues_state_idx            ON issues(state);
CREATE INDEX IF NOT EXISTS issues_difficulty_idx       ON issues(difficulty);
CREATE INDEX IF NOT EXISTS issues_updated_idx          ON issues(updated_at DESC);
CREATE INDEX IF NOT EXISTS issues_repository_idx       ON issues(repository_id);

-- GIN indexes enable efficient array containment queries (@>) for skill/tech filtering
CREATE INDEX IF NOT EXISTS issues_required_skills_idx  ON issues USING GIN(required_skills);
CREATE INDEX IF NOT EXISTS issues_technologies_idx     ON issues USING GIN(technologies);
CREATE INDEX IF NOT EXISTS issues_learning_tags_idx    ON issues USING GIN(learning_tags);

-- Contribution & saved queries
CREATE INDEX IF NOT EXISTS contributions_user_idx      ON contributions(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS contributions_issue_idx     ON contributions(issue_id);
CREATE INDEX IF NOT EXISTS saved_issues_user_idx       ON saved_issues(user_id, created_at DESC);

-- Sync scheduler queue: pick lowest-priority, stalest repositories first
CREATE INDEX IF NOT EXISTS sync_state_priority_idx     ON sync_state(priority, last_synced_at NULLS FIRST);
CREATE INDEX IF NOT EXISTS sync_state_next_sync_idx    ON sync_state(next_sync_at);

-- Audit log lookups
CREATE INDEX IF NOT EXISTS audit_log_contribution_idx  ON contribution_audit_log(contribution_id, created_at DESC);
