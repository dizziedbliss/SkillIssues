-- Live catalog counts for the public GET /stats (the landing page's bar and
-- stat grid, and the flow site).
--
-- Counting open issues on demand is a scan of millions of rows, so the counts
-- are kept as they change. Like issues_notify_closed (00024), that is a
-- trigger's job rather than Go code's: the crawl, the archive, the
-- tracked-issue check and the webhook all write issues, and a count must not
-- depend on each remembering to.
--
-- The triggers are statement-level, over transition tables, and append one
-- row per repository a statement touched to catalog_stat_deltas. Nothing
-- updates a shared counter row on the write path, so concurrent writers never
-- contend. ingestd folds the deltas into catalog_stats, catalog_stats_daily
-- and catalog_activity every ingest.stats.rollup_seconds, and recounts the
-- exact totals every ingest.stats.recount_seconds in the same snapshot
-- (ingest/stats.go).
--
--   indexed  issues inserted (all-time; pruning does not lower it)
--   arrived  issues inserted open
--   opened   the change in open issues
--   closed   OPEN -> CLOSED transitions
--
-- catalog_stats keys: open_issues, issues_indexed, repositories_indexed
-- (eligible repositories) and closed_since (closed since tracking began,
-- the time in since).

-- +goose Up

CREATE TABLE IF NOT EXISTS catalog_stat_deltas (
  id            BIGSERIAL   PRIMARY KEY,
  at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  repository_id INTEGER     NOT NULL,
  indexed       INTEGER     NOT NULL DEFAULT 0,
  arrived       INTEGER     NOT NULL DEFAULT 0,
  opened        INTEGER     NOT NULL DEFAULT 0,
  closed        INTEGER     NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS catalog_stats (
  key        TEXT        PRIMARY KEY,
  value      BIGINT      NOT NULL,
  since      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS catalog_stats_daily (
  day     DATE    PRIMARY KEY,
  arrived INTEGER NOT NULL DEFAULT 0,
  closed  INTEGER NOT NULL DEFAULT 0
);

-- The most recent commits that brought issues in or saw them close, newest
-- first: the landing page's activity ticker.
CREATE TABLE IF NOT EXISTS catalog_activity (
  id            BIGSERIAL   PRIMARY KEY,
  at            TIMESTAMPTZ NOT NULL,
  repository_id INTEGER     NOT NULL,
  arrived       INTEGER     NOT NULL,
  closed        INTEGER     NOT NULL
);
CREATE INDEX IF NOT EXISTS catalog_activity_at_idx ON catalog_activity (at DESC, id DESC);

-- Closed since tracking began starts at zero, now.
INSERT INTO catalog_stats (key, value) VALUES ('closed_since', 0) ON CONFLICT (key) DO NOTHING;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION catalog_stats_inserted() RETURNS trigger AS $$
BEGIN
  INSERT INTO catalog_stat_deltas (repository_id, indexed, arrived, opened)
  SELECT repository_id, count(*), count(*) FILTER (WHERE state = 'OPEN'), count(*) FILTER (WHERE state = 'OPEN')
  FROM catalog_new GROUP BY repository_id;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION catalog_stats_updated() RETURNS trigger AS $$
BEGIN
  INSERT INTO catalog_stat_deltas (repository_id, opened, closed)
  SELECT n.repository_id,
         sum(CASE WHEN n.state = 'OPEN' THEN 1 WHEN o.state = 'OPEN' THEN -1 ELSE 0 END),
         count(*) FILTER (WHERE o.state = 'OPEN' AND n.state = 'CLOSED')
  FROM catalog_old o JOIN catalog_new n ON n.id = o.id
  WHERE o.state IS DISTINCT FROM n.state
  GROUP BY n.repository_id;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION catalog_stats_deleted() RETURNS trigger AS $$
BEGIN
  INSERT INTO catalog_stat_deltas (repository_id, opened)
  SELECT repository_id, -count(*)
  FROM catalog_old WHERE state = 'OPEN' GROUP BY repository_id;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS issues_catalog_stats_inserted ON issues;
CREATE TRIGGER issues_catalog_stats_inserted
  AFTER INSERT ON issues REFERENCING NEW TABLE AS catalog_new
  FOR EACH STATEMENT EXECUTE FUNCTION catalog_stats_inserted();

DROP TRIGGER IF EXISTS issues_catalog_stats_updated ON issues;
CREATE TRIGGER issues_catalog_stats_updated
  AFTER UPDATE ON issues REFERENCING OLD TABLE AS catalog_old NEW TABLE AS catalog_new
  FOR EACH STATEMENT EXECUTE FUNCTION catalog_stats_updated();

DROP TRIGGER IF EXISTS issues_catalog_stats_deleted ON issues;
CREATE TRIGGER issues_catalog_stats_deleted
  AFTER DELETE ON issues REFERENCING OLD TABLE AS catalog_old
  FOR EACH STATEMENT EXECUTE FUNCTION catalog_stats_deleted();

-- Triggers run as the invoker: both roles write issues (go-api's webhook
-- updates them), so both append deltas.
GRANT INSERT                         ON catalog_stat_deltas TO skillissues_api;
GRANT USAGE                          ON SEQUENCE catalog_stat_deltas_id_seq TO skillissues_api;
GRANT SELECT, INSERT, DELETE         ON catalog_stat_deltas TO skillissues_ingest;
GRANT USAGE                          ON SEQUENCE catalog_stat_deltas_id_seq TO skillissues_ingest;
GRANT SELECT                         ON catalog_stats, catalog_stats_daily, catalog_activity TO skillissues_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON catalog_stats, catalog_stats_daily, catalog_activity TO skillissues_ingest;
GRANT USAGE                          ON SEQUENCE catalog_activity_id_seq TO skillissues_ingest;
GRANT SELECT ON catalog_stat_deltas, catalog_stats, catalog_stats_daily, catalog_activity TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.stats.rollup_seconds', '5', 'int', 1, 3600,
   'How often ingestd folds the issue-change deltas into the live catalog counts behind GET /stats.'),
  ('ingest.stats.recount_seconds', '3600', 'int', 60, 86400,
   'How often ingestd recounts open issues and eligible repositories exactly, correcting any drift in the live counts.'),
  ('ingest.stats.activity_rows', '50', 'int', 0, 1000,
   'How many recent crawl and archive commits GET /stats lists as activity.'),
  ('api.stats.max_age_seconds', '10', 'int', 0, 3600,
   'Public cache lifetime of GET /stats, at the edge and in Redis.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP TRIGGER IF EXISTS issues_catalog_stats_inserted ON issues;
DROP TRIGGER IF EXISTS issues_catalog_stats_updated ON issues;
DROP TRIGGER IF EXISTS issues_catalog_stats_deleted ON issues;
DROP FUNCTION IF EXISTS catalog_stats_inserted();
DROP FUNCTION IF EXISTS catalog_stats_updated();
DROP FUNCTION IF EXISTS catalog_stats_deleted();
DROP TABLE IF EXISTS catalog_activity;
DROP TABLE IF EXISTS catalog_stats_daily;
DROP TABLE IF EXISTS catalog_stats;
DROP TABLE IF EXISTS catalog_stat_deltas;
DELETE FROM settings WHERE key IN ('ingest.stats.rollup_seconds', 'ingest.stats.recount_seconds',
  'ingest.stats.activity_rows', 'api.stats.max_age_seconds');
