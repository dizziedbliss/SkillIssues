-- +goose Up
-- How many recommendations GET /recommendations returns without ?limit,
-- until now a literal in the handler.
INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.recommendations.default_limit', '3', 'int', 1, 100,
   'Recommendations GET /recommendations returns without ?limit (never above api.recommendations.max_limit). Only this size is cached.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key = 'api.recommendations.default_limit';
