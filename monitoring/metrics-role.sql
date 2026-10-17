-- Enables login for the metrics_ro role created by migration 00017, using
-- METRICS_DB_PASSWORD from this container's environment. \getenv keeps the
-- password out of every process argument list.
--
-- psql interpolates :'pw' client-side, so the server receives the literal.
-- Raising log_min_error_statement means that if this ever fails, Postgres
-- logs the error but not the statement text containing the password.
SET log_min_error_statement = panic;
-- A missing password fails with an error, not \quit: psql's \quit takes no exit
-- status, so `\quit 1` would exit 0 and the one-shot would look successful.

\getenv pw METRICS_DB_PASSWORD
\if :{?pw}
\else
  DO $$ BEGIN RAISE EXCEPTION 'METRICS_DB_PASSWORD is not set'; END $$;
\endif

ALTER ROLE metrics_ro LOGIN PASSWORD :'pw';
