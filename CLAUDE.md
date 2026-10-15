# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

**This is production code.** It runs against the real GitHub API with a real registered GitHub
App, crawls real repositories, and attributes real contributions to real user accounts. It is
not a prototype or a demo, and "it works on my machine" is not a finished state. Concretely,
that means:

- **Nothing swallows an error.** A discarded `err`, a `catch(() => [])`, an ignored `UPDATE`
  result, or a handler that returns success after a failed write is a bug, not a style
  preference. A failure the user cannot see is worse than a crash.
- **No operational value is hardcoded.** See *Configuration* below; thresholds, intervals,
  page sizes and worker counts are live settings, and a literal in the code is a defect.
- **Migrations are append-only.** Never edit one that has been applied anywhere.
- **Failures are observable.** `/metrics` and `/health/detail` must reflect reality — a
  component that is down must not be logged as connected.
- **Credentials are never logged**, not even a prefix or a length.

**This file describes what is true now.** Do not document planned features here — that is
how a previous version came to describe a Chi router, a `kaggle_etl` binary and a migration
policy that did not exist. Where a designed component is deliberately not built yet, this file
says so explicitly. Work in flight lives in the plan files under `~/.claude/plans/`.

---

## Commands

### Full stack (Docker — recommended)
```bash
cp .env.example .env          # first time only
docker compose up -d --build
docker compose ps
docker compose logs -f go-api ingestd
```

The `migrate` service applies the schema and exits; `go-api` and `ingestd` wait for it to
complete successfully. A fresh `up` needs **no manual SQL**.

### Production (VPS, `compose.prod.yaml`)
Pulls pre-built GHCR images; never builds on the host. Deployed by CI (`.github/workflows/deploy.yml`)
on every push to `main` — see *Deployment* below and `DEPLOYMENT.md` for the full runbook,
including first-time host setup, rollback and restore.

### Hybrid dev (databases in Docker, services native)
```bash
docker compose up -d postgres redis

export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"

cd backend
go run ./cmd/migrate          # apply schema first

export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="ghp_your_token_here"
export PORT=8000
go run ./cmd/api

npm ci            # root install — see *Web (React + TypeScript)* below
cd web && npm run dev
```

### Repository metadata import
```bash
cd backend
# Streams the Kaggle dataset (data/repo_metadata.json, ~3GB) and stores every
# record. Resumable and idempotent — safe to re-run, and a re-run is how a
# refreshed dataset lands.
DATABASE_URL=... go run ./cmd/repoimport --file ../data/repo_metadata.json
DATABASE_URL=... go run ./cmd/repoimport --batch-size 1000 --log-every 100000
```

### Backend (Go)
```bash
cd backend
go test ./...                 # all tests
go test ./internal/issue/...  # single package
go test -short ./...          # skips integration tests needing a live API
go fmt ./... && go vet ./...
```

### Web (React + TypeScript)
`web`, `desktop` and `packages/*` are one npm workspace (root `package.json`) — `npm ci`
(never `npm install` — dependencies are pinned) runs **once, at the repo root**, not inside
`web/` or `desktop/`. There is one lockfile for the whole tree, not one per package.
```bash
npm ci            # repo root — not `npm install`, and not inside web/
cd web
npm run dev       # Vite dev server
npm run build     # tsc -b && vite build
npm run typecheck # tsc --noEmit
```

### Desktop (Electron)
```bash
# npm ci already ran at the repo root above.
cd desktop
npm run dev       # electron-vite dev server, renderer HMR
npm run build && npm start   # build then run the built app (no HMR)
npm run check     # tsc --noEmit over src/main, src/preload
npm run dist      # build, then electron-builder release artifacts
```

---

## Architecture

### Backend (`backend/`)
Entrypoints under `backend/cmd/`:

- **`api`** — HTTP API on `:8000`. Uses the **standard library `http.NewServeMux`** with Go
  1.22 method patterns (`"GET /issues"`) and `{id}` path params. There is no third-party
  router; do not add one without a reason routing actually causes.
- **`ingestd`** — GitHub GraphQL crawler on `:8002`. Owns the ingestion coordinator, its
  worker pools and the claim/cycle loop. See *Ingestion policy*.
- **`repoimport`** — offline importer for the repository metadata dataset. Streams the JSON
  array and stores only records that pass `eligible()`. Touches only `repositories` and
  `import_skipped`.
- **`migrate`** — one-shot; applies the embedded schema and exits.
- **`healthcheck`** — tiny binary used as the Docker `HEALTHCHECK` command.

Middleware chain, outermost first: RequestID → Logger → CORS → OptionalAuth → RateLimiter.
`/admin/` and `/webhooks/` are mounted outside that chain (they authenticate themselves and
have no browser callers).

Use `pgxpool` for all database access — never open individual connections. Always thread
`ctx context.Context` through to database, Redis and HTTP calls.

### Metrics (`backend/internal/metrics/`)
`prometheus/client_golang`, served at `GET /metrics` by **both** `api` (:8000) and `ingestd`
(:8002). Each binary registers only the families it produces (`RegisterAPI`,
`RegisterIngest`, `RegisterGitHub`, `RegisterDBPool`), so a process never exports another's
gauge as a misleading zero.

- HTTP metrics are labelled by the **matched `ServeMux` pattern** (`route="GET /issues/{id}"`),
  never the raw path — `middleware.RecordRoute` must wrap each mux directly, because the mux
  sets `r.Pattern` on a request the outer middleware only sees as a clone. Unmatched paths are
  `route="unmatched"`.
- Every outbound GitHub call goes through `metrics.InstrumentedTransport` on the shared
  `github.Client`; tag new client methods with `metrics.WithOperation(ctx, "Name")`.
- ingestd's Postgres-backed gauges (`ingest_jobs`, `repositories`,
  `github_rate_limit_remaining{lane}`) refresh every `ingest.metrics_refresh_seconds`, not per
  claim cycle and not per scrape.
- A failure you log on an operational path should also increment a counter
  (`ingest_cycle_errors_total`, `github_budget_errors_total`, …) so it shows on a dashboard.

### Shared packages (`packages/`)
npm workspace members, imported by both clients as `@skillissues/*` (a real `node_modules`
symlink, not a relative path reaching outside the importing project):
- **`packages/markdown`** — sanitised Markdown rendering (`markdown-it` + DOMPurify). Issue
  bodies are attacker-controlled; never hand-roll rendering in either client.
- **`packages/api-client`** — one typed client for every API endpoint: the `Issue` /
  `Contribution` / `Profile` types, `ApiError` (keeps `status` + `detail`, so 401 ≠ 429 ≠
  network), `AbortController` support, and the recommendations ETag/localStorage
  revalidation. Pluggable `Transport`: `createFetchTransport` (web) and `createIpcTransport`
  (desktop, routed through the existing preload bridge).
- **`packages/ui`** — shared React components: `Modal` (body scroll lock, Escape-to-close,
  focus trap, backdrop click — fixes the "background scrolls behind the popup" bug for every
  overlay in both clients), `ConfirmDialog`, `Toast`, `IssueCard`/`IssueList`,
  `WorkingList`/`SubmissionList`, `DifficultyTabs`, `SectionTabs`, `Badge`, `ProgressBar`,
  `Skeleton`. Platform differences (desktop's fork/clone/editor buttons, web's "Open in
  Desktop App" deep link) come in as props/slots passed to these, never as a forked copy of
  the component.

### Web (`web/src/`)
- **`App.tsx`** — top-level router and most of the UI. Guests see `LandingPage`;
  authenticated users get the challenge catalog.
- **`LandingPage.tsx`** — 3D CSS perspective hero with cursor-tilt tracking.
- **`index.css` / `landing.css` / `styles.css`** — theming. Palette: `#f7f6fb` background,
  `#1e1338` / `#241442` deep violet, `#7c3aed` accent.
- Avoid `any`; declare interfaces for API payloads.

Runs **React 19**.

### Desktop (`desktop/`)
`electron-vite` layout: `src/main/index.ts` (windows, IPC, git — TypeScript, non-strict),
`src/preload/index.ts` (context bridge — `contextIsolation: true` is mandatory),
`src/renderer/` (React + TypeScript on `packages/ui`/`packages/api-client`, entry
`src/renderer/src/main.tsx`; it reaches the API only via the preload bridge). Builds to
`out/main`, `out/preload`, `out/renderer`. Git operations run as async child processes so
the UI never blocks.

Runs **Electron 44**.

### Database (`backend/internal/migrate/migrations/`)
Goose migrations, embedded into the binaries with `go:embed`. **This directory is the single
source of truth for the schema** — there is no separate `schema.sql`.

### Docker images
One root **`Dockerfile`** with named targets (`api`, `ingestd`, `migrate`, `web`, `repoimport`),
built from the **repo root** — not `backend/` or `web/` — because the `web` target needs to
see the npm workspace (`packages/*`), which a narrower context can't reach. Root
**`.dockerignore`** keeps the build context small despite that (excludes `data/`, `.git`,
`node_modules`, secrets). `compose.prod.yaml` (prod) only ever pulls pre-built images from
GHCR and never builds anything on the host; the deploy pipeline builds against these targets.
**`web/Dockerfile`** is the one exception — a dev-only Vite HMR image used by `compose.yaml`'s
`frontend` service, also built from the repo root for the same workspace reason. Don't add a
third one back.

---

## Ingestion policy

Design of record: `docs/ingestion-architecture.md`. Read it before changing
anything in `backend/internal/ingest/`.

**Collect broadly first; analyse later.** Coverage comes before prioritisation. Scope is open
issues, most-recently-updated first — not every comment, reaction, event or closed issue.

### The pipeline

```
Kaggle repository-metadata dataset  (cmd/repoimport)
        │  ~4.2M records streamed, filtered at import; only passers stored
        ▼
eligible repositories  (~288k at the current thresholds)
        │
        ▼
Initial  — one first page of open issues per eligible repository
        │  hasNextPage?
        ▼
Deep Diver — every remaining page, from the stored cursor, until exhausted
        │
        ▼
Freshness — once a repository's last sync is older than ingest.freshness_interval_seconds,
             fetch only what changed since its last completed pass: open issues
             updated since (upserted) and issues closed since (marked CLOSED)
```

The coordinator (`internal/ingest/coordinator.go`) runs one cycle every
`ingest.claim_interval_seconds`: recompute eligibility (throttled), reclaim expired leases
(throttled), seed never-ingested repositories, re-queue stale ones, then
check tracked (saved / in-progress) issues on GitHub, then claim and submit work to the Initial
and Deep pools.

### Rules

- **Every eligible repository enters the pipeline.** No repository priority, ranking, activity
  score or demand score anywhere in the crawl path. Ordering is FIFO on `next_retry_at`, and
  the freshness cycle orders by oldest successful sync — that is cycle fairness, not a
  judgement about repository value. Data-science ranking comes later, from the collected data.
- **Eligibility is abuse control and pacing, not quality.** The star floor exists because
  anyone can open an issue on a throwaway repository to burn crawl budget.
- **Only eligible repositories are stored.** The dataset is ~4.2M records and ~288k pass the
  filter; storing the rest cost ~2.8 GB of rows nothing is ever allowed to crawl. A full
  import pass is ~16 minutes and idempotent, so **loosening** a threshold means re-running
  `cmd/repoimport`, which is cheaper than warehousing the dataset.
- **Tightening is live.** `ingest.RecomputeEligibility` re-tests the rows that *are* stored
  against the current thresholds on a throttled cycle, so raising the star floor, or metadata
  going stale, takes repositories out of the crawl without a re-import. The rule it applies
  must stay identical to `cmd/repoimport`'s `eligible()` — if one changes, both change.
- **Unknown is not a negative.** A NULL metadata field means "not imported yet", never
  "fails the check" — the same principle as unlabeled issues not being `BEGINNER`.
- **Cursors advance only after the rows they describe are committed**, in the same
  transaction, along with `repositories.last_synced_at`. Never mark a repository complete on a
  truncated run.
- **The watermark only ever trails the truth.** `repositories.issues_synced_through` is the
  *enqueue* time of the last pass that completed (`ingest_jobs.pass_started_at`), set in the
  transaction of the pass's last page. A freshness pass fetches changes after it (`ingest_jobs.since`);
  NULL means no pass has completed and the next one is a full crawl. A pass pages two
  connections in lockstep (`cursor`/`open_done`, `closed_cursor`/`closed_done`) and is done
  only when both are exhausted or the page cap is hit.
- **Closed issues are detected, never inferred.** The crawl asks GitHub for issues closed since
  the watermark and marks exactly those `CLOSED`. "Not seen in this pass" is not evidence of
  closure — the page cap means many open issues are never seen. Issues a user saved or is
  working on are also re-checked directly (`ingest.tracked_issue_check_seconds`), so their
  closure shows up within that interval rather than at the repository's next pass.
- **Closed issues are pruned, unless a user holds them.** Once closed on GitHub for
  `ingest.closed_issue_retention_days`, an issue is deleted — never one in `saved_issues` or
  with any `contributions` row (`saved_issues` would cascade; a merged contribution is the
  user's permanent record).
- **Repository metadata is live once crawled.** The first page of every pass writes GitHub's
  current stars, forks, watchers, open-issue count, archived flag and last push over the
  dataset snapshot (`repositories.metadata_refreshed_at`; NULL = still the snapshot), so
  eligibility follows live values. `issue_count` stays the dataset's open+closed total — the
  import's `no_issues` rule reads it; `open_issue_count` is the live open count.
- **One failing repository must never block the others.** `NOT_FOUND` marks a repository
  unavailable; `RESOURCE_LIMITS_EXCEEDED` halves the page size and retries down to 1;
  secondary rate limits pause the lane; anything else gets exponential backoff and, past
  `ingest.max_attempts`, lands in `failed`.
- **Workers share one GitHub budget** (`github_budget`, lanes `crawler` and `interactive`) and
  claim work atomically with `FOR UPDATE SKIP LOCKED` plus a lease. Never assume a worker owns
  the whole API budget.
- **Every job write is fenced on its lease.** Expired leases are reclaimed continuously
  (`ingest.lease_reclaim_interval_seconds`), so a stalled worker's job can be re-claimed while
  it still runs. Any write that finishes a job must match `status = 'running' AND
  lease_owner = <claimer>` and treat zero rows as `ErrLeaseLost` (rolling back the page's
  transaction) — never a bare `WHERE id = $1`.
- **Postgres is the queue.** `ingest_jobs` is the durable state store; Redis is not involved.

### Not built

- **Discovery Sweeper / Sleeping Giant worker** — GitHub repository Search over
  created/pushed date windows to find repositories the dataset does not contain. Designed in
  §2 of the architecture doc, **not implemented**. New repositories currently arrive only from
  a dataset import. `ingest_jobs.job_type` already permits `'discovery'`; nothing writes it.
- **Freshness job type** — stale repositories are re-queued as `job_type='initial'`; nothing
  writes `'freshness'`.

---

## Deployment

Single VPS, `compose.prod.yaml`, Caddy same-origin. Full runbook: `DEPLOYMENT.md`.

- `.github/workflows/ci.yml` gates everything: backend (vet/gofmt/build/test), web/desktop
  (typecheck then a real build, not just a type check), and a buildx build of the four
  deployed Dockerfile targets (`api`, `ingestd`, `migrate`, `web` — not `repoimport`) plus
  `docker compose config` on both compose files. It runs on pull requests and as the deploy
  gate, not on plain pushes, and a `changes` job (`dorny/paths-filter`) skips any job whose
  paths did not change. A skipped job counts as passing, so when adding a job, add its paths
  to the filter. `workflow_dispatch` runs everything.
- `.github/workflows/deploy.yml` runs on push to `main`: `ci` → `build-push` (builds and pushes
  `ghcr.io/dizziedbliss/skillissues-{api,ingestd,migrate,web}` tagged `sha-<short>` and
  `latest`) → `deploy` (scp + ssh, runs `scripts/deploy-remote.sh` on the host). A deploy cannot
  happen without CI passing. Rollback is `workflow_dispatch` with an older `sha-` tag — deploys
  always pin that, never `latest`.
- Caddy is the only service publishing a public port. `handle_path /api/*` strips the prefix
  before proxying to `go-api` (whose routes have no `/api` prefix of their own) and returns
  **404 for `/metrics` and `/health/detail`** — those are internal only; everything else goes
  to the `frontend` (nginx) image. `go-api` and `ingestd` have no published ports in prod.
- Monitoring runs in both compose files: Prometheus, Grafana (six provisioned dashboards in
  `monitoring/grafana/dashboards/`), node-exporter, cAdvisor, postgres- and redis-exporter.
  Grafana (:3000) and Prometheus (:9090) bind to `127.0.0.1` and are reached over `ssh -L`.
  `deploy.yml` scp's `monitoring/` alongside the compose file. Runbook: `DEPLOYMENT.md`,
  *Monitoring*.
- Grafana's Postgres datasource and postgres-exporter connect as **`metrics_ro`** (migration
  00017): read-only, 15s `statement_timeout`, explicit per-table grants. **Never grant it a
  credential column**, and a new table a dashboard needs must be granted in a new migration.
  Its password comes from `METRICS_DB_PASSWORD` via the `metrics-role` one-shot service.
- Two disk guardrails exist specifically because the host is 40GB: `ingest.max_db_size_mb`
  (coordinator stops enqueuing/claiming once `pg_database_size()` exceeds it) and
  `ingest.max_pages_per_repo` (caps the Deep Diver per repository regardless of `hasNextPage`).
  Both are live settings — see *Configuration* below.

---

## Conventions

These encode decisions already made. Please do not quietly reverse them.

### Configuration
Three layers, in precedence order:

| Layer | Source | Changeable | Holds |
|---|---|---|---|
| 1. Bootstrap | env (`internal/config`) | restart | `DATABASE_URL`, `REDIS_URL`, `PORT`, `FRONTEND_URL` |
| 2. Secrets | env (`internal/config`) | restart | tokens, client secrets, webhook secret |
| 3. Operational | `settings` table (`internal/settings`) | **live** | limits, page sizes, TTLs, intervals, worker counts, thresholds |

**Nothing operational is fixed.** If a number changes how the system behaves — a threshold, an
interval, a batch size, a page size, a worker count, a cap — it is a setting, not a literal.
The precedent is `ingest.eligibility.min_stars`, `ingest.eligibility.pushed_within_days`,
`ingest.eligibility.recheck_seconds`, `ingest.freshness_interval_seconds`,
`ingest.enqueue_batch_size`, `ingest.graphql_page_size`, `ingest.routine_workers`, `ingest.priority_workers`, `ingest.rate_limit_floor`,
`ingest.max_pages_per_repo`, `ingest.max_db_size_mb`, `ingest.metrics_refresh_seconds`,
`ingest.claim_interval_seconds`, `ingest.lease_seconds`, `api.issue_body.cache_max_rows`.
Every key with its default and bounds: `docs/settings.md`.

- **Never hardcode an operational value.** Add a key to `settings.Registry` **and** a seeded
  row in a migration. A key missing from the table falls back to its default and is logged
  as missing on every settings load.
- **Secrets never go in the `settings` table.** It is readable through an admin endpoint and
  reproduced in `settings_audit` and in database backups.
- Read values through `live.Current()` and hold the returned snapshot for the duration of an
  operation, so a mid-flight reload cannot change values underneath you.
- Change a value at runtime:
  ```bash
  curl localhost:8000/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"
  curl -XPUT localhost:8000/admin/config/ingest.routine_workers \
       -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 4}'
  ```
  `psql` works too, and hits the same validation trigger — which matters when a bad value has
  made the API itself unhealthy.
- Bounds live in the database trigger, which is the authoritative boundary; the registry
  mirrors them so the API can reject early with a useful message.
- Anything gated on concurrency must use `worker.Limiter`, not a fixed goroutine count, or the
  setting cannot actually be resized.

### Migrations
- **Never edit a migration that has been applied anywhere.** Add a new one.
- Keep migrations idempotent (`IF NOT EXISTS`) and pair every `Up` with a `Down`.

### Credentials and attribution
- Server-side work (crawling, on-demand reads) uses the server credential.
- Fork, push and pull-request creation use **the user's own** GitHub authorization.
- **Never fall back to the server token for a write.** It attributes the user's contribution
  to the server identity.
- Never log a token, a prefix of one, or its length — a prefix leaks secret bytes and the
  length identifies the credential type.

### Data model
- Issue identity is `UNIQUE(repository_id, number)`. Upserts must conflict on that, not on
  `github_id`.
- Issue `updated_at` comes from GitHub's `updatedAt`, never from import time or a DB default.
  When *we* fetched it is separate: `first_seen_at` (set on insert, never changed) and
  `last_synced_at` (every write of GitHub data to the row — crawl, on-demand check, webhook).
  Both are NULL for rows that predate migration 00019; that means unknown.
- **Unlabeled issues are not `BEGINNER`.** Absence of a difficulty label is `UNCLASSIFIED` —
  unknown difficulty, not easy difficulty — and `difficulty_score` is NULL for it.
- Preserve raw GitHub labels; derived tags are not a substitute.
- **Notifications are written by the database.** The `issues_notify_closed` trigger inserts an
  `issue_closed` row in `notifications` for everyone who saved the issue or has a `STARTED` /
  `PR_OPEN` contribution on it, on the `OPEN` → `CLOSED` transition — whichever path closed it.
  Never add a Go code path that writes them for issue closure; `internal/notification` only
  reads and marks them read (`GET /notifications`, `POST /notifications/{id}/read`,
  `POST /notifications/read-all`). No client shows them yet.

### Frontend
- Use a maintained Markdown library with sanitisation; never hand-roll Markdown rendering.
  Issue bodies are attacker-controlled — anyone can open an issue on a crawled repo.

### Testing
- **Tests must pass with Redis stopped.** Redis is cache and rate limiting only; sessions and
  authentication fall through to Postgres.

---

## Key design decisions

- **Balanced search**: `GET /issues?search=` uses a PostgreSQL window CTE
  (`ROW_NUMBER() OVER (PARTITION BY match_category)`) and caps results per category
  (`api.search.per_category_cap`) rather than flooding one. Categories: Repository, Title,
  Language, Author.
- **Unified account linking**: a case-insensitive unique index on `lower(username)` merges
  PAT-login and OAuth-login into one user record.
- **Rate limiting**: Redis `INCR` counter per user (signed in) or per IP (signed out) —
  180 / 1,000 per `api.rate_limit.window_seconds`. Only `/health`, `/health/detail` and
  `/metrics` are exempt; `/auth/*` (login) **is** limited. Fixed window: the first request sets
  the key's expiry with `EXPIRE … NX`, so later requests never push it back. Fails open
  when Redis is down — within `api.cache.redis_timeout_ms`, and after a failure Redis is skipped
  (`cache_skipped_total`) except one probe per `api.cache.redis_retry_seconds`.
- **XP**: base per difficulty (`BEGINNER` 75, `INTERMEDIATE` 140, `ADVANCED` 250) **plus**
  bonuses — `+25` merged, `+15` for the user's first or second merge in that repo, `+10`
  multi-discipline (`UNCLASSIFIED` counts as `BEGINNER` base). So
  a BEGINNER issue can award up to 125, not 75. Awarded only after GitHub confirms the PR is
  merged and the author login matches.
- **Levels** follow a quadratic curve, not a flat table: each level costs
  `150 + level * 150` more than the previous one.

---

## Branching and commits

Branch naming: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `refactor/<topic>`,
`chore/<topic>`.

> Note: a branch named `refactor` already exists, which blocks `refactor/<topic>` — git
> cannot have both a ref and a directory of refs at the same path. Use `chore/<topic>` for
> refactor work until that branch is gone.

Commit style: [Conventional Commits](https://www.conventionalcommits.org/) —
`feat(search): …`, `fix(electron): …`, `perf(ingestd): …`.

**Push after every commit.** `git commit` and `git push` are one step on the working branch,
so work is never stranded on one machine. Never push directly to `main` — PRs gate that.
