-- Filtering the catalog by repository or by owner (issue/filters.go): "more
-- issues from this repository" and "from this organisation" on the web app.
--
-- A repository is matched on lower(full_name), which
-- repositories_full_name_lower_idx (00003) already serves. An owner is the
-- part of full_name before the slash; this index finds an owner's
-- repositories without reading every row.
--
-- +goose NO TRANSACTION
-- +goose Up

CREATE INDEX CONCURRENTLY IF NOT EXISTS repositories_owner_lower_idx
  ON repositories (lower(split_part(full_name, '/', 1)));

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS repositories_owner_lower_idx;
