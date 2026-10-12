-- Live-tunable knobs for the continuous ingestion cycle: how often a
-- repository's first page is re-fetched (freshness), how much work one
-- coordinator cycle enqueues, and how often stored repository metadata is
-- re-tested against the current eligibility thresholds.
--
-- Nothing about the cycle is a compile-time constant: changing a threshold
-- or an interval must take effect without a re-import, a redeploy or a
-- restart.
--
-- +goose Up

-- The two fields the eligibility rule tests but the repositories table never
-- stored, so eligibility could only ever be decided at import time. With
-- these, ingest.RecomputeEligibility can re-test every stored repository in
-- SQL against the live thresholds.
-- issue_count is nullable on purpose: NULL means "not imported with this
-- field yet", which RecomputeEligibility treats as unknown rather than as
-- zero. Reading it as zero would make the whole pre-existing catalog
-- ineligible on the first recompute.
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS is_fork     BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS issue_count INTEGER;

-- Best-effort backfill from what has actually been crawled, so repositories
-- already known to have issues do not sit on an unknown count until the next
-- importer pass. A repository with no crawled issues yet stays NULL.
UPDATE repositories r
SET issue_count = counted.n
FROM (SELECT repository_id, count(*) AS n FROM issues GROUP BY repository_id) counted
WHERE r.id = counted.repository_id AND r.issue_count IS NULL;

-- The freshness cycle claims "oldest successful sync first", so that column
-- needs to be reachable without a sort of the whole table. Partial: only
-- eligible repositories are ever cycled.
CREATE INDEX IF NOT EXISTS repositories_freshness_idx
  ON repositories (last_synced_at NULLS FIRST)
  WHERE is_eligible;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.freshness_interval_seconds', '86400', 'int', 300, 2592000,
   'How old a repository''s last successful sync must be before its first issue page is re-fetched.'),
  ('ingest.enqueue_batch_size', '500', 'int', 1, 50000,
   'Jobs enqueued per coordinator cycle, for both new-repository seeding and freshness re-queues.'),
  ('ingest.eligibility.recheck_seconds', '3600', 'int', 60, 86400,
   'How often stored repository metadata is re-tested against the current eligibility thresholds.')
ON CONFLICT (key) DO NOTHING;

-- Repositories imported before the "store every record" change have no
-- eligibility_reason recorded for the ineligible case. Backfilling is not
-- possible from here (their metadata was never stored), so this is left to
-- the next importer pass, which recomputes both columns for every record.

-- +goose Down

DELETE FROM settings WHERE key IN (
  'ingest.freshness_interval_seconds',
  'ingest.enqueue_batch_size',
  'ingest.eligibility.recheck_seconds'
);

DROP INDEX IF EXISTS repositories_freshness_idx;
ALTER TABLE repositories DROP COLUMN IF EXISTS issue_count;
ALTER TABLE repositories DROP COLUMN IF EXISTS is_fork;
