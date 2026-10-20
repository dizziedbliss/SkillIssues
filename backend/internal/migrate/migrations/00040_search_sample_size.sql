-- Bounded search.
--
-- GET /issues?search= now runs one indexed query per match category instead
-- of one query ORing every kind of match, which ran past the query timeout on
-- the full catalog (issue/search_categories.go).
--
--   api.search.sample_size   the Title and Author categories can match
--                            hundreds of thousands of issues for a common
--                            word; they read this many off their index and
--                            order those by stars
--   api.search.per_repo_cap  the most issues one repository contributes to
--                            the Repository or Language category, so one
--                            large repository cannot fill it

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.search.sample_size', '2000', 'int', 100, 100000,
   'Title and author matches a search reads before ordering them by stars; bounds the work for a common word.'),
  ('api.search.per_repo_cap', '3', 'int', 1, 100,
   'Most issues one repository contributes to a search''s Repository or Language category, so one large repository cannot fill it.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN ('api.search.sample_size', 'api.search.per_repo_cap');
