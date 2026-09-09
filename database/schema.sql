CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  github_id TEXT UNIQUE,
  username TEXT NOT NULL,
  avatar_url TEXT,
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
  last_synced_at TIMESTAMPTZ
);
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
  rate_limit_remaining INTEGER,
  rate_limit_reset_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS issues_difficulty_idx ON issues(difficulty);
CREATE INDEX IF NOT EXISTS issues_updated_idx ON issues(updated_at);
CREATE INDEX IF NOT EXISTS issues_state_idx ON issues(state);
