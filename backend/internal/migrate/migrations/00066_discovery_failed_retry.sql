-- +goose Up
-- A discovery sweep that ended failed is reopened after this long: its failed
-- windows are searched again (ingest.RetryFailedSweep). Before, a failed full
-- sweep waited out ingest.discovery.full_interval_seconds, since the
-- incremental sweep is off while the GH Archive reader is current.

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.discovery.failed_retry_seconds', '86400', 'int', 3600, 2592000,
   'How long after a sweep ends failed its failed windows are searched again, rather than waiting for the next full sweep.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key = 'ingest.discovery.failed_retry_seconds';
