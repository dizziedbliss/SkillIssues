-- Both clients now offer OAuth only (Phase 1 of the client bug sweep). The
-- PAT login endpoint stays registered server-side but is gated behind this
-- live setting, default off, rather than removed outright.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, description) VALUES
  ('auth.pat_login_enabled', 'false', 'bool',
   'Whether POST /auth/github/token is reachable. Both clients only offer OAuth; this stays off unless explicitly turned on.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'auth.pat_login_enabled';
