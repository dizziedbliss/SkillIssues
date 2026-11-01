# Backend (Go)

Binaries under `cmd/`: `api` (HTTP API, :8000), `ingestd` (GitHub crawler, :8002), `flowd` (live
visualizer relay, :8003), `repoimport` (offline dataset import), `migrate` (applies the schema and
exits), `healthcheck` (Docker `HEALTHCHECK`). Deeper guides: `internal/ingest/CLAUDE.md`,
`internal/issue/CLAUDE.md`, `internal/migrate/CLAUDE.md`.

## Workflow

```bash
cd backend
# 1. Make changes
# 2. Format, build, vet (fast)
gofmt -l . && go build ./... && go vet ./...
# 3. Test what you touched
go test ./internal/issue/...                     # one package
go test ./internal/issue/ -run TestInterleave    # one test
go test -short ./...                             # everything except tests needing a live API
# 4. Before a PR: what CI runs
go vet ./... && gofmt -l . && go build ./... && go test -short ./... && govulncheck ./...
```

- Database tests run when `TEST_DATABASE_URL` points at a migrated Postgres. **Tests must pass with
  Redis stopped**: Redis is cache and rate limiting only.
- Run natively against Docker's databases: `docker compose up -d postgres redis`, then
  `DATABASE_URL=postgresql://skillissues:skillissues@localhost:5432/skillissues go run ./cmd/migrate`,
  then `go run ./cmd/api` with `REDIS_URL`, `FRONTEND_URL`, `GITHUB_TOKEN`, `PORT=8000`.
- Dataset import: `go run ./cmd/repoimport --file ../data/repo_metadata.json` (resumable, idempotent;
  touches only `repositories` and `import_skipped`).

## Rules

- **Standard library `http.NewServeMux`** with Go 1.22 patterns (`"GET /issues/{id}"`). No third-party
  router.
- Middleware, outermost first: RequestID → Logger → FlowTap → MaxBody → CORS → CSRF → OptionalAuth →
  RateLimiter. `/admin/` and `/webhooks/` are mounted outside it (they authenticate themselves).
- **`pgxpool` for all database access**; never open single connections. Thread `ctx` into every
  database, Redis and HTTP call.
- **Configuration has three layers:** bootstrap env (`DATABASE_URL`, `REDIS_URL`, `PORT`,
  `FRONTEND_URL`), secrets env, and the live `settings` table for everything operational. Never hardcode
  a threshold, interval, page size, worker count or cap: add it to `settings.Registry` and seed it in a
  migration. **Secrets never go in `settings`** (it's readable by admins, audited and backed up). Read
  through `live.Current()` and hold the snapshot for the whole operation. Anything sized by a setting
  uses `worker.Limiter`, not a fixed goroutine count.
- **Metrics** (`internal/metrics`): each binary registers only its own families (`RegisterAPI`,
  `RegisterIngest`, …). HTTP metrics are labelled by the matched mux pattern, never the raw path:
  `middleware.RecordRoute` wraps each mux directly. Every GitHub call goes through
  `metrics.InstrumentedTransport`; tag new client methods with `metrics.WithOperation(ctx, "Name")`. A
  failure logged on an operational path also increments a counter.
- **Flow tap** (`internal/flow`): redaction lives only in `flow.Emit`; contexts marked by
  `flow.MarkUser` / `flow.UserLinked` publish as random symbols. Never build a label from anything you
  couldn't show a stranger, and never add an identifying field to `flow.Event`
  (`TestEventHasNoIdentifyingFields`). `Emit` never blocks or fails its caller; with no publisher
  installed it's a no-op. `flow.enabled` (live) turns the tap off. flowd relays at most
  `flow.max_events_per_second` labelled events, reads settings as `skillissues_flow`, and re-encodes
  every event through `flow.Event` so an unknown field never reaches a viewer. Links with no tap get
  ambient rates from cAdvisor via Prometheus (`PROMETHEUS_URL`).
- **Credentials:** the server holds **zero user GitHub tokens**, not even for a minute. Sign-in (the
  no-scope Login app) reads id, login, name and avatar, then discards the token; desktop device-flow
  tokens are verified with `POST /applications/{client_id}/token`. Server work uses the GitHub App
  installation token (or `GITHUB_TOKEN`) and never acts for a user. **Never log a token, a prefix, or its
  length.**
- **`github_id` is the only user identity.** `auth.UpsertUser` never matches on username; a colliding
  username on another row is cleared, not merged. A session ends only on logout, expiry
  (`api.session.ttl_hours`), `POST /auth/logout-all` or purge.
- Browser requests: `__Host-oauth_state` binds the OAuth callback; `http.CrossOriginProtection` refuses
  cross-site mutations; a request with the session cookie must send `Content-Type: application/json`
  (bearer requests are exempt). Callback errors redirect with a fixed code, never reflected text.

## Data model

- Issue identity is `UNIQUE(repository_id, number)`; upserts conflict on that, not `github_id`.
- `updated_at` is GitHub's `updatedAt`, never import time. `first_seen_at` (insert) and
  `last_synced_at` (every write of GitHub data) are ours; NULL before 00019 means unknown.
- **Unlabelled issues are `UNCLASSIFIED`, not `BEGINNER`**; `difficulty_score` is NULL for them. Keep raw
  labels; derived tags don't replace them.
- **Notifications are written by the database** (`issues_notify_closed`, `pr_merged` triggers). Never add
  Go code that writes them; `internal/notification` only reads and marks read.
- Live counts (`GET /stats`) read `catalog_stats`, fed by statement-level triggers on `issues` (00043)
  and folded by ingestd. Never count per request. `/stats` is public to any origin (the flow site reads
  it) and exempt from rate limiting, so go-api keeps its answer for `api.stats.max_age_seconds`.

## Features with fixed rules

- **Issue bodies:** clients read them from GitHub; `GET /issues/{id}/body` is the fallback, from the
  bounded `issue_bodies` cache, misses capped by `api.bodies.fallback_misses_per_hour` (refused when
  Redis can't count) and fetched by ingestd on the `interactive` lane. go-api holds no GitHub credential
  and never writes a body.
- **XP** (`internal/xp`, `internal/progress`) is a ledger of verified merges. ingestd reads pull requests
  on the `interactive` lane (`internal/ingest/prs.go`), the watermark moves only when a pass completes,
  and a login whose `databaseId` isn't the account's `github_id` stores nothing. `xp.Plan` is pure; every
  number is an `xp.*` setting; `xp_events` is append-only. Quests and learning-path steps ride the same
  pass. Learning paths pick issues with `issue.Service.Pick`; a swapped-out issue goes to
  `learning_path_skips` and is never offered again on that path. Keep `/docs/xp` (web) in step.
- **Repository health** is stored raw and judged per request against `api.health.*`, so a threshold
  change needs no re-crawl.
- **Public profiles are opt-in** (`users.public_profile`); a private or missing account is the same 404.
  The leaderboard answers 404 until `api.leaderboards.enabled`. Both `no-store`.
- **Claims** (`internal/claim`): one active claim per issue, `api.claims.max_active` per person; the rule
  is one SQL predicate, `claim.active`.
- **Reports hide by count, never by query** (`internal/report`): targets reported by
  `api.reports.hide_after` people, or upheld, are re-read every `api.reports.hidden_refresh_seconds`. A
  failed read keeps the last set and counts `reports_hidden_refresh_errors_total`. `users.moderator` is
  set only by an operator in the database; nothing in the API writes it.
- **Rate limiting:** a fixed window per user or client (IPv6 by /64; `X-Forwarded-For` read from the right
  past `TRUSTED_PROXIES`, which must parse or the API won't start). The first request sets the expiry
  with `EXPIRE … NX`. Exempt: `/health`, `/health/detail`, `/metrics`, `GET /stats` only (`/auth/*` is
  limited). Fails open when Redis is down, skipping Redis after a failure except one probe per
  `api.cache.redis_retry_seconds`.
- **Themes** (`internal/theme`) are someone else's code: TOML checked against a fixed schema (an unknown
  key is an error), stylesheets checked token by token against an allowlist. `packages/theme` repeats
  every check on the same fixtures, with problems worded identically (`fixtures/problems`). Change both
  sides together.

## Not built

- **Live issue state on demand.** The server answers no freshness check; ingestd's tracked-issue check
  and the archive reader keep state correct.

## Gotchas

- After resolving a merge conflict, check that settings keys removed on `main` didn't come back.
