-- Limits on what a profile holds (PUT /profile, profile.UpdateProfile). Before
-- these, a bio could be as long as a request body and the lists any length,
-- and an unknown level was quietly stored as a default instead of refused.

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.profile.bio_max_chars', '280', 'int', 0, 2000,
   'The longest bio a profile may hold, in characters.'),
  ('api.profile.max_languages', '12', 'int', 1, 100,
   'The most preferred languages a profile may list.'),
  ('api.profile.max_interests', '12', 'int', 1, 100,
   'The most interests a profile may list.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN ('api.profile.bio_max_chars', 'api.profile.max_languages', 'api.profile.max_interests');
