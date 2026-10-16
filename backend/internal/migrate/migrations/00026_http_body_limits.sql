-- Cap request bodies. Nothing bounded them: the webhook handler read the whole
-- body into memory before checking its signature, and JSON endpoints decoded
-- whatever they were sent. Both limits are read per request.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.http.max_body_bytes', '1048576', 'int', 1024, 104857600,
   'Largest request body accepted on any API route except the webhook; larger bodies get 413.'),
  ('api.webhook.max_body_bytes', '26214400', 'int', 1024, 104857600,
   'Largest GitHub webhook body accepted (GitHub''s own cap is 25 MiB); larger bodies get 413.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'api.http.max_body_bytes',
  'api.webhook.max_body_bytes'
);
