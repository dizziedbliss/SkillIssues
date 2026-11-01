# Migrations (`internal/migrate/migrations/`)

Goose migrations, embedded with `go:embed`. **This directory is the only source of the schema**; there
is no `schema.sql`.

```bash
cd backend
go run ./cmd/migrate                            # apply (DATABASE_URL)
go test ./internal/migrate/                     # the grants checks (TEST_DATABASE_URL)
```

## Rules

- **Never edit a migration that has been applied anywhere.** Add a new one, numbered after the last.
- Idempotent (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`) with a working `Down`.
- **A new table is granted** to the roles that use it (`skillissues_api`, `skillissues_ingest`,
  `skillissues_flow`), plus `USAGE` on its sequence. Grants are explicit per table; there are no default
  privileges. `roles_test.go` fails when a table is granted to no role and isn't listed as deliberately
  ungranted.
- **Triggers run as the invoker:** a table a trigger writes must be granted to whoever fires it.
- **`metrics_ro`** (Grafana, postgres-exporter) gets a new table only if a dashboard needs it, in a new
  migration. **Never grant it a credential column.**
- **A new setting** is a key in `settings.Registry` **and** a seeded row here, with bounds in the
  validation trigger (the authoritative boundary; the registry mirrors them). A key missing from the
  table falls back to its default and is logged as missing on every load. `docs/settings.md` lists every
  key: update it too.
- Services connect as the roles above, never the owner: no DDL, `TRUNCATE` or `COPY ... PROGRAM` at
  runtime. Only `migrate` and backups use the owner.
