-- Repositories that must never be crawled, whatever their stars and activity.
--
-- The star floor and activity window are abuse and pacing controls; they say
-- nothing about a repository that passes them and is still worthless to
-- crawl. These four did: three use their issue tracker as a database (one has
-- ~127k issues of generated content) and one is a public archive that the
-- dataset snapshot still listed as active.
--
-- A table rather than a setting: it is a list with a reason per entry, and a
-- setting value is a single bounded scalar.
--
-- An excluded repository is ineligible ('excluded' in eligibility_reason), so
-- it is neither seeded nor re-queued nor claimed, and the coordinator's prune
-- step deletes the issues already stored for it (never one a user saved).
--
-- To exclude another repository (takes effect on the next eligibility
-- recheck, ingest.eligibility.recheck_seconds):
--   INSERT INTO excluded_repositories (full_name, reason) VALUES ('owner/name', 'why');
--
-- +goose Up

CREATE TABLE IF NOT EXISTS excluded_repositories (
  full_name  TEXT        NOT NULL,
  reason     TEXT        NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Matched case-insensitively, like repositories.full_name.
CREATE UNIQUE INDEX IF NOT EXISTS excluded_repositories_name_idx
  ON excluded_repositories (lower(full_name));

GRANT SELECT ON excluded_repositories TO metrics_ro;

INSERT INTO excluded_repositories (full_name, reason) VALUES
  ('mypdns/matrix',             'issue tracker used as a database (~127k generated issues)'),
  ('onysakura/news-daily',      'issue tracker used as a database'),
  ('BruceFeIix/picker',         'issue tracker used as a database'),
  ('microsoft/azuredatastudio', 'public archive; the dataset snapshot predates the archiving')
ON CONFLICT DO NOTHING;

-- Take effect now, not at the next recheck.
UPDATE repositories r
SET is_eligible = false, eligibility_reason = 'excluded'
FROM excluded_repositories x
WHERE lower(r.full_name) = lower(x.full_name)
  AND r.eligibility_reason IS DISTINCT FROM 'not_found';

-- +goose Down

DROP TABLE IF EXISTS excluded_repositories;
