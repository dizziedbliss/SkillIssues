-- Closed-issue cleanup, and live repository metadata from the crawl.
--
-- Closed issues are deleted once they have been closed on GitHub for
-- ingest.closed_issue_retention_days, unless someone saved them or has a
-- contribution on them — those are the user's record and stay forever.
-- issues_closed_idx keeps that delete off a scan of the whole table: it
-- covers only CLOSED rows, a small fraction.
--
-- The first page of every pass now also returns the repository's live
-- stars, forks, watchers, open issue count, archived flag and last push, and
-- the crawler writes them over the dataset snapshot's values.
--
--   repositories.open_issue_count       open issues on GitHub right now. Not
--                                       issue_count, which is the dataset's
--                                       open+closed total and stays as is.
--   repositories.metadata_refreshed_at  when those columns last came from
--                                       GitHub; NULL = still the snapshot.
--
-- Runs outside a transaction so the index can be built CONCURRENTLY: a
-- plain CREATE INDEX would block every issue write for the length of a scan
-- over the whole table. Every statement is idempotent, so a partial run can
-- be repeated — except that a failed concurrent build leaves an INVALID
-- issues_closed_idx behind, which IF NOT EXISTS would then skip: drop it
-- first.
--
-- +goose NO TRANSACTION
-- +goose Up

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS open_issue_count      INTEGER;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS metadata_refreshed_at TIMESTAMPTZ;

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_closed_idx ON issues (updated_at) WHERE state = 'CLOSED';

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.closed_issue_retention_days', '7', 'int', 0, 3650,
   'Days after closing on GitHub before an issue nobody saved or worked on is deleted. 0 keeps every closed issue.'),
  ('ingest.closed_issue_prune_interval_seconds', '3600', 'int', 60, 86400,
   'How often closed issues past their retention are deleted.'),
  ('ingest.closed_issue_prune_batch_size', '5000', 'int', 100, 100000,
   'Closed issues deleted per statement.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.closed_issue_retention_days',
  'ingest.closed_issue_prune_interval_seconds',
  'ingest.closed_issue_prune_batch_size'
);

DROP INDEX CONCURRENTLY IF EXISTS issues_closed_idx;

ALTER TABLE repositories DROP COLUMN IF EXISTS metadata_refreshed_at;
ALTER TABLE repositories DROP COLUMN IF EXISTS open_issue_count;
