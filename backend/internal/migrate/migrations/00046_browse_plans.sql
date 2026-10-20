-- Plans for filtered browsing that held up on production (issue/shuffle.go,
-- issue/filters.go).
--
-- issues_open_repo_labels_idx: the shuffled feed looks up each repository's
-- best issue with a label. Without an index covering both it read the
-- repository's open issues one by one, evaluating catalog_lower_labels() on
-- each, until one matched; for a common label in repositories without it
-- that ran to seconds. A GIN index over (repository_id, labels) finds the
-- matches directly. btree_gin supplies the integer column's GIN operators.
--
-- issues_difficulty_dep: a difficulty filter is also written on
-- difficulty_score (filters.go), which the classifier fixes per difficulty.
-- Without being told, the planner multiplied the two conditions'
-- selectivities, estimated 221 ADVANCED issues where there are 39,318, and
-- read all of them to sort instead of walking the catalog index in order:
-- 7s for one page. Functional-dependency statistics fix the estimate.
--
-- Runs outside a transaction to build the index CONCURRENTLY; a failed build
-- leaves an INVALID index, which must be dropped before retrying.
--
-- +goose NO TRANSACTION
-- +goose Up

CREATE EXTENSION IF NOT EXISTS btree_gin;

CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_repo_labels_idx
  ON issues USING GIN (repository_id, catalog_lower_labels(labels))
  WHERE state = 'OPEN';

CREATE STATISTICS IF NOT EXISTS issues_difficulty_dep (dependencies) ON difficulty, difficulty_score FROM issues;
ANALYZE issues;

-- +goose Down

DROP STATISTICS IF EXISTS issues_difficulty_dep;
DROP INDEX CONCURRENTLY IF EXISTS issues_open_repo_labels_idx;
