-- Live flow visualizer: settings for the tap in go-api and ingestd and for
-- flowd, the relay that serves the public stream.
--
-- go-api and ingestd publish one small event per hop (an HTTP request, a
-- query's verb and table, a GitHub call, a crawled page) to Redis. flowd
-- subscribes, caps the labelled events per second, counts the rest as link
-- density, and streams the result to viewers. Redaction happens in the
-- publishing process (internal/flow): anything tied to a signed-in session
-- leaves it as symbols.
--
--   skillissues_flow   flowd's role. It reads its live settings and nothing
--                      else: no table holding user, issue or repository data.
--                      Created NOLOGIN; db/app-roles.sql enables login from
--                      FLOW_DB_PASSWORD on every deploy, like the other two.
--
-- +goose Up

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'skillissues_flow') THEN
    CREATE ROLE skillissues_flow NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
  END IF;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO skillissues_flow', current_database());
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO skillissues_flow;
GRANT SELECT ON settings TO skillissues_flow;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('flow.enabled', 'true', 'bool', NULL, NULL,
   'Publish live flow events from go-api and ingestd for the public visualizer (flowd). Signed-in traffic is published as symbols only.'),
  ('flow.buffer_size', '1024', 'int', 64, 65536,
   'Flow events a publisher holds while Redis catches up, and lines flowd holds for one slow viewer; past it, events are dropped and counted.'),
  ('flow.publish_timeout_ms', '250', 'int', 10, 5000,
   'Deadline for publishing one batch of flow events to Redis.'),
  ('flow.max_label_chars', '80', 'int', 8, 500,
   'Longest public label or payload (an issue title, a repository name) a flow event carries; longer text is truncated.'),
  ('flow.max_events_per_second', '40', 'int', 1, 1000,
   'Labelled flow events flowd relays per second; the rest count only towards their link''s density.'),
  ('flow.max_viewers', '200', 'int', 1, 5000,
   'Concurrent /stream viewers flowd serves; past it, new viewers get 503.'),
  ('flow.density_interval_ms', '1000', 'int', 100, 60000,
   'Window flowd counts events per link over before sending each link''s density.'),
  ('flow.heartbeat_seconds', '15', 'int', 1, 300,
   'How often flowd sends a keep-alive on an idle stream.'),
  ('flow.ambient_refresh_seconds', '5', 'int', 1, 300,
   'How often flowd reads cAdvisor network rates from Prometheus for the links nothing is instrumented on.'),
  ('flow.ambient_rate_window_seconds', '120', 'int', 30, 3600,
   'rate() window for those network rates; must span at least two cAdvisor scrapes.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM settings WHERE key IN (
  'flow.enabled',
  'flow.buffer_size',
  'flow.publish_timeout_ms',
  'flow.max_label_chars',
  'flow.max_events_per_second',
  'flow.max_viewers',
  'flow.density_interval_ms',
  'flow.heartbeat_seconds',
  'flow.ambient_refresh_seconds',
  'flow.ambient_rate_window_seconds'
);

REVOKE SELECT ON settings FROM skillissues_flow;
REVOKE USAGE ON SCHEMA public FROM skillissues_flow;
-- +goose StatementBegin
DO $$
BEGIN
  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM skillissues_flow', current_database());
END
$$;
-- +goose StatementEnd
DROP ROLE IF EXISTS skillissues_flow;
