-- Newest, narrowed to a difficulty: each rated difficulty's open issues by
-- created_at, newest first. Without it, Newest with BEGINNER walked every open
-- issue by created_at to find the few rated ones (recent issues are mostly
-- unrated) and ran past the query timeout. issue.newestByDifficulty reads one
-- range of it per difficulty; unrated issues come from issues_open_newest_idx.
-- Partial on rated issues only, so it is a fraction of the table's size.
--
-- Built CONCURRENTLY, outside a transaction, so issue writes go on meanwhile.
-- A failed concurrent build leaves an INVALID index that IF NOT EXISTS would
-- skip: drop it first.
--
-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS issues_open_rated_newest_idx
  ON issues (difficulty_score, created_at DESC, id DESC)
  WHERE state = 'OPEN' AND difficulty_score IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS issues_open_rated_newest_idx;
