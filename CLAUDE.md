# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

**This is production code.** It runs against the real GitHub API with a real registered GitHub
App, crawls real repositories, and holds real people's GitHub identities. It is
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
- **`flowd`** — relay for the public live visualizer on `:8003`. See *Flow tap*.
- **`repoimport`** — offline importer for the repository metadata dataset. Streams the JSON
  array and stores only records that pass `eligible()`. Touches only `repositories` and
  `import_skipped`.
- **`migrate`** — one-shot; applies the embedded schema and exits.
- **`healthcheck`** — tiny binary used as the Docker `HEALTHCHECK` command.

Middleware chain, outermost first: RequestID → Logger → FlowTap → MaxBody → CORS → CSRF →
OptionalAuth → RateLimiter.
`/admin/` and `/webhooks/` are mounted outside that chain (they authenticate themselves and
have no browser callers).

Use `pgxpool` for all database access — never open individual connections. Always thread
`ctx context.Context` through to database, Redis and HTTP calls.

### Metrics (`backend/internal/metrics/`)
`prometheus/client_golang`, served at `GET /metrics` by **both** `api` (:8000) and `ingestd`
(:8002), and by `flowd` (:8003). Each binary registers only the families it produces
(`RegisterAPI`, `RegisterIngest`, `RegisterGitHub`, `RegisterDBPool`, `RegisterFlowPublisher`,
`RegisterFlowd`), so a process never exports another's gauge as a misleading zero.

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

### Flow tap (`backend/internal/flow/`, `internal/flowd/`, `cmd/flowd`)
The public live visualizer (a separate site, `flow.dizziedbliss.lol`) shows data moving
through the system. go-api and ingestd describe each hop as a `flow.Event` and call
`flow.Emit`; a `Publisher` batches them to Redis (`PUBLISH flow:events`) and flowd relays them
to viewers at `GET /stream` (SSE; `?format=ndjson` for `curl -N`). The taps reuse existing
seams: `middleware.FlowTap` (route pattern, status, size), `flow.QueryTracer` on every pool
(`db.New`; verb and first table only, never SQL or arguments), `flow.Transport` on the GitHub
client and the outbound client (operation name and status), the Redis cache wrapper, and one
emit after each committed crawl or discovery page (repository, counts, first title).

- **Redaction lives only in `flow.Emit`.** A context marked by `flow.MarkUser` (a request
  presenting a session, valid or not, and every `/auth/` request) or `flow.UserLinked` (the
  tracked-issue check) is published as random symbols of random length. Never build a label
  from anything a caller could not show a stranger, and never add an identifying field to
  `flow.Event` — `TestEventHasNoIdentifyingFields` pins the field set.
- **Fire-and-forget.** `Emit` never blocks or fails its caller; a full buffer, a Redis error or
  no listening flowd drops events and counts them (`flow_events_dropped_total{reason}`). Redis
  being down changes nothing else, and with no publisher installed (`migrate`, `repoimport`,
  flowd itself) `Emit` is a no-op.
- flowd relays at most `flow.max_events_per_second` labelled events; every event still counts
  towards its link's density line. It reads its settings as `skillissues_flow` and re-encodes
  every event through `flow.Event`, so an unknown field never reaches a viewer. Links nothing is
  instrumented on get ambient rates from cAdvisor via Prometheus (`PROMETHEUS_URL`).
- `flow.enabled` (live) turns the tap off. The visualizer site is not in this repository.

### Shared packages (`packages/`)
npm workspace members, imported as `@skillissues/*` (a real `node_modules` symlink, not a relative
path reaching outside the importing project). The web app was rebuilt on the first two; the desktop
app still uses the last three until its own rebuild.
- **`packages/client`** — the typed API client: response types, `ApiError` (`status`, `problems`;
  status 0 = never answered), abort support, a pluggable `Transport` (`fetchTransport` for the
  browser). Cursor paging always sends `?cursor=`, even empty: it is what selects the shuffled feed.
- **`packages/theme`** — user themes on the client: the TOML schema and stylesheet allowlist
  mirroring `backend/internal/theme`, run on the same fixtures (`packages/theme/fixtures`, CI runs
  both sides), the presets, and `compileTheme`/`applyTheme`, which re-validates a theme and applies
  it through constructed stylesheets scoped to the app's root. `npm test` runs the checks.
- **`packages/markdown`** — sanitised Markdown rendering (`markdown-it` + DOMPurify). Issue bodies
  are attacker-controlled; never hand-roll rendering in either client.
- **`packages/api-client`**, **`packages/ui`** — the previous client and component kit, used only
  by the desktop app now. Don't build anything new on them.

### Web (`web/src/`)
- **`landing/`** — the landing page, the owner's design. Its look is **locked**
  (`landing/tokens.module.css`) and outside the theme system: no user theme reaches it. Each open
  half of the zig-zag holds a live or explanatory filler (`landing/fillers/`). Fonts are
  self-hosted through `@fontsource` (Instrument Serif headings and nav, JetBrains Mono everything
  else, Krona One for the wordmark at 9% tracking over its 24:8 JetBrains Mono line).
- **`pages/`** — the written pages (`pages/content.ts`); every claim must describe what the code does.
  Pages marked `draft` say so on the page.
- **`app/`** — the signed-in app. Until its designs arrive, a plain working catalog.
- **`lab/`** — drafts for the owner to judge (`/lab/backdrop`); nothing there ships on `/`.
- **Prerendered**: `npm run build` renders `/` and every content page to HTML
  (`scripts/prerender.mjs`, `src/entry-server.tsx`); the client hydrates a page only when its
  `data-route` matches. nginx serves `shell.html` (no markup) for every other route.
- CSS Modules over custom properties; no Tailwind, no CSS-in-JS. Live counts poll `GET /stats`
  every 10s (React Query, paused in background tabs).
- nginx sends a CSP that allows fetching nothing from elsewhere (a theme stylesheet is someone
  else's code); widen it only deliberately.

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
Kaggle repository-metadata dataset      Discovery Sweeper (ingestd, off by default)
  (cmd/repoimport)                        GitHub repository Search over pushed-date
  ~4.2M records streamed,                 windows; full sweep monthly, incremental daily
  filtered at import                             │
        │  only passers stored                   │  only passers stored
        ▼                                        ▼
eligible repositories  (~278k after the first full crawl)  ◀── archive candidates
        │                                                       (looked up, passers stored)
        ▼                                                              ▲
Initial  — one first page of open issues per eligible repository       │
        │  hasNextPage?                                                │
        ▼                                                              │
Deep Diver — every remaining page, from the stored cursor, until exhausted
        │                                                              │
        ▼                                                              │
Freshness — once a repository's last sync is older than ingest.freshness_interval_seconds,
             fetch only what changed since its last completed pass: open issues
             updated since (upserted) and issues closed since (marked CLOSED)
                                                                       │
GH Archive reader (ingestd, off by default) — hourly public event files; issue events on
             stored repositories applied when newer, unknown repositories → candidates.
             While it is current, Freshness runs every ingest.archive.repair_interval_seconds.
```

The coordinator (`internal/ingest/coordinator.go`) runs one cycle every
`ingest.claim_interval_seconds`: recompute eligibility (throttled), reclaim expired leases
(throttled), seed never-ingested repositories, re-queue stale ones, then
check tracked (saved / in-progress) issues on GitHub, then claim and submit work to the Initial
and Deep pools, then — if `ingest.discovery.enabled` — start a due discovery sweep and claim its
windows for the Discovery pool. If `ingest.archive.enabled`, the same cycle (before the disk-guard
early return) queues newly ended archive hours, claims due ones for the Archive pool, and looks up
one batch of repository candidates. The first full crawl (every eligible repository through one pass)
took 3 days 23 h for 7.19M open issues; see §9 of the architecture doc.

### Rules

- **Every eligible repository enters the pipeline.** No repository priority, ranking, activity
  score or demand score anywhere in the crawl path. Ordering is FIFO on `next_retry_at`, and
  the freshness cycle orders by oldest successful sync — that is cycle fairness, not a
  judgement about repository value. Data-science ranking comes later, from the collected data.
- **Eligibility is abuse control and pacing, not quality.** The star floor exists because
  anyone can open an issue on a throwaway repository to burn crawl budget.
- **Some repositories are excluded by name.** `excluded_repositories` (migration 00028) lists
  repositories that pass every threshold and are still not worth crawling — an issue tracker
  used as a database, a public archive. A listed repository is ineligible (`excluded` in
  `eligibility_reason`): `ingest.RecomputeEligibility` applies the list, `cmd/repoimport`
  checks it before `eligible()` so a re-import cannot store one, and `ClaimJobs` only claims
  jobs of eligible repositories, so a job queued earlier does not run. The coordinator also
  deletes the stored issues of excluded, **archived** and `duplicate` repositories (never ones a user saved)
  on the closed-issue prune interval. Add one with
  `INSERT INTO excluded_repositories (full_name, reason) VALUES ('owner/name', 'why')`.
- **Only eligible repositories are stored.** The dataset is ~4.2M records and ~288k pass the
  filter; storing the rest cost ~2.8 GB of rows nothing is ever allowed to crawl. A full
  import pass is ~16 minutes and idempotent, so **loosening** a threshold means re-running
  `cmd/repoimport`, which is cheaper than warehousing the dataset.
- **Tightening is live.** `ingest.RecomputeEligibility` re-tests the rows that *are* stored
  against the current thresholds on a throttled cycle, so raising the star floor, or metadata
  going stale, takes repositories out of the crawl without a re-import. The rule it applies
  must stay identical to `ingest.Eligible` (used by `cmd/repoimport` and the Discovery Sweeper)
  — if one changes, both change. It never re-judges `not_found` or `duplicate` rows.
- **New repositories come from discovery.** The Discovery Sweeper (`internal/ingest/discovery.go`)
  searches `stars:>=min_stars fork:false archived:false pushed:<window>` and stores what passes
  `ingest.Eligible` and is not excluded; `EnqueueInitialJobs` takes it from there. A full sweep
  (whole pushed-within range) runs every `ingest.discovery.full_interval_seconds`, an incremental
  one (since the last `done` sweep) every `ingest.discovery.incremental_interval_seconds`. A
  window over GitHub's 1,000-result search cap is replaced by its halves down to
  `ingest.discovery.min_window_seconds`. Windows (`discovery_windows`) are a leased, fenced queue
  like `ingest_jobs`; a sweep with a failed window ends `failed` and does not advance the
  watermark. Discovery checks the shared `crawler` budget against its own, higher
  `ingest.discovery.rate_limit_floor`, so it yields before the crawl. `POST /sync/discovery`
  (ingestd, admin) starts a full sweep.
- **`github_database_id` is a repository's identity; `full_name` is display data.** Discovery
  and the first page of every crawl pass record GitHub's repository id. A row found by id under a
  new name is renamed. An id-less row that is the same repository as one holding its id (a
  dataset row imported under the post-rename name) is marked `duplicate`: never crawled, never
  re-judged, its unsaved issues pruned, nothing deleted. A name now held by a different
  repository is reported (`repository_identity_conflicts_total`) and left as it is.
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
  `ingest.closed_issue_retention_days`, an issue is deleted — never one in `saved_issues`
  (`saved_issues` would cascade).
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
- **The crawler never spends the interactive reserve.** Both lanes draw on one counter: GitHub
  counts per credential, and `CanProceed` reads every lane's row as one. The crawler lane (crawl,
  discovery) stops while `ingest.interactive_reserve_percent` of the limit is still left. Its floors
  are measured above that reserve. Work a user is waiting on (the tracked-issue check) runs on
  `interactive` and spends down to `ingest.rate_limit_floor`. A secondary limit pauses both lanes.
  New user-facing GitHub work goes on `interactive`; new background work goes on `crawler`. There
  is one GitHub App installation: never add an identity to get more limit.
- **Every job write is fenced on its lease.** Expired leases are reclaimed continuously
  (`ingest.lease_reclaim_interval_seconds`), so a stalled worker's job can be re-claimed while
  it still runs. Any write that finishes a job must match `status = 'running' AND
  lease_owner = <claimer>` and treat zero rows as `ErrLeaseLost` (rolling back the page's
  transaction) — never a bare `WHERE id = $1`.
- **Postgres is the queue.** `ingest_jobs` is the durable state store; Redis is not involved.
- **The archive carries freshness only while it is current.** The GH Archive reader
  (`internal/ingest/archive.go`) reads one gzipped file per hour from GH Archive, as a stream.
  IssuesEvent and IssueCommentEvent carry the whole issue, and those for stored, eligible
  repositories go through `writeIssueTx`: inserted, or updated only if newer than what is
  stored. A closed issue that was never stored is not inserted, and while the disk guard is on
  nothing is. Pull requests are skipped. `archive_hours` is a leased, fenced queue like
  `ingest_jobs`. While every hour up to `ingest.archive.max_lag_seconds` ago is done or
  `missing`, the freshness crawl drops to `ingest.archive.repair_interval_seconds` and the
  incremental discovery sweep stops. A `failed` hour holds coverage back, so the daily crawl
  resumes on its own (`archive_fallback`). Reset a failed hour with `UPDATE archive_hours SET
  status = 'pending', attempts = 0 WHERE hour = ...`. Archive writes never touch
  `repositories.last_synced_at` or `issues_synced_through`: the watermarks stay crawl
  watermarks.
- **Unknown repositories in the archive are candidates, not repositories.** `repo_candidates`
  holds repositories with issue activity that are not stored. They are looked up with
  `github.LookupRepositories` (aliased, ≤100 per call, on the crawler lane at discovery's floor)
  and stored through the Discovery Sweeper's `upsertDiscoveredTx`, so eligibility, exclusion and
  identity rules are the same. One that was not stored is looked up again after
  `ingest.archive.candidate_recheck_days`, if it was active since. A quiet one is deleted then
  (`PruneRepoCandidates`), which keeps the table from growing with every small repository.

### Not built

- **`'discovery'` job type** — `ingest_jobs.job_type` permits it, but the Discovery Sweeper has its
  own tables (`discovery_sweeps`, `discovery_windows`); nothing writes it. Tests use it as a job
  type no running coordinator claims.
- **Freshness job type** — stale repositories are re-queued as `job_type='initial'`; nothing
  writes `'freshness'`.
- **Pull requests from the archive** — the reader skips `PullRequestEvent` (and its `merged`
  action). Nothing tracks pull requests yet. Bodies in issue events only refresh bodies already
  cached; they never add one.

---

## Deployment

Single VPS, `compose.prod.yaml`, Caddy same-origin. Full runbook: `DEPLOYMENT.md` (local only).

**Local-only documents.** `DEPLOYMENT.md` and `docs/runbook.md` (the operator runbook) describe
how production is run, so they live only on the owner's machine: they are in `.gitignore`, were
removed from the index, and must **never be committed or pushed**. Do not `git add -f` them. The
same applies to any new operational document of that kind (host setup, access, rotation or
restore procedures): create it, add it to `.gitignore`, and keep it out of tracked files. Do not
link to them from tracked docs (`README.md`, `ARCHITECTURE.md`); code comments may name them. They
remain in earlier git history, which is not rewritten.

- `.github/workflows/ci.yml` gates everything: backend (vet/gofmt/build/test), web/desktop
  (typecheck then a real build, not just a type check), and a buildx build of the five
  deployed Dockerfile targets (`api`, `ingestd`, `flowd`, `migrate`, `web` — not `repoimport`) plus
  `docker compose config` on both compose files and `caddy validate` on the Caddyfile. It runs on pull requests and as the deploy
  gate, not on plain pushes, and a `changes` job (`dorny/paths-filter`) skips any job whose
  paths did not change. A skipped job counts as passing, so when adding a job, add its paths
  to the filter. `workflow_dispatch` runs everything.
- `.github/workflows/deploy.yml` runs on push to `main`: `ci` → `build-push` (builds and pushes
  `ghcr.io/dizziedbliss/skillissues-{api,ingestd,flowd,migrate,web}` tagged `sha-<short>` and
  `latest`) → `deploy` (scp + ssh, runs `scripts/deploy-remote.sh` on the host). A deploy cannot
  happen without CI passing. Rollback is `workflow_dispatch` with an older `sha-` tag — deploys
  always pin that, never `latest`. `build-push` attests provenance and cosign-signs each image by
  digest; `deploy-remote.sh` resolves the tag to a digest, runs `cosign verify` (signer: `deploy.yml`
  on `refs/heads/main`) and deploys that digest, so the host needs `cosign`. Actions are pinned to
  commit SHAs, workflows default to `contents: read`, `govulncheck` gates backend changes, and
  Dependabot (`.github/dependabot.yml`) keeps all of it current.
- Caddy takes the visitor's address from `CF-Connecting-IP` only when the peer is in
  Cloudflare's ranges (`trusted_proxies` in the `Caddyfile`), and sends go-api exactly that address
  in `X-Forwarded-For` (`header_up X-Forwarded-For {client_ip}`). That works with or without
  Cloudflare proxying: without it the header is ignored and the peer is the client. Keep the
  ranges current with https://www.cloudflare.com/ips/.
- Caddy is the only service publishing a public port. `handle_path /api/*` strips the prefix
  before proxying to `go-api` (whose routes have no `/api` prefix of their own) and returns
  **404 for `/metrics`, `/health/detail` and `/admin/*`** — those are internal only (admin is reached over ssh, see `DEPLOYMENT.md`); everything else goes
  to the `frontend` (nginx) image. A second site, `{$STREAM_DOMAIN}`, proxies only `GET /stream`
  to `flowd` (`flush_interval -1`) and returns 404 for everything else. `go-api`, `ingestd` and
  `flowd` have no published ports in prod.
- **Secrets are in git, encrypted.** `secrets/prod.env.sops` (SOPS + age, to the owner's key and
  the VPS's key; `.sops.yaml`). `deploy.yml` ships it as ciphertext and `deploy-remote.sh` runs
  `decrypt-env.sh` on the host to write `.env`; CI never holds a key. A present file that will not
  decrypt fails the deploy rather than falling back to a stale `.env`. `scripts/check-secrets-encrypted.sh`
  (CI) fails on a plaintext file under `secrets/` or a committed age private key. Never commit a plaintext secret.
- **Backups are encrypted on the host.** `scripts/backup-db.sh` pipes `pg_dump | gzip | age -r <public
  key>`; the private key is off the server, so the host can write backups but not read them.
  `BACKUP_DESTINATION` is required and has no default: `r2` uploads with rclone (retention is an R2
  lifecycle rule), `local` keeps encrypted files in `BACKUP_DIR` (pruned after `BACKUP_RETENTION_DAYS`)
  with **no off-site copy** unless the operator pulls them. It fails loudly (non-zero) on any missing
  setting, failed dump or failed upload, never keeps an unencrypted copy, and retries an unuploaded
  `r2` dump next run.
- Monitoring runs in both compose files: Prometheus, Grafana (six provisioned dashboards in
  `monitoring/grafana/dashboards/`), node-exporter, cAdvisor, postgres- and redis-exporter.
  Grafana (:3000) and Prometheus (:9090) bind to `127.0.0.1` and are reached over `ssh -L`.
  `deploy.yml` scp's `monitoring/` alongside the compose file. Runbook: `DEPLOYMENT.md`,
  *Monitoring*.
- **The services do not run as the database owner.** `go-api` connects as `skillissues_api`,
  `ingestd` as `skillissues_ingest` (migration 00029) and `flowd` as `skillissues_flow` (00033,
  `SELECT` on `settings` only): no ownership, no DDL, no `TRUNCATE`, no `COPY ... PROGRAM`.
  Passwords come from `API_DB_PASSWORD` / `INGEST_DB_PASSWORD` / `FLOW_DB_PASSWORD` via the
  `app-roles` one-shot (`db/app-roles.sql`). `migrate` and backups use the owner. Grants are
  explicit per table with no default privileges: **a migration that adds a table grants it** to
  the roles that use it, plus `USAGE` on its sequence — remember triggers run as the invoker, so
  a table a trigger writes needs granting to whoever fires it. `internal/migrate/roles_test.go`
  fails when a table is granted to neither role and not listed as deliberately ungranted.
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
`ingest.claim_interval_seconds`, `ingest.lease_seconds`, `api.session.ttl_hours`.
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

### Credentials and identity
- **The server holds zero user GitHub tokens.** Not encrypted, not hashed, not for a minute.
  Sign-in uses the **Login app**, a no-scope OAuth App: the server reads the numeric `id`,
  login and avatar, then discards the token. Desktop sign-in is a device flow on the same app,
  posted once to `POST /auth/github/device`, which asks GitHub (`POST
  /applications/{client_id}/token`) whether the Login app issued it. Fork, push and pull
  requests are the desktop client's job, with a token that stays on the user's machine.
- Server-side work (crawling) uses the server credential: the GitHub App's installation token,
  or `GITHUB_TOKEN` as a fallback. It never acts for a user.
- **`github_id` is the only identity.** A username is display data GitHub lets another account
  take, so `auth.UpsertUser` never matches on it; a colliding username on another row is
  cleared, not merged.
- A session ends only on logout, expiry (`api.session.ttl_hours`), `POST /auth/logout-all` or
  purge. Revoking the Login app on GitHub does not end it — there is no token to invalidate.
- Browser requests: a state cookie (`__Host-oauth_state`) binds an OAuth callback to the
  browser that began it; `http.CrossOriginProtection` refuses cross-site mutations; a request
  carrying the session cookie must send `Content-Type: application/json`. Bearer requests are
  exempt. Callback errors redirect with a fixed error code, never reflected text.
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
  `issue_closed` row in `notifications` for everyone who saved the issue, on the `OPEN` →
  `CLOSED` transition — whichever path closed it.
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

- **Balanced search**: `GET /issues?search=` matches four categories in precedence order:
  - **Repository:** the name contains the term.
  - **Title:** full-text match on the title and labels.
  - **Language:** the repository's language is the term.
  - **Author:** the author's login contains the term.

  Each category is its own indexed query, run concurrently (`issue/search_categories.go`). An
  issue appears once, in its first category. Each category is capped at
  `api.search.per_category_cap`, and the categories are interleaved rank by rank. Never fold
  them back into one query that ORs the matches across the join: no index serves it, and on the
  full catalog it ran past the query timeout.
  - Title and Author order a bounded sample (`api.search.sample_size`) by stars.
  - Repository and Language take at most `api.search.per_repo_cap` issues per repository.
  - Terms under 3 characters skip the substring categories, since trigram indexes can't serve
    them.
- **Catalog browsing reads in index order**: the catalog order (`difficulty_score`, `updated_at
  DESC`, `id DESC`) is exactly `issues_open_browse_idx` / `issues_open_difficulty_browse_idx`
  (migration 00035). Change one and you must change the other, or every page sorts every open issue.
  `issue.BrowseIssues` serves the first page and every `?cursor=` page as a run of ordered index
  ranges, never a UNION, which Postgres sorts in full. `?page` > 1 is the OFFSET query.
  A language with fewer than `api.issues.rare_language_threshold` open issues is sorted instead,
  because the planner cannot tell a rare language from a common one. Browse queries use
  `pgx.QueryExecModeExec` so they are never run from a cached generic plan.
- **Issue bodies are the client's to fetch; the server is the fallback.** Clients read bodies
  from GitHub (`ApiClient.loadIssueBody`) and call `GET /issues/{id}/body` only when GitHub
  refuses them for their own rate limit. That route serves `issue_bodies` (a bounded cache,
  `ingest.bodies.cache_max_mb`, saved issues' bodies never evicted). On a miss it is capped per
  caller (`api.bodies.fallback_misses_per_hour`, refused when Redis cannot count) and fetched by
  ingestd (`POST /bodies/{id}`, admin token) on the `interactive` lane. go-api holds no GitHub
  credential and never writes a body. Bodies are attacker-controlled; render them only through
  `packages/markdown`.
- **The default feed is a fair shuffle**: `sort=shuffle` (the cursor-paging default) walks
  repositories in `repositories.shuffle_key` order and shows each one's best open issue, so a
  repository with thousands of issues gets the same one slot as one with three. The goal is the
  whole backlog, not the biggest projects. ingestd redraws the order every
  `ingest.catalog.shuffle_interval_seconds`. In between it's the same for everyone, so pages
  cache. Don't add popularity weighting to it; a user who wants an order picks `easiest` or
  `newest`.
- **Catalog filters share one builder** (`issue/filters.go`): any-of within a list, all-of across
  filters. Labels match case-insensitively through `catalog_lower_labels()`, tech through the
  generated `repositories.tech`. A filtered page whose matches are rare (`issue.isRare`, under
  `api.issues.rare_language_threshold`) is read through the filters' own indexes and sorted,
  rather than by walking an order index. That count is itself bounded (`api.issues.rare_probe_ms`):
  matches too many to count quickly are too many to sort. A difficulty filter is restated on
  `difficulty_score` (fixed per difficulty by `sync.ClassifyLabels`) so the order indexes can range
  over it, and functional-dependency statistics (`issues_difficulty_dep`, migration 00046) keep the
  planner from multiplying the two conditions' selectivities. The shuffled feed walks repositories
  without a rarity count (only a languages-only filter checks the cached counts), finds each one's
  labelled issues through `issues_open_repo_labels_idx` (GIN over repository and labels), and after
  `api.issues.shuffle_walk_max_repos` repositories reads the rest of the page from the matches. go-api's connections run with `DB_WORK_MEM_MB` (32 in prod)
  so label bitmaps stay exact: a lossy one re-checked ~550k rows and took 8s. The dropdown values are `catalog_facets`, recomputed by
  ingestd every `ingest.catalog.facets_refresh_seconds`; never count them per request.
- **Live counts are kept, never counted per request**: `GET /stats` (open, closed since tracking
  began, arrived/closed today, indexed issues and repositories, signed-up developers, recent
  activity) reads `catalog_stats`. Statement-level triggers on `issues` (migration 00043) append one
  delta row per repository a statement touched, whichever path wrote it, and ingestd folds them in
  every `ingest.stats.rollup_seconds` and recounts exactly every `ingest.stats.recount_seconds`, in
  one REPEATABLE READ snapshot so nothing is counted twice (`ingest/stats.go`). `/stats` is public
  to any origin (the flow site reads it). `GET /repositories/lookup?name=owner/repo` says whether a
  repository is indexed and why not.
- **Catalog responses are cacheable only when signed out**: public `Cache-Control` + `ETag` and a
  Redis copy shared through `singleflight`. A signed-in response is ordered by the caller's
  languages and is `private, no-store`.
- **One account per GitHub id**: web and desktop sign-in both resolve to the `github_id`
  row. The case-insensitive unique index on `lower(username)` is why a username that moved to
  another account is cleared on the old row first.
- **Rate limiting**: Redis `INCR` counter per user (signed in) or per client (signed out; IPv6 keyed by /64, `X-Forwarded-For` read from the right past `TRUSTED_PROXIES`, which must parse or the API will not start) —
  180 / 1,000 per `api.rate_limit.window_seconds`. Only `/health`, `/health/detail` and
  `/metrics` are exempt; `/auth/*` (login) **is** limited. Fixed window: the first request sets
  the key's expiry with `EXPIRE … NX`, so later requests never push it back. Fails open
  when Redis is down — within `api.cache.redis_timeout_ms`, and after a failure Redis is skipped
  (`cache_skipped_total`) except one probe per `api.cache.redis_retry_seconds`.

- **Themes are someone else's code on your page**: a theme (`internal/theme`) is a TOML file of
  design tokens checked against a fixed schema — an unknown key is an error — plus an optional
  stylesheet checked token by token against an allowlist: no `url()` but base64 raster `data:`
  images, no `@import`/`@font-face`, no function off the list, no escapes. Themes are per account
  and shareable by id once public; making one private takes it off every other account. They
  restyle the signed-in app only, never the landing page. `packages/theme` repeats every check on
  the same fixtures (`packages/theme/fixtures`), and the web app's CSP blocks fetches regardless.
  Routes: `GET/POST /themes`, `GET/PUT/DELETE /themes/{id}`, `GET/PUT /me/theme`.

### Not built
- **Contributions, XP, levels and badges.** Removed with the no-user-tokens redesign
  (migration 00027): the old design awarded XP from PRs verified with a stored user token.
  The rebuild is client-side work with server-side verification from public GitHub data.
- **Live issue state on demand.** The server answers no freshness check; ingestd's
  tracked-issue check keeps saved issues' state correct, and the archive reader everyone else's.

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
