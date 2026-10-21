-- +goose Up
-- How a repository's latest merges went, read with its live stats on the
-- first page of each crawl pass: how many merged pull requests were read,
-- how many came from people outside the project (not its owner, an
-- organisation member or a collaborator), and the median hours from opening
-- to merging. NULL = not read yet.
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS health_sampled SMALLINT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS health_outside SMALLINT;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS health_median_merge_hours INTEGER;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.health.sample', '20', 'int', 0, 100,
   'Latest merged pull requests read per repository for its health; 0 stops reading them.'),
  ('api.health.min_sample', '5', 'int', 1, 100,
   'Merged pull requests a repository needs before its health is shown.'),
  ('api.health.welcoming_min', '3', 'int', 1, 100,
   'Merges from outside contributors, among those read, that mark a repository as merging outside contributions.'),
  ('api.health.slow_hours', '1440', 'int', 24, 87600,
   'Median hours to merge from which a repository is marked slow to merge.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN ('ingest.health.sample', 'api.health.min_sample', 'api.health.welcoming_min', 'api.health.slow_hours');
ALTER TABLE repositories DROP COLUMN IF EXISTS health_median_merge_hours;
ALTER TABLE repositories DROP COLUMN IF EXISTS health_outside;
ALTER TABLE repositories DROP COLUMN IF EXISTS health_sampled;
