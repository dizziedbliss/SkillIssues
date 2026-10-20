-- A time bound on one page of the shuffled feed (issue/shuffle.go). Some
-- filter combinations match nothing anywhere (a label the classifier always
-- rates, filtered to "unrated"), and no read of them is cheap: the walk ran
-- into the 10s query timeout. Past this budget the page is returned short,
-- with a cursor to continue from.

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.issues.shuffle_walk_budget_ms', '2000', 'int', 0, 9000,
   'How long one page of the shuffled feed may look for matches before it is returned short, with a cursor to continue from. 0 does not bound it.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'api.issues.shuffle_walk_budget_ms';
