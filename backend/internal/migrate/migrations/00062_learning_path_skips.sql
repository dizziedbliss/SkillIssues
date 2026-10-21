-- +goose Up
-- Issues swapped out of a learning path step. A swap never offers one of them
-- again on that path, so swapping walks on through the catalog instead of
-- trading the same few issues back and forth.

CREATE TABLE IF NOT EXISTS learning_path_skips (
  path_id    BIGINT      NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
  issue_id   INTEGER     NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  skipped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (path_id, issue_id)
);

GRANT SELECT, INSERT ON learning_path_skips TO skillissues_api;

-- +goose Down
DROP TABLE IF EXISTS learning_path_skips;
