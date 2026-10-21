-- +goose Up
-- Moderators review reports in the app (GET /moderation/reports, POST
-- /moderation/reports/resolve). Nothing in the API sets this: an operator
-- does, as the database owner:
--   UPDATE users SET moderator = true WHERE github_id = '<numeric id>';
ALTER TABLE users ADD COLUMN IF NOT EXISTS moderator BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS moderator;
