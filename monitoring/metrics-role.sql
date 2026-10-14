-- Enables login for the metrics_ro role created by migration 00017, using
-- METRICS_DB_PASSWORD from this container's environment. \getenv keeps the
-- password out of every process argument list.
--
-- psql interpolates :'pw' client-side, so the server receives the literal.
-- Raising log_min_error_statement means that if this ever fails, Postgres
-- logs the error but not the statement text containing the password.
SET log_min_error_statement = panic;

\getenv pw METRICS_DB_PASSWORD
\if :{?pw}
\else
  \warn 'METRICS_DB_PASSWORD is not set'
  \quit 1
\endif

ALTER ROLE metrics_ro LOGIN PASSWORD :'pw';
