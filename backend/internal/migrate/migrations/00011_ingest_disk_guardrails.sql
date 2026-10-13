-- Two guardrails so the ingestion pipeline can never fill the host's disk:
-- a per-repository page cap (bounds one mega-repository's issue tracker)
-- and a whole-database size ceiling (bounds the catalog overall). Both are
-- live settings, not literals — raising or lowering either takes effect on
-- the coordinator's next cycle, no restart.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.max_pages_per_repo', '100', 'int', 0, 100000,
   'Pages the Deep Diver will fetch for one repository before stopping, regardless of hasNextPage. 0 disables the cap.'),
  ('ingest.max_db_size_mb', '30000', 'int', 0, 1000000,
   'pg_database_size() ceiling above which the coordinator stops enqueuing and claiming new ingestion work. 0 disables the cap.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.max_pages_per_repo',
  'ingest.max_db_size_mb'
);
