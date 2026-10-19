-- Fast catalog browsing.
--
-- GET /issues without a search term lists open issues ordered by
-- difficulty_score, then most recently updated, then id. No index matched
-- that order, so every cache miss sorted every open issue to return a page:
-- 35 seconds on the production catalog, past the server's write timeout,
-- which Caddy reported as a 502.
--
--   issues_open_browse_idx             the unfiltered order, and the keyset
--                                      cursor that pages through it
--   issues_open_difficulty_browse_idx  the same order within one difficulty,
--                                      for ?difficulty=; without it a filter
--                                      on UNCLASSIFIED (most open issues,
--                                      sorted last) walks every classified
--                                      issue first
--   repositories_lower_language_idx    lower(language), which ?language= and
--                                      preferred languages compare on; mostly
--                                      for its statistics, without which the
--                                      planner cannot tell a rare language
--                                      from a common one and walks the whole
--                                      browse index looking for a few matches
--                                      (4.9s for a language with no issues,
--                                      0.2ms with it)
--
-- id is the final key so a cursor names exactly one position even when
-- several issues share a score and an updated_at.
--
--   api.issues.query_timeout_ms     deadline for one catalog or search
--                                   request's queries, below the server's
--                                   30s write timeout so a slow query ends in
--                                   a 503 the client can retry rather than a
--                                   dropped connection
--   api.issues.rare_language_threshold
--                                   below this many open issues, browsing a
--                                   language sorts its issues instead of
--                                   walking the catalog index to find them;
--                                   the planner cannot tell (see
--                                   issue.browseOrder)
--   api.issues.language_count_ttl_seconds
--                                   how long that count is reused
--   api.cache.public_max_age_seconds   Cache-Control max-age on responses to
--                                      signed-out callers (0 = no-cache)
--   api.cache.public_stale_seconds     stale-while-revalidate on the same
--
-- Runs outside a transaction so the indexes are built CONCURRENTLY: a plain
-- CREATE INDEX would block every issue write for a scan of the whole table.
-- Every statement is idempotent, so a partial run can be repeated — except
-- that a failed concurrent build leaves an INVALID index behind, which IF NOT
-- EXISTS would then skip: drop it first.
--
-- +goose NO TRANSACTION
-- +goose Up

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_browse_idx
  ON issues (difficulty_score, updated_at DESC, id DESC)
  WHERE state = 'OPEN';

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_difficulty_browse_idx
  ON issues (difficulty, difficulty_score, updated_at DESC, id DESC)
  WHERE state = 'OPEN';

CREATE INDEX CONCURRENTLY IF NOT EXISTS repositories_lower_language_idx
  ON repositories (lower(language));

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.issues.query_timeout_ms', '10000', 'int', 100, 25000,
   'Deadline for the queries behind one GET /issues or GET /issues/{id}. A query that misses it is cancelled and the caller gets 503.'),
  ('api.issues.rare_language_threshold', '20000', 'int', 0, 1000000,
   'A language with fewer open issues than this is read repository-first and sorted when browsing, instead of walking the catalog index. 0 always walks the index.'),
  ('api.issues.language_count_ttl_seconds', '600', 'int', 0, 86400,
   'How long a language''s counted open issues are reused when deciding how to browse it. 0 counts on every request.'),
  ('api.cache.public_max_age_seconds', '60', 'int', 0, 3600,
   'Cache-Control max-age on catalog and search responses to signed-out callers. 0 sends no-cache.'),
  ('api.cache.public_stale_seconds', '300', 'int', 0, 86400,
   'Cache-Control stale-while-revalidate on catalog and search responses to signed-out callers.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'api.issues.query_timeout_ms',
  'api.issues.rare_language_threshold',
  'api.issues.language_count_ttl_seconds',
  'api.cache.public_max_age_seconds',
  'api.cache.public_stale_seconds'
);

DROP INDEX CONCURRENTLY IF EXISTS repositories_lower_language_idx;
DROP INDEX CONCURRENTLY IF EXISTS issues_open_difficulty_browse_idx;
DROP INDEX CONCURRENTLY IF EXISTS issues_open_browse_idx;
