-- A bounded cache of issue bodies, for clients that cannot fetch them.
--
-- Clients read an issue's body from GitHub themselves. A signed-out browser
-- gets 60 unauthenticated GitHub requests an hour, though, and past that the
-- body cannot load. GET /issues/{id}/body is the fallback: it answers from
-- this cache, and on a miss asks ingestd to fetch the body on the interactive
-- lane of the server's GitHub budget, capped per caller
-- (api.bodies.fallback_misses_per_hour).
--
--   issue_bodies   one body per issue. Bodies are attacker-controlled
--                  Markdown, rendered only through packages/markdown. The
--                  archive reader refreshes a body already here when its issue
--                  changes (source 'archive'); it never adds one. ingestd
--                  evicts the least recently read bodies once the table holds
--                  more than ingest.bodies.cache_max_mb, never one of an issue
--                  someone saved.
--
-- skillissues_api reads bodies and marks them read; only ingestd, which holds
-- the GitHub credential, writes them.

-- +goose Up

CREATE TABLE IF NOT EXISTS issue_bodies (
  issue_id     INTEGER     PRIMARY KEY REFERENCES issues(id) ON DELETE CASCADE,
  body         TEXT        NOT NULL,
  source       TEXT        NOT NULL CHECK (source IN ('fetch', 'archive')),
  fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_read_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS issue_bodies_last_read_idx ON issue_bodies (last_read_at);

GRANT SELECT, UPDATE                 ON issue_bodies TO skillissues_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON issue_bodies TO skillissues_ingest;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.bodies.fallback_misses_per_hour', '30', 'int', 0, 10000,
   'Issue bodies one caller may have fetched from GitHub per hour through GET /issues/{id}/body; cached bodies do not count. 0 serves the cache only.'),
  ('ingest.bodies.cache_max_mb', '512', 'int', 0, 100000,
   'Size the issue body cache is trimmed back to, least recently read first; bodies of saved issues are always kept. 0 keeps only those.'),
  ('ingest.bodies.prune_interval_seconds', '3600', 'int', 60, 86400,
   'How often the issue body cache is trimmed.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'api.bodies.fallback_misses_per_hour',
  'ingest.bodies.cache_max_mb',
  'ingest.bodies.prune_interval_seconds'
);

DROP TABLE IF EXISTS issue_bodies;
