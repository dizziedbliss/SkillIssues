-- Enables login for the skillissues_api and skillissues_ingest roles created
-- by migration 00029 and the skillissues_flow role created by 00033, using
-- API_DB_PASSWORD, INGEST_DB_PASSWORD and FLOW_DB_PASSWORD from this
-- container's environment. Same approach as monitoring/metrics-role.sql:
-- \getenv keeps the passwords out of every process argument list, psql
-- interpolates :'pw' client-side, and raising log_min_error_statement means a
-- failure logs the error but not the statement text containing a password.
SET log_min_error_statement = panic;
-- A missing password fails with an error, not \quit: psql's \quit takes no exit
-- status, so `\quit 1` would exit 0 and the one-shot would look successful.

\getenv api_pw API_DB_PASSWORD
\getenv ingest_pw INGEST_DB_PASSWORD
\getenv flow_pw FLOW_DB_PASSWORD
\if :{?api_pw}
\else
  DO $$ BEGIN RAISE EXCEPTION 'API_DB_PASSWORD is not set'; END $$;
\endif
\if :{?ingest_pw}
\else
  DO $$ BEGIN RAISE EXCEPTION 'INGEST_DB_PASSWORD is not set'; END $$;
\endif
\if :{?flow_pw}
\else
  DO $$ BEGIN RAISE EXCEPTION 'FLOW_DB_PASSWORD is not set'; END $$;
\endif

ALTER ROLE skillissues_api    LOGIN PASSWORD :'api_pw';
ALTER ROLE skillissues_ingest LOGIN PASSWORD :'ingest_pw';
ALTER ROLE skillissues_flow   LOGIN PASSWORD :'flow_pw';
