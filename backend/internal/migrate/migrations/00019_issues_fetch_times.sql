-- When SkillIssues itself saw an issue, as opposed to GitHub's own
-- created_at/updated_at (which an unedited issue shares, so they say nothing
-- about our crawl).
--
--   first_seen_at  — when the row was first inserted; never changes after.
--   last_synced_at — the last time GitHub data was written to the row (a
--                    crawl page, an on-demand freshness check or a webhook).
--
-- Rows that predate this migration stay NULL: when they were fetched is
-- unknown. repositories.last_synced_at is not a substitute — it only records
-- the repository's latest page, and a freshness pass re-fetches page one, not
-- the deeper pages most issues came from.
--
-- issues is granted to metrics_ro at table level (00017), so the new columns
-- are readable there without a new grant.
--
-- +goose Up

ALTER TABLE issues ADD COLUMN IF NOT EXISTS first_seen_at  TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

-- Applies to rows inserted from now on only; existing rows keep NULL.
ALTER TABLE issues ALTER COLUMN first_seen_at  SET DEFAULT now();
ALTER TABLE issues ALTER COLUMN last_synced_at SET DEFAULT now();

-- +goose Down

ALTER TABLE issues DROP COLUMN IF EXISTS last_synced_at;
ALTER TABLE issues DROP COLUMN IF EXISTS first_seen_at;
