-- D23 (the co-contribution commit trailer) is removed from both clients and
-- the API in the Phase 1 client bug sweep. Drop the now-unused preference
-- column it was the only reader/writer of.
--
-- +goose Up

ALTER TABLE developer_preferences DROP COLUMN IF EXISTS co_contribution_enabled;

-- +goose Down

ALTER TABLE developer_preferences ADD COLUMN IF NOT EXISTS co_contribution_enabled BOOLEAN NOT NULL DEFAULT false;
