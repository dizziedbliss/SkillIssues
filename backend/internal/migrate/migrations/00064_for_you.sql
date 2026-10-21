-- +goose Up
-- The For you feed (issue/foryou.go), the default when browsing by cursor:
-- the shuffled walk from a point seeded per person per day, each chunk of it
-- ordered by a weighted random draw on Fit × Doable × Need.

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.foryou.pool_repos', '150', 'int', 20, 1000,
   'Repositories the For you feed walks per chunk; each chunk is ordered by one draw.'),
  ('api.foryou.randomness_percent', '60', 'int', 0, 300,
   'How random For you''s draw is: 100 samples in proportion to score, 0 is a plain ranking, more spreads attention further.'),
  ('api.foryou.weight_fit_percent', '100', 'int', 0, 300,
   'Weight (exponent, percent) of Fit in the For you score: languages, level, interests, past merges.'),
  ('api.foryou.weight_doable_percent', '100', 'int', 0, 300,
   'Weight (exponent, percent) of Doable in the For you score: labelled, short discussion, recent, welcoming repository.'),
  ('api.foryou.weight_need_percent', '100', 'int', 0, 300,
   'Weight (exponent, percent) of Need in the For you score: how long it has waited, how few were shown it.'),
  ('api.foryou.exposure_days', '7', 'int', 1, 30,
   'Days of serves the For you feed counts against an issue.'),
  ('api.foryou.exposure_half', '20', 'int', 0, 100000,
   'Serves in that window at which an issue''s Need halves. 0 ignores serves.'),
  ('api.foryou.chunk_ttl_seconds', '1800', 'int', 0, 86400,
   'How long a drawn For you chunk is kept in Redis, so paging doesn''t redraw it. 0 draws every time.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key LIKE 'api.foryou.%';
