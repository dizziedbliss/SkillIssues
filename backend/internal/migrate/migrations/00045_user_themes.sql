-- User themes: a TOML file of design tokens and an optional stylesheet that
-- restyle the signed-in app, BetterDiscord-style (internal/theme). A theme
-- belongs to its author and, once public, can be applied by anyone with its
-- id. users.active_theme_id is the theme an account has applied — its own or
-- someone's public one; deleting a theme, or making it private, clears it for
-- everyone else (the API does the latter).
--
-- Content is validated on the way in, never trusted on the way out: the web
-- app re-checks it before applying it, and its CSP blocks fetches either way.

-- +goose Up

CREATE TABLE IF NOT EXISTS user_themes (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT        NOT NULL,
  toml       TEXT        NOT NULL,
  css        TEXT        NOT NULL DEFAULT '',
  public     BOOLEAN     NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS user_themes_user_idx ON user_themes (user_id, updated_at DESC);

ALTER TABLE users ADD COLUMN IF NOT EXISTS active_theme_id UUID REFERENCES user_themes(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS users_active_theme_idx ON users (active_theme_id) WHERE active_theme_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON user_themes TO skillissues_api;
-- Bookkeeping only: what a theme says is the author's, not a metric.
GRANT SELECT (id, user_id, public, created_at, updated_at) ON user_themes TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.themes.max_per_user', '20', 'int', 1, 200,
   'Most themes one account may keep.'),
  ('api.themes.max_bytes', '65536', 'int', 1024, 524288,
   'Largest theme, TOML and stylesheet together, in bytes.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP INDEX IF EXISTS users_active_theme_idx;
ALTER TABLE users DROP COLUMN IF EXISTS active_theme_id;
DROP TABLE IF EXISTS user_themes;
DELETE FROM settings WHERE key IN ('api.themes.max_per_user', 'api.themes.max_bytes');
