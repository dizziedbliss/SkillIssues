-- A built-in theme applied as it is (Settings → Themes → Use), without a copy
-- in the account's own themes: built-ins are not editable, only their copies
-- are. One account has at most one of active_theme_id (its own or a public
-- theme) and active_preset (a built-in's name, checked against
-- theme.Presets by the API).

-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS active_preset TEXT;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_one_active_theme;
ALTER TABLE users ADD CONSTRAINT users_one_active_theme
  CHECK (active_theme_id IS NULL OR active_preset IS NULL);

-- +goose Down

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_one_active_theme;
ALTER TABLE users DROP COLUMN IF EXISTS active_preset;
