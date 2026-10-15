-- Bound how long a request can wait on Redis. Without a deadline, a call to
-- an unreachable Redis waited out the go-redis dial timeout and its retries
-- (~20s) before failing open, so a Redis outage made every request take ~20s.
-- Now each call has a deadline, and after a failure Redis is skipped and
-- probed once per retry interval until it answers.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.cache.redis_timeout_ms', '250', 'int', 10, 5000,
   'Deadline for one Redis call, connecting included. A call that misses it fails open.'),
  ('api.cache.redis_retry_seconds', '5', 'int', 1, 300,
   'After a failed Redis call, how long Redis is skipped before one call probes whether it has recovered.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'api.cache.redis_timeout_ms',
  'api.cache.redis_retry_seconds'
);
