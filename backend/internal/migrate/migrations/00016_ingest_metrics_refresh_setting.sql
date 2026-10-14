-- ingestd's /metrics gauges (ingest_jobs by type/status, repositories,
-- github_budget) are read from Postgres on their own cadence rather than on
-- every 5s claim cycle.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.metrics_refresh_seconds', '30', 'int', 5, 3600,
   'How often ingestd refreshes its queue, repository and GitHub budget gauges from Postgres for /metrics.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'ingest.metrics_refresh_seconds';
