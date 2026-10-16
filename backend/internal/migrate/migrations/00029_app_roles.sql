-- Least-privilege database roles for the two long-running services.
--
-- Until now api and ingestd connected as the bootstrap user, which owns every
-- table and is a superuser: a SQL injection or a compromised process could
-- DROP the schema, read the password hashes of every role, or run commands on
-- the database host (COPY ... PROGRAM). They now connect as:
--
--   skillissues_api     the HTTP API: accounts, sessions, saved issues,
--                       notifications, webhook state changes, settings edits.
--   skillissues_ingest  ingestd and cmd/repoimport: the crawl and import.
--
-- Neither owns anything, can create objects, or can TRUNCATE. Migrations keep
-- running as the bootstrap user (the owner), so it stays the one place DDL
-- happens; there is no separate migration role because the owner already is
-- one, and a non-superuser could not create these roles or install pg_trgm.
--
-- Roles are created NOLOGIN with no password, like metrics_ro (00017): the
-- passwords are secrets, so db/app-roles.sql enables login from
-- API_DB_PASSWORD / INGEST_DB_PASSWORD on every deploy.
--
-- Grants are explicit per table, with no default privileges: a new table is
-- invisible to both roles until its migration grants it, and a forgotten grant
-- fails loudly instead of quietly handing out access. roles_test.go fails if a
-- table is neither granted nor listed as deliberately ungranted.
--
-- Trigger functions run as the invoker, so the grants include what the
-- triggers touch: whoever UPDATEs issues.state needs INSERT/UPDATE on
-- notifications (and its sequence) and SELECT on saved_issues; whoever
-- UPDATEs settings needs INSERT on settings_audit. Foreign-key checks run as
-- the table owner, so they need no grants.
--
-- +goose Up

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'skillissues_api') THEN
    CREATE ROLE skillissues_api NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'skillissues_ingest') THEN
    CREATE ROLE skillissues_ingest NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
  END IF;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO skillissues_api, skillissues_ingest', current_database());
END
$$;
-- +goose StatementEnd

-- Older clusters let PUBLIC create objects in the public schema.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO skillissues_api, skillissues_ingest;

-- ----- skillissues_api -----
GRANT SELECT, INSERT, UPDATE, DELETE ON users                         TO skillissues_api;
GRANT SELECT, INSERT, DELETE         ON auth_sessions, oauth_states   TO skillissues_api;
GRANT SELECT, INSERT, UPDATE         ON developer_profiles, developer_preferences TO skillissues_api;
GRANT SELECT, INSERT, DELETE         ON saved_issues                  TO skillissues_api;
GRANT SELECT, INSERT, UPDATE         ON notifications                 TO skillissues_api;
GRANT SELECT, UPDATE                 ON issues                        TO skillissues_api;
GRANT SELECT                         ON repositories                  TO skillissues_api;
GRANT SELECT, INSERT                 ON webhook_events                TO skillissues_api;
GRANT SELECT, UPDATE                 ON settings                      TO skillissues_api;
GRANT INSERT                         ON settings_audit                TO skillissues_api;
GRANT USAGE ON SEQUENCE users_id_seq, notifications_id_seq, settings_audit_id_seq TO skillissues_api;

-- ----- skillissues_ingest -----
GRANT SELECT, INSERT, UPDATE         ON repositories                  TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE, DELETE ON issues                        TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE         ON ingest_jobs, github_budget, import_skipped TO skillissues_ingest;
GRANT SELECT                         ON excluded_repositories, saved_issues, settings TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE         ON notifications                 TO skillissues_ingest;
GRANT USAGE ON SEQUENCE repositories_id_seq, issues_id_seq, ingest_jobs_id_seq, notifications_id_seq TO skillissues_ingest;

-- +goose Down

REVOKE ALL ON ALL TABLES    IN SCHEMA public FROM skillissues_api, skillissues_ingest;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM skillissues_api, skillissues_ingest;
REVOKE USAGE ON SCHEMA public FROM skillissues_api, skillissues_ingest;
-- +goose StatementBegin
DO $$
BEGIN
  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM skillissues_api, skillissues_ingest', current_database());
END
$$;
-- +goose StatementEnd
DROP ROLE IF EXISTS skillissues_api;
DROP ROLE IF EXISTS skillissues_ingest;
