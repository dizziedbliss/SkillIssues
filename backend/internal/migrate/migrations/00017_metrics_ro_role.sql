-- Read-only role for Grafana's Postgres datasource and postgres-exporter.
--
-- Created NOLOGIN with no password: the password is a secret, so
-- scripts/deploy-remote.sh enables login with METRICS_DB_PASSWORD from the
-- environment. Grants are explicit per table (no default privileges), so a
-- new table stays invisible to dashboards until it is granted here — and
-- credential columns are never granted:
--   users.github_access_token, users.github_refresh_token,
--   auth_sessions.token, and all of oauth_states.
--
-- +goose Up

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'metrics_ro') THEN
    CREATE ROLE metrics_ro NOLOGIN;
  END IF;
END
$$;
-- +goose StatementEnd

-- Dashboards are a convenience; they must not be able to hold up the API on
-- a 2-vCPU host with a runaway query.
ALTER ROLE metrics_ro SET statement_timeout = '15s';
ALTER ROLE metrics_ro SET default_transaction_read_only = on;

GRANT pg_monitor TO metrics_ro;
GRANT USAGE ON SCHEMA public TO metrics_ro;

GRANT SELECT (id, created_at) ON users TO metrics_ro;
GRANT SELECT (user_id, provider, expires_at, created_at) ON auth_sessions TO metrics_ro;

GRANT SELECT ON
  repositories, issues, contributions, contribution_audit_log,
  developer_profiles, developer_preferences, saved_issues,
  ingest_jobs, github_budget, sync_state, import_skipped,
  webhook_events, settings, settings_audit
TO metrics_ro;

-- +goose Down

REVOKE ALL ON
  repositories, issues, contributions, contribution_audit_log,
  developer_profiles, developer_preferences, saved_issues,
  ingest_jobs, github_budget, sync_state, import_skipped,
  webhook_events, settings, settings_audit, users, auth_sessions
FROM metrics_ro;
REVOKE USAGE ON SCHEMA public FROM metrics_ro;
REVOKE pg_monitor FROM metrics_ro;
DROP ROLE IF EXISTS metrics_ro;
