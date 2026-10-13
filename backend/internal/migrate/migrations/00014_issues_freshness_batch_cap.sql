-- POST /issues/freshness (D2a, Phase 2): a client checks the handful of
-- issues it is actually showing (Saved/Working render), never the whole
-- catalog. This caps that request size.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.issues.freshness_batch_cap', '25', 'int', 1, 100,
   'Maximum issue ids accepted by one POST /issues/freshness call.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key = 'api.issues.freshness_batch_cap';
