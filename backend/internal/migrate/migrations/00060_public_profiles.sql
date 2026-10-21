-- +goose Up
-- Public profiles are opt-in: off until the person turns it on. Only people
-- who turned it on appear at /users/{login} and on the leaderboard, which
-- itself is off until api.leaderboards.enabled.
ALTER TABLE users ADD COLUMN IF NOT EXISTS public_profile BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS users_public_idx ON users (id) WHERE public_profile;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.leaderboards.enabled', 'false', 'bool', NULL, NULL,
   'Serve GET /leaderboards. People who keep their profile private are never listed.'),
  ('api.leaderboards.size', '50', 'int', 1, 500, 'People one leaderboard lists.'),
  ('api.profiles.public_pulls', '20', 'int', 0, 200, 'Merged pull requests a public profile lists.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN ('api.leaderboards.enabled', 'api.leaderboards.size', 'api.profiles.public_pulls');
DROP INDEX IF EXISTS users_public_idx;
ALTER TABLE users DROP COLUMN IF EXISTS public_profile;
