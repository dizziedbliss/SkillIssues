-- Live-reloadable operational configuration.
--
-- Values here can be changed on a running system: an AFTER trigger issues
-- NOTIFY settings_changed and every process reloads. Validation lives in the
-- database rather than only in the API handler, so neither the admin endpoint
-- nor a raw psql UPDATE can install a value that would break the fleet.
--
-- Secrets never belong in this table. It is readable through an admin endpoint
-- and reproduced in settings_audit and in database backups.

-- +goose Up

CREATE TABLE IF NOT EXISTS settings (
  key         TEXT        PRIMARY KEY,
  value       JSONB       NOT NULL,
  value_type  TEXT        NOT NULL CHECK (value_type IN ('int', 'bool', 'string')),
  min_value   NUMERIC,
  max_value   NUMERIC,
  description TEXT        NOT NULL DEFAULT '',
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by  TEXT        NOT NULL DEFAULT 'seed'
);

-- Append-only record of every change, including ones made directly via psql.
CREATE TABLE IF NOT EXISTS settings_audit (
  id         BIGSERIAL   PRIMARY KEY,
  key        TEXT        NOT NULL,
  old_value  JSONB,
  new_value  JSONB       NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  changed_by TEXT        NOT NULL
);

CREATE INDEX IF NOT EXISTS settings_audit_key_idx ON settings_audit(key, changed_at DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION settings_validate() RETURNS trigger AS $$
DECLARE
  num NUMERIC;
BEGIN
  -- The type and bounds define the contract. Allowing them to be edited would
  -- let a caller widen a range and then install an out-of-range value, so on
  -- UPDATE only `value` and `updated_by` may change.
  IF TG_OP = 'UPDATE' THEN
    IF NEW.value_type IS DISTINCT FROM OLD.value_type
       OR NEW.min_value IS DISTINCT FROM OLD.min_value
       OR NEW.max_value IS DISTINCT FROM OLD.max_value
       OR NEW.key IS DISTINCT FROM OLD.key THEN
      RAISE EXCEPTION
        'setting %: type and bounds are immutable; change them with a migration', NEW.key;
    END IF;
  END IF;

  IF NEW.value_type = 'int' THEN
    IF jsonb_typeof(NEW.value) <> 'number' THEN
      RAISE EXCEPTION 'setting %: expected a number, got %', NEW.key, jsonb_typeof(NEW.value);
    END IF;
    num := (NEW.value #>> '{}')::numeric;
    IF num <> trunc(num) THEN
      RAISE EXCEPTION 'setting %: expected a whole number, got %', NEW.key, num;
    END IF;
  ELSIF NEW.value_type = 'bool' THEN
    IF jsonb_typeof(NEW.value) <> 'boolean' THEN
      RAISE EXCEPTION 'setting %: expected a boolean, got %', NEW.key, jsonb_typeof(NEW.value);
    END IF;
  ELSIF NEW.value_type = 'string' THEN
    IF jsonb_typeof(NEW.value) <> 'string' THEN
      RAISE EXCEPTION 'setting %: expected a string, got %', NEW.key, jsonb_typeof(NEW.value);
    END IF;
  END IF;

  IF num IS NOT NULL THEN
    IF NEW.min_value IS NOT NULL AND num < NEW.min_value THEN
      RAISE EXCEPTION 'setting %: % is below the minimum of %', NEW.key, num, NEW.min_value;
    END IF;
    IF NEW.max_value IS NOT NULL AND num > NEW.max_value THEN
      RAISE EXCEPTION 'setting %: % is above the maximum of %', NEW.key, num, NEW.max_value;
    END IF;
  END IF;

  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION settings_announce() RETURNS trigger AS $$
BEGIN
  INSERT INTO settings_audit (key, old_value, new_value, changed_by)
  VALUES (
    NEW.key,
    CASE WHEN TG_OP = 'UPDATE' THEN OLD.value ELSE NULL END,
    NEW.value,
    NEW.updated_by
  );

  -- NOTIFY is not durable: a listener that is disconnected right now never
  -- receives this. Processes pair it with a periodic reconcile poll.
  PERFORM pg_notify('settings_changed', NEW.key);
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS settings_validate_trg ON settings;
CREATE TRIGGER settings_validate_trg
  BEFORE INSERT OR UPDATE ON settings
  FOR EACH ROW EXECUTE FUNCTION settings_validate();

DROP TRIGGER IF EXISTS settings_announce_trg ON settings;
CREATE TRIGGER settings_announce_trg
  AFTER INSERT OR UPDATE ON settings
  FOR EACH ROW EXECUTE FUNCTION settings_announce();

-- ---------------------------------------------------------------------------
-- Seed every key at the value that was previously hardcoded, so applying this
-- migration changes no behaviour.
-- ---------------------------------------------------------------------------

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('api.rate_limit.unauth_per_hour', '180',  'int', 1,  100000,
   'Requests per window allowed for unauthenticated callers.'),
  ('api.rate_limit.auth_per_hour',   '1000', 'int', 1,  1000000,
   'Requests per window allowed for authenticated callers.'),
  ('api.rate_limit.window_seconds',  '3600', 'int', 60, 86400,
   'Length of the rate-limit window.'),

  ('api.issues.default_page_size',   '20', 'int', 1, 100,
   'Issues returned when the catalog is browsed without a search term.'),
  ('api.issues.search_page_size',    '50', 'int', 1, 200,
   'Issues returned for a search query.'),
  ('api.search.per_category_cap',    '25', 'int', 1, 100,
   'Maximum results per match category, so one category cannot flood the rest.'),

  ('api.session.ttl_hours',          '168', 'int', 1, 8760,
   'Session lifetime. Applies to newly issued sessions only.'),

  ('api.cache.hot_issues_ttl_seconds',      '120', 'int', 0, 3600,
   'Cache lifetime for the default unauthenticated catalog page. 0 disables.'),
  ('api.cache.search_ttl_seconds',          '60',  'int', 0, 3600,
   'Cache lifetime for unauthenticated search results. 0 disables.'),
  ('api.cache.recommendations_ttl_seconds', '300', 'int', 0, 3600,
   'Cache lifetime for per-user recommendations. 0 disables.'),

  ('api.contribution.verify_per_hour', '5', 'int', 1, 1000,
   'Maximum PR verification attempts per user per contribution per hour.'),

  ('ingest.sync_interval_seconds', '900', 'int', 30, 86400,
   'Delay between routine sync sweeps.'),
  ('ingest.graphql_page_size',     '50',  'int', 1,  100,
   'Issues requested per GraphQL page. GitHub caps this at 100.'),
  ('ingest.rate_limit_floor',      '100', 'int', 0,  5000,
   'Stop issuing GraphQL calls when the remaining point budget falls below this.'),
  ('ingest.max_attempts',          '3',   'int', 1,  10,
   'Attempts per repository before a sync is recorded as failed.'),
  ('ingest.backoff_base_seconds',  '1',   'int', 1,  60,
   'Base for exponential retry backoff between sync attempts.'),
  ('ingest.routine_workers',       '4',   'int', 1,  64,
   'Concurrent routine sync workers.'),
  ('ingest.priority_workers',      '2',   'int', 1,  64,
   'Concurrent priority sync workers.'),
  ('ingest.ondemand_workers',      '8',   'int', 1,  64,
   'Concurrent on-demand sync workers.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP TRIGGER IF EXISTS settings_announce_trg ON settings;
DROP TRIGGER IF EXISTS settings_validate_trg ON settings;
DROP FUNCTION IF EXISTS settings_announce();
DROP FUNCTION IF EXISTS settings_validate();
DROP TABLE IF EXISTS settings_audit;
DROP TABLE IF EXISTS settings;
