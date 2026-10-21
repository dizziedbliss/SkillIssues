-- +goose Up
-- The repository's contributing guide on GitHub (its own CONTRIBUTING file, or
-- the owner's community-health default), read on the first page of each
-- crawl pass at no extra cost. NULL with metadata_refreshed_at set means
-- GitHub reported none; with metadata_refreshed_at NULL, not read yet.
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS contributing_url TEXT;

-- +goose Down
ALTER TABLE repositories DROP COLUMN IF EXISTS contributing_url;
