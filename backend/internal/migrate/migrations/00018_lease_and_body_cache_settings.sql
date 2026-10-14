-- Operational values that were literals in the code become live settings,
-- and two keys that no code ever read are removed.
--
--   ingest.claim_interval_seconds          coordinator cycle (was 5s)
--   ingest.lease_seconds                   job lease (was 5 min)
--   ingest.lease_reclaim_interval_seconds  expired leases are now reclaimed
--                                          continuously, not only at startup
--   ingest.rate_limit_pause_seconds        lane pause after a GitHub rate
--                                          limit, and the retry delay for a
--                                          job released for budget (was 1 min)
--   api.issue_body.*                       issue-body cache cap and prune
--                                          cadence, fetch concurrency, and
--                                          the installation lane floor
--
-- ingest.sync_interval_seconds predates the coordinator, and
-- ingest.ondemand_workers was reserved for a pool the body read path never
-- needed (it uses api.issue_body.max_concurrent_fetches).
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.claim_interval_seconds', '5', 'int', 1, 300,
   'How often the ingest coordinator runs a cycle: enqueue, reclaim, claim and submit.'),
  ('ingest.lease_seconds', '300', 'int', 60, 3600,
   'How long a claimed ingest job may wait and run before its lease can be reclaimed. Must exceed the slowest page fetch plus queue wait.'),
  ('ingest.lease_reclaim_interval_seconds', '60', 'int', 5, 3600,
   'How often expired ingest job leases are returned to pending.'),
  ('ingest.rate_limit_pause_seconds', '60', 'int', 5, 3600,
   'How long the crawler lane pauses after a GitHub rate limit, and how long a job released for lack of budget waits.'),
  ('api.issue_body.cache_max_rows', '50000', 'int', 100, 10000000,
   'Rows kept in issue_body_cache; the least recently read are pruned beyond this.'),
  ('api.issue_body.prune_interval_seconds', '600', 'int', 60, 86400,
   'How often issue_body_cache is pruned back to api.issue_body.cache_max_rows.'),
  ('api.issue_body.max_concurrent_fetches', '8', 'int', 1, 64,
   'Concurrent GitHub fetches for issue bodies across all users.'),
  ('api.issue_body.installation_floor', '100', 'int', 0, 5000,
   'Remaining GraphQL points below which the installation token stops serving issue-body reads.')
ON CONFLICT (key) DO NOTHING;

DELETE FROM settings WHERE key IN ('ingest.sync_interval_seconds', 'ingest.ondemand_workers');

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.claim_interval_seconds',
  'ingest.lease_seconds',
  'ingest.lease_reclaim_interval_seconds',
  'ingest.rate_limit_pause_seconds',
  'api.issue_body.cache_max_rows',
  'api.issue_body.prune_interval_seconds',
  'api.issue_body.max_concurrent_fetches',
  'api.issue_body.installation_floor'
);

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.sync_interval_seconds', '900', 'int', 30, 86400,
   'Delay between routine sync sweeps.'),
  ('ingest.ondemand_workers', '1', 'int', 1, 64,
   'Concurrent on-demand sync workers.')
ON CONFLICT (key) DO NOTHING;
