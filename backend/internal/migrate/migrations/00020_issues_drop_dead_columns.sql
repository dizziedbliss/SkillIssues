-- Removes what nothing reads (checked against prod on 2026-10-15, 3.7M rows):
--
--   has_difficulty_label          equal to (difficulty <> 'UNCLASSIFIED') on
--                                 every row; written by the crawler, never
--                                 read.
--   classification_rules_version  1 on every row, never read. Re-classifying
--                                 needs no version: raw labels are kept and
--                                 can be re-run through the rules.
--   issues_github_id_key          UNIQUE on github_id, 0 scans (~104 MB).
--                                 Issue identity is (repository_id, number);
--                                 nothing looks issues up by github_id, and
--                                 the constraint could only fail a crawl
--                                 page if one id ever showed up under two
--                                 repositories. The column itself stays —
--                                 the API returns it.
--   GIN indexes on labels, required_skills, technologies, learning_tags
--                                 0 scans (~65 MB). Search tests them with
--                                 `$1 = ANY(col)`, which a GIN index cannot
--                                 serve; it only helps @> and &&.
--
-- Adds a partial index on first_seen_at for the "catalog growth" panels.
-- Rows from before 00019 are NULL and excluded, so it starts small.
--
-- +goose Up

ALTER TABLE issues DROP COLUMN IF EXISTS has_difficulty_label;
ALTER TABLE issues DROP COLUMN IF EXISTS classification_rules_version;
ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_github_id_key;

DROP INDEX IF EXISTS issues_labels_idx;
DROP INDEX IF EXISTS issues_required_skills_idx;
DROP INDEX IF EXISTS issues_technologies_idx;
DROP INDEX IF EXISTS issues_learning_tags_idx;

CREATE INDEX IF NOT EXISTS issues_first_seen_idx
  ON issues (first_seen_at) WHERE first_seen_at IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS issues_first_seen_idx;

CREATE INDEX IF NOT EXISTS issues_learning_tags_idx   ON issues USING GIN (learning_tags);
CREATE INDEX IF NOT EXISTS issues_technologies_idx    ON issues USING GIN (technologies);
CREATE INDEX IF NOT EXISTS issues_required_skills_idx ON issues USING GIN (required_skills);
CREATE INDEX IF NOT EXISTS issues_labels_idx          ON issues USING GIN (labels);

ALTER TABLE issues ADD CONSTRAINT issues_github_id_key UNIQUE (github_id);

ALTER TABLE issues ADD COLUMN IF NOT EXISTS classification_rules_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS has_difficulty_label BOOLEAN NOT NULL DEFAULT false;
UPDATE issues SET has_difficulty_label = (difficulty <> 'UNCLASSIFIED');
