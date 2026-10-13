-- GitHub App user-to-server tokens expire in 8 hours and rotate via a
-- refresh token (~182-day lifetime), unlike the classic OAuth App token
-- users.github_access_token previously always held. Both the access token
-- and refresh token are encrypted the same way (see internal/crypto).
--
-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS github_token_expires_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_refresh_token TEXT;

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS github_refresh_token;
ALTER TABLE users DROP COLUMN IF EXISTS github_token_expires_at;
