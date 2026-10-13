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

cd web && npm ci && npm run dev
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
```bash
cd web
npm ci            # not `npm install` — dependencies are pinned
npm run dev       # Vite dev server
npm run build     # tsc -b && vite build
npm run typecheck # tsc --noEmit
```

### Desktop (Electron)
```bash
cd desktop
npm ci
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
  array, stores every record, computes eligibility. Touches only `repositories` and
  `import_skipped`.
- **`migrate`** — one-shot; applies the embedded schema and exits.
- **`healthcheck`** — tiny binary used as the Docker `HEALTHCHECK` command.

Middleware chain, outermost first: RequestID → Logger → CORS → OptionalAuth → RateLimiter.
`/admin/` and `/webhooks/` are mounted outside that chain (they authenticate themselves and
have no browser callers).

Use `pgxpool` for all database access — never open individual connections. Always thread
`ctx context.Context` through to database, Redis and HTTP calls.

### Web (`web/src/`)
- **`App.tsx`** — top-level router and most of the UI. Guests see `LandingPage`;
  authenticated users get the challenge catalog.
- **`LandingPage.tsx`** — 3D CSS perspective hero with cursor-tilt tracking.
- **`index.css` / `landing.css` / `styles.css`** — theming. Palette: `#f7f6fb` background,
  `#1e1338` / `#241442` deep violet, `#7c3aed` accent.
- Avoid `any`; declare interfaces for API payloads.

Runs **React 19**. Dependencies are pinned to exact versions, so use `npm ci`.

### Desktop (`desktop/`)
`electron-vite` layout: `src/main/index.ts` (windows, IPC, git — TypeScript, non-strict),
`src/preload/index.ts` (context bridge — `contextIsolation: true` is mandatory),
`src/renderer/` (plain-JS `app.js`, `index.html`, `app.css`, bundled by Vite). Builds to
`out/main`, `out/preload`, `out/renderer`. The renderer imports the shared
`packages/markdown` source directly (Vite bundles it) — there is no checked-in vendor
bundle. Git operations run as async child processes so the UI never blocks.

Runs **Electron 44**.

### Database (`backend/internal/migrate/migrations/`)
Goose migrations, embedded into the binaries with `go:embed`. **This directory is the single
source of truth for the schema** — there is no separate `schema.sql`.

---

## Ingestion policy

Design of record: `SkillIssues_Ingestion_Architecture_Updated (1).md`. Read it before changing
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
Freshness — re-fetch page one once a repository's last sync goes stale
```

The coordinator (`internal/ingest/coordinator.go`) runs one cycle every few seconds:
recompute eligibility (throttled), seed never-ingested repositories, re-queue stale ones, then
claim and submit work to the Initial and Deep pools.

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
- **One failing repository must never block the others.** `NOT_FOUND` marks a repository
  unavailable; `RESOURCE_LIMITS_EXCEEDED` halves the page size and retries down to 1;
  secondary rate limits pause the lane; anything else gets exponential backoff and, past
  `ingest.max_attempts`, lands in `failed`.
- **Workers share one GitHub budget** (`github_budget`, lanes `crawler` and `interactive`) and
  claim work atomically with `FOR UPDATE SKIP LOCKED` plus a lease. Never assume a worker owns
  the whole API budget.
- **Postgres is the queue.** `ingest_jobs` is the durable state store; Redis is not involved.

### Not built

- **Discovery Sweeper / Sleeping Giant worker** — GitHub repository Search over
  created/pushed date windows to find repositories the dataset does not contain. Designed in
  §2 of the architecture doc, **not implemented**. New repositories currently arrive only from
  a dataset import. `ingest_jobs.job_type` already permits `'discovery'`; nothing writes it.

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
`ingest.enqueue_batch_size`, `ingest.graphql_page_size`, `ingest.sync_interval_seconds`,
`ingest.routine_workers`, `ingest.priority_workers`, `ingest.rate_limit_floor`.

- **Never hardcode an operational value.** Add a key to `settings.Registry` **and** a seeded
  row in a migration. A key in one but not the other falls back to its default and is logged
  as drift at startup.
- **Secrets never go in the `settings` table.** It is readable through an admin endpoint and
  reproduced in `settings_audit` and in database backups.
- Read values through `live.Current()` and hold the returned snapshot for the duration of an
  operation, so a mid-flight reload cannot change values underneath you.
- Change a value at runtime:
  ```bash
  curl localhost:8000/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"
  curl -XPUT localhost:8000/admin/config/ingest.sync_interval_seconds \
       -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 300}'
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
- **Unlabeled issues are not `BEGINNER`.** Absence of a difficulty label is `UNCLASSIFIED` —
  unknown difficulty, not easy difficulty — and `difficulty_score` is NULL for it.
- Preserve raw GitHub labels; derived tags are not a substitute.

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
- **Rate limiting**: Redis counter per window — 180 requests unauthenticated, 1,000
  authenticated. Health checks, `/metrics` and all `/auth/` paths are currently exempt;
  exempting the whole of `/auth/` means login is unlimited, which is a known issue.
- **XP**: base per difficulty (`BEGINNER` 75, `INTERMEDIATE` 140, `ADVANCED` 250) **plus**
  bonuses — `+25` merged, `+15` first contribution to that repo, `+10` multi-discipline. So
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
