-- Enables login for the skillissues_api and skillissues_ingest roles created
-- by migration 00029, using API_DB_PASSWORD and INGEST_DB_PASSWORD from this
-- container's environment. Same approach as monitoring/metrics-role.sql:
-- \getenv keeps the passwords out of every process argument list, psql
-- interpolates :'pw' client-side, and raising log_min_error_statement means a
-- failure logs the error but not the statement text containing a password.
SET log_min_error_statement = panic;

\getenv api_pw API_DB_PASSWORD
\getenv ingest_pw INGEST_DB_PASSWORD
\if :{?api_pw}
\else
  \warn 'API_DB_PASSWORD is not set'
  \quit 1
\endif
\if :{?ingest_pw}
\else
  \warn 'INGEST_DB_PASSWORD is not set'
  \quit 1
\endif

ALTER ROLE skillissues_api    LOGIN PASSWORD :'api_pw';
ALTER ROLE skillissues_ingest LOGIN PASSWORD :'ingest_pw';
