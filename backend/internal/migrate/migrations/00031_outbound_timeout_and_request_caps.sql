-- Bound what a caller can make the server do, and what the server will wait on.
--
-- The outbound timeout replaces http.DefaultClient (no timeout) in sign-in, the
-- GitHub App token mint and the ingestd proxy, and the crawler's literal 15s.
-- The caps stop one request from asking for an unbounded page, or an offset deep
-- enough to scan the table. Read per request / per call.
--
-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('http.outbound_timeout_seconds', '15', 'int', 1, 120,
   'Deadline for each outbound call to GitHub or ingestd; a call that exceeds it fails.'),
  ('api.issues.max_limit', '100', 'int', 1, 500,
   'Largest ?limit honoured by GET /issues; a larger request is clamped.'),
  ('api.issues.max_page', '200', 'int', 1, 10000,
   'Deepest ?page honoured by GET /issues; a deeper request is clamped. Deep offsets are expensive.'),
  ('api.recommendations.max_limit', '20', 'int', 1, 100,
   'Largest ?limit honoured by GET /recommendations; a larger request is clamped.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'http.outbound_timeout_seconds',
  'api.issues.max_limit',
  'api.issues.max_page',
  'api.recommendations.max_limit'
);
