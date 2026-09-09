CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  github_id TEXT UNIQUE,
  username TEXT NOT NULL,
  avatar_url TEXT,
  github_access_token TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_access_token TEXT;
CREATE TABLE IF NOT EXISTS auth_sessions (
  token TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL DEFAULT 'demo',
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '7 days'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS oauth_states (
  state TEXT PRIMARY KEY,
  code_verifier TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '10 minutes'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS developer_profiles (
  user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  bio TEXT DEFAULT '',
  experience_level TEXT NOT NULL DEFAULT 'BEGINNER',
  target_level TEXT NOT NULL DEFAULT 'INTERMEDIATE',
  progress_score INTEGER NOT NULL DEFAULT 18,
  completed_count INTEGER NOT NULL DEFAULT 0,
  demonstrated_skills TEXT[] NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS developer_preferences (
  user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  interests TEXT[] NOT NULL DEFAULT '{}',
  preferred_languages TEXT[] NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS repositories (
  id SERIAL PRIMARY KEY,
  github_id BIGINT UNIQUE,
  full_name TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL,
  description TEXT DEFAULT '',
  language TEXT,
  stars INTEGER NOT NULL DEFAULT 0,
  license TEXT,
  forkable BOOLEAN NOT NULL DEFAULT true,
  last_activity_at TIMESTAMPTZ,
  last_synced_at TIMESTAMPTZ
);
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS issues (
  id SERIAL PRIMARY KEY,
  github_id BIGINT UNIQUE,
  repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  body TEXT DEFAULT '',
  url TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'OPEN',
  difficulty_score INTEGER NOT NULL,
  difficulty TEXT NOT NULL,
  required_skills TEXT[] NOT NULL DEFAULT '{}',
  technologies TEXT[] NOT NULL DEFAULT '{}',
  learning_tags TEXT[] NOT NULL DEFAULT '{}',
  comments INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(repository_id, number)
);
CREATE TABLE IF NOT EXISTS saved_issues (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, issue_id)
);
CREATE TABLE IF NOT EXISTS contributions (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  issue_id INTEGER NOT NULL REFERENCES issues(id),
  pr_url TEXT,
  pr_number INTEGER,
  state TEXT NOT NULL DEFAULT 'STARTED',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  merged_at TIMESTAMPTZ,
  UNIQUE(user_id, issue_id)
);
CREATE TABLE IF NOT EXISTS sync_state (
  repository_id INTEGER PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  last_synced_at TIMESTAMPTZ,
  priority TEXT NOT NULL DEFAULT 'MEDIUM',
  rate_limit_limit INTEGER,
  rate_limit_remaining INTEGER,
  rate_limit_used INTEGER,
  rate_limit_reset_at TIMESTAMPTZ,
  rate_limit_cost INTEGER
);
ALTER TABLE sync_state ADD COLUMN IF NOT EXISTS rate_limit_limit INTEGER;
ALTER TABLE sync_state ADD COLUMN IF NOT EXISTS rate_limit_used INTEGER;
ALTER TABLE sync_state ADD COLUMN IF NOT EXISTS rate_limit_cost INTEGER;
CREATE TABLE IF NOT EXISTS webhook_events (
  delivery_id TEXT PRIMARY KEY,
  event_name TEXT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS issues_difficulty_idx ON issues(difficulty);
CREATE INDEX IF NOT EXISTS issues_updated_idx ON issues(updated_at);
CREATE INDEX IF NOT EXISTS issues_state_idx ON issues(state);
CREATE INDEX IF NOT EXISTS repositories_language_idx ON repositories(language);
CREATE INDEX IF NOT EXISTS issues_repository_idx ON issues(repository_id);
CREATE INDEX IF NOT EXISTS issues_required_skills_idx ON issues USING GIN(required_skills);
CREATE INDEX IF NOT EXISTS issues_technologies_idx ON issues USING GIN(technologies);
CREATE INDEX IF NOT EXISTS contributions_user_idx ON contributions(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS saved_issues_user_idx ON saved_issues(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS sync_state_priority_idx ON sync_state(priority, last_synced_at);
