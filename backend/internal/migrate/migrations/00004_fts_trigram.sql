-- Full-text and fuzzy search support for the issue catalog.
--
-- There is no issues.body column, so the search vector covers title and
-- labels only (see 00003's comment on why body is never stored unbounded).
--
-- +goose Up

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- to_tsvector(regconfig, text) is STABLE, not IMMUTABLE, in Postgres's own
-- catalog (it depends on the dictionary configuration, which can in theory
-- be altered), so it can't back a GENERATED ALWAYS ... STORED column. A
-- trigger has no such restriction and gives the same "always in sync, never
-- recomputed by the API" guarantee.
ALTER TABLE issues ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION issues_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('english', coalesce(NEW.title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(array_to_string(NEW.labels, ' '), '')), 'B');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS issues_search_vector_trg ON issues;
CREATE TRIGGER issues_search_vector_trg
  BEFORE INSERT OR UPDATE OF title, labels ON issues
  FOR EACH ROW EXECUTE FUNCTION issues_search_vector_update();

-- Backfill existing rows; new rows are covered by the trigger from here on.
UPDATE issues SET title = title WHERE search_vector IS NULL;

CREATE INDEX IF NOT EXISTS issues_search_vector_idx ON issues USING GIN (search_vector);

-- Trigram indexes back fuzzy/partial matching (ILIKE '%term%', similarity())
-- on repository name and issue author, independent of the tsvector above.
CREATE INDEX IF NOT EXISTS repositories_full_name_trgm_idx ON repositories USING GIN (full_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS issues_author_trgm_idx ON issues USING GIN (author gin_trgm_ops);

-- +goose Down

DROP INDEX IF EXISTS issues_author_trgm_idx;
DROP INDEX IF EXISTS repositories_full_name_trgm_idx;
DROP INDEX IF EXISTS issues_search_vector_idx;
DROP TRIGGER IF EXISTS issues_search_vector_trg ON issues;
DROP FUNCTION IF EXISTS issues_search_vector_update();
ALTER TABLE issues DROP COLUMN IF EXISTS search_vector;
-- pg_trgm is left installed: dropping an extension other objects might come
-- to depend on is riskier than leaving an unused extension in place.
