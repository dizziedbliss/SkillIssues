-- Catalog filters, the Newest sort and the shuffled feed.
--
-- GET /issues takes multi-value filters (labels, tech, topics, languages,
-- difficulty, a comment-count range) and a sort: shuffle (the default when
-- browsing by cursor), easiest or newest.
--
--   repositories.shuffle_key  a random position per repository. The shuffled
--                             feed walks repositories in this order and shows
--                             each one's best open issue, so a repository with
--                             thousands of issues gets the same one slot as
--                             one with three. ingestd re-randomises it every
--                             ingest.catalog.shuffle_interval_seconds, so
--                             each repository spends time near the top; the
--                             default gives a new repository its own place
--                             at once.
--   repositories.tech         the lower-cased names of every language GitHub
--                             reports in the repository (from languages), for
--                             the tech filter; GIN-indexed, as topics is.
--   issues_open_newest_idx    the Newest sort: open issues by created_at.
--   issues_open_repo_browse_idx
--                             each repository's open issues in catalog order.
--                             The shuffled feed and search's repository-led
--                             categories ask every repository for its best
--                             issue (or few); without it a large repository's
--                             issues are all read and sorted to find one
--                             (a filtered shuffle page: 4.1 s without, 0.3 s
--                             with, on 2M issues).
--   issues_open_labels_lower_idx
--                             the labels filter, case-insensitively: labels
--                             are stored as GitHub has them ("Bug", "bug"),
--                             so the filter matches catalog_lower_labels().
--   catalog_facets            the values the filter dropdowns offer, with how
--                             many open issues each has: labels (grouped
--                             case-insensitively, shown in their commonest
--                             spelling), topics, tech and languages.
--                             Counting them over every issue is too heavy for
--                             a request; ingestd refreshes it every
--                             ingest.catalog.facets_refresh_seconds.
--   catalog_refresh           when each of those periodic jobs last ran, so a
--                             restart does not rerun them early.
--
-- Settings:
--   ingest.catalog.shuffle_interval_seconds
--   ingest.catalog.facets_refresh_seconds
--   ingest.catalog.facets_max_values   values kept per facet kind
--   api.issues.filter_max_values       values one filter may name
--
-- Runs outside a transaction so the issues index is built CONCURRENTLY. A
-- failed concurrent build leaves an INVALID index that IF NOT EXISTS would
-- skip: drop it first. The repositories changes rewrite that table (about
-- 350k rows, seconds) under a brief exclusive lock.
--
-- +goose NO TRANSACTION
-- +goose Up

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS shuffle_key INTEGER NOT NULL
  DEFAULT floor(random() * 2147483647)::integer;
CREATE INDEX IF NOT EXISTS repositories_shuffle_idx ON repositories (shuffle_key, id);

CREATE OR REPLACE FUNCTION catalog_tech(languages JSONB) RETURNS TEXT[]
  LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(array_agg(DISTINCT lower(e->>'name')) FILTER (WHERE e->>'name' <> ''), '{}')
    FROM jsonb_array_elements(CASE WHEN jsonb_typeof(languages) = 'array' THEN languages ELSE '[]'::jsonb END) e
  $$;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS tech TEXT[]
  GENERATED ALWAYS AS (catalog_tech(languages)) STORED;
CREATE INDEX IF NOT EXISTS repositories_tech_idx   ON repositories USING GIN (tech);
CREATE INDEX IF NOT EXISTS repositories_topics_idx ON repositories USING GIN (topics);

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_newest_idx
  ON issues (created_at DESC, id DESC)
  WHERE state = 'OPEN';

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_repo_browse_idx
  ON issues (repository_id, difficulty_score, updated_at DESC, id DESC)
  WHERE state = 'OPEN';

CREATE OR REPLACE FUNCTION catalog_lower_labels(labels TEXT[]) RETURNS TEXT[]
  LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(array_agg(lower(l)), '{}') FROM unnest(labels) l
  $$;
CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_labels_lower_idx
  ON issues USING GIN (catalog_lower_labels(labels))
  WHERE state = 'OPEN';

CREATE TABLE IF NOT EXISTS catalog_facets (
  kind     TEXT    NOT NULL CHECK (kind IN ('label', 'topic', 'tech', 'language')),
  value    TEXT    NOT NULL,
  issues   INTEGER NOT NULL,
  PRIMARY KEY (kind, value)
);

CREATE TABLE IF NOT EXISTS catalog_refresh (
  name         TEXT        PRIMARY KEY,
  refreshed_at TIMESTAMPTZ NOT NULL
);

GRANT SELECT                         ON catalog_facets, catalog_refresh TO skillissues_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON catalog_facets, catalog_refresh TO skillissues_ingest;
GRANT SELECT                         ON catalog_facets, catalog_refresh TO metrics_ro;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.catalog.shuffle_interval_seconds', '86400', 'int', 3600, 2592000,
   'How often the shuffled feed''s repository order is re-randomised.'),
  ('ingest.catalog.facets_refresh_seconds', '86400', 'int', 3600, 2592000,
   'How often the filter dropdowns'' values and counts (catalog_facets) are recomputed.'),
  ('ingest.catalog.facets_max_values', '300', 'int', 10, 5000,
   'Values kept per filter (labels, topics, tech, languages), the most common first.'),
  ('api.issues.filter_max_values', '20', 'int', 1, 100,
   'Most values one GET /issues filter may name; more is a 400.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.catalog.shuffle_interval_seconds',
  'ingest.catalog.facets_refresh_seconds',
  'ingest.catalog.facets_max_values',
  'api.issues.filter_max_values'
);

DROP TABLE IF EXISTS catalog_refresh;
DROP TABLE IF EXISTS catalog_facets;
DROP INDEX CONCURRENTLY IF EXISTS issues_open_labels_lower_idx;
DROP INDEX CONCURRENTLY IF EXISTS issues_open_repo_browse_idx;
DROP FUNCTION IF EXISTS catalog_lower_labels(TEXT[]);
DROP INDEX CONCURRENTLY IF EXISTS issues_open_newest_idx;
DROP INDEX IF EXISTS repositories_topics_idx;
DROP INDEX IF EXISTS repositories_tech_idx;
ALTER TABLE repositories DROP COLUMN IF EXISTS tech;
DROP FUNCTION IF EXISTS catalog_tech(JSONB);
DROP INDEX IF EXISTS repositories_shuffle_idx;
ALTER TABLE repositories DROP COLUMN IF EXISTS shuffle_key;
