-- Live-tunable crawl/import eligibility thresholds (D1, D25): the star
-- floor is abuse control, not ranking, and both values should be
-- changeable without a restart or a new deploy.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.eligibility.min_stars', '25', 'int', 0, 100000,
   'Minimum stars for a repository to be eligible for ingestion. Abuse control, not ranking.'),
  ('ingest.eligibility.pushed_within_days', '730', 'int', 1, 3650,
   'A repository must have been pushed to within this many days to be eligible for ingestion.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN ('ingest.eligibility.min_stars', 'ingest.eligibility.pushed_within_days');
