-- Bounds on reading a filtered catalog (issue/filters.go, issue/shuffle.go).
--
--   api.issues.rare_probe_ms           counting a combination of filters to
--                                      choose how to read it could itself run
--                                      for 34s (two common labels' bitmaps);
--                                      past this it is taken as common
--   api.issues.shuffle_walk_max_repos  a filter whose matches sit in few
--                                      repositories made the shuffled feed walk
--                                      most of 300k repositories for a page;
--                                      past this many it reads the matches

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.issues.rare_probe_ms', '500', 'int', 0, 10000,
   'How long counting a combination of filters may take when deciding how to browse it; one that takes longer is read in index order. 0 does not bound it.'),
  ('api.issues.shuffle_walk_max_repos', '3000', 'int', 0, 1000000,
   'Repositories one page of the shuffled feed walks before reading the rest of the page from the matching issues. 0 walks without a bound.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN ('api.issues.rare_probe_ms', 'api.issues.shuffle_walk_max_repos');
