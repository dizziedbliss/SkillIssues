-- Bounded recommendations.
--
-- GET /recommendations read every open issue into memory to score them —
-- 7.5M rows on production. It now reads a bounded pool off the shuffled feed
-- (one issue per repository, matching the caller's languages and level) and
-- scores that (issue/recommend.go).
--
--   api.recommendations.candidate_pool  how many issues that pool holds

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.recommendations.candidate_pool', '200', 'int', 10, 1000,
   'Open issues GET /recommendations reads off the shuffled feed, matching the caller''s languages and level, before scoring them.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'api.recommendations.candidate_pool';
