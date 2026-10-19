# Changelog

All notable changes to **SkillIssues** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed
- Grafana. Every panel was run through Grafana's own query API against production: 107 of
  121 queries returned values. The rest are fixed:
  - **Product dashboard:** six panels queried the contributions and XP tables migration 00027
    removed. They now show saves (7d, per day, users who saved), saved issues by state,
    notifications sent and read, and the issue body cache.
  - **Timeout:** "Open issues by difficulty" counted 7.5M rows past `metrics_ro`'s 15 s
    timeout. It now reads Postgres column statistics, labelled as an estimate.
  - **Empty panels:** panels that are legitimately empty say so, instead of a bare "No data".
  - **Grant:** migration 00039 gives `metrics_ro` the body cache's bookkeeping columns, never
    the bodies.

### Added
- Ready for Cloudflare in front of the site. Caddy believes `CF-Connecting-IP` only from
  Cloudflare's published ranges, and passes go-api exactly the resolved visitor in
  `X-Forwarded-For`. Without that, every visitor behind one Cloudflare edge would share one
  rate-limit bucket.
  - A direct client cannot spoof its address either way: tested with a forged
    `CF-Connecting-IP` from an untrusted peer and from a trusted one.
  - Nothing changes until DNS is proxied.

### Added
- The issue body fallback (migration 00038). `GET /issues/{id}/body` existed only in the clients:
  the web and desktop apps called it and got 404, so issue descriptions never loaded.
  - It serves a bounded cache (`issue_bodies`, trimmed to `ingest.bodies.cache_max_mb`, never a
    saved issue's body).
  - On a miss, ingestd fetches the body with the server credential on the interactive lane
    (`POST /bodies/{id}`), capped at `api.bodies.fallback_misses_per_hour` per caller. The cap is
    refused, not skipped, when Redis is down.
  - The archive reader refreshes cached bodies. `ApiClient.loadIssueBody` reads from GitHub first
    and uses the server only when GitHub refuses the caller's own rate limit.
  - `body_fetches_total{outcome}` on ingestd.
- GH Archive reader in `ingestd` (migration 00037). **Off by default** (`ingest.archive.enabled`).
  - It reads GH Archive's hourly public event files as a stream and applies `IssuesEvent` and
    `IssueCommentEvent` issues to stored, eligible repositories, with no GitHub API call. An issue
    is inserted, or updated only when newer than what is stored. Pull requests are skipped.
  - Hours are a leased queue (`archive_hours`). Repositories with issue activity that are not
    stored become `repo_candidates`; they are looked up in batches with the new
    `github.LookupRepositories` and stored through the Discovery Sweeper's rules.
  - While the archive is current (`ingest.archive.max_lag_seconds`), the freshness crawl runs
    every 30 days (`ingest.archive.repair_interval_seconds`) instead of daily, and the
    incremental discovery sweep stops. If it falls behind, both resume by themselves.
  - New metrics: `archive_*`. Grafana *Ingestion* has an Archive row.
  - A real hour (2026-10-18 15:00 UTC, 19,226 events) applies in 1.2 s.
- An interactive reserve on the GitHub budget (migration 00036,
  `ingest.interactive_reserve_percent`, 30%). The crawl and discovery stop while that share of the
  rate limit is still left, and their floors are now measured above it. The tracked-issue check moves
  to the `interactive` lane and spends into the reserve, down to `ingest.rate_limit_floor`, so a
  saturated crawl no longer delays it. The two lanes are read as the one counter GitHub keeps
  per credential, and a secondary limit pauses both.
- Cursor paging for the catalog: `GET /issues?cursor=` returns `{"issues", "next_cursor"}` and
  costs the same on every page, however deep. `?page` still works, in the same order; the order now
  breaks ties on id. Signed-out `/issues` responses carry `Cache-Control` and an `ETag`
  (`api.cache.public_max_age_seconds`, `api.cache.public_stale_seconds`), and answer
  `If-None-Match` with 304. Signed-in responses are `private, no-store`. `ApiClient.getIssuesPage`
  pages by cursor.

- Three more excluded repositories (migration 00034): `type-challenges/type-challenges`,
  `haizlin/fe-interview` and `microsoft/JARVIS`, none of which is work a contributor can pick up.
- Live flow tap and `flowd`, the backend of the public data-flow visualizer. go-api and ingestd
  publish one event per hop — an HTTP request (route pattern, status, size), a statement (verb and
  table), a GitHub call (operation, status), a Redis call, a committed crawl or discovery page
  (repository, counts, first title) — to Redis, fire-and-forget. flowd relays up to
  `flow.max_events_per_second` of them with their labels, sends per-link density for all of them,
  reads cAdvisor rates for uninstrumented links, and serves `GET /stream` (SSE, or ndjson for
  `curl -N`) on `STREAM_DOMAIN`. Anything tied to a signed-in session leaves the process as random
  symbols. On by default (`flow.enabled`). Migration 00033 adds ten `flow.*` settings and the
  `skillissues_flow` role (reads `settings` only); new required `.env` keys `FLOW_DB_PASSWORD` and
  `STREAM_DOMAIN`. Grafana *Ingestion* dashboard has a Flow row.
- Discovery Sweeper in `ingestd`: GitHub repository Search (`stars:>=min_stars fork:false
  archived:false pushed:<window>`) finds repositories the dataset does not contain and stores the
  eligible ones, which then enter the crawl like imported ones. A full sweep covers the whole
  pushed-within range every 30 days, and an incremental sweep covers what was pushed since the last
  completed sweep every day. Windows over GitHub's 1,000-result cap are halved down to one minute.
  **Off by default** (`ingest.discovery.enabled`); `POST /sync/discovery` on ingestd starts a full
  sweep. Migration 00032; Grafana *Ingestion* dashboard has a Discovery row.
- `repositories.github_database_id`: GitHub's stable repository id, filled by the sweeper and by
  the first page of every crawl pass. A renamed repository is followed by id; an id-less row that
  turns out to be the same repository as one holding its id is marked `duplicate` (not crawled,
  nothing deleted; its unsaved issues are pruned).
- Measured first full crawl recorded in `docs/ingestion-architecture.md` §9: 3 days 23 h,
  284,478 repositories, 7.19M open issues.
- Grafana *Ingestion* dashboard: tables of jobs by type and status, repositories running now, and
  the next pending jobs.
- `excluded_repositories`: repositories that are never crawled, seeded with four (three issue
  trackers used as databases, one public archive). Applied by the eligibility recompute, the
  importer and job claiming; their stored issues, and those of archived repositories, are
  pruned unless a user saved them.

### Fixed
- A cold catalog page took 35 s and returned 502. Nothing indexed the catalog order, so every cache
  miss sorted every open issue; the query outlived the 30 s write timeout and kept running after
  the connection dropped. Signed-in callers, who skip the cache and sort by preferred language,
  paid it on every page. Migration 00035 adds `issues_open_browse_idx` and
  `issues_open_difficulty_browse_idx` (built `CONCURRENTLY`), and `repositories_lower_language_idx`
  for language statistics. The first page and every cursor page now read in index order, a few
  milliseconds on 2M open issues. A rare language (`api.issues.rare_language_threshold`, counted
  once per `api.issues.language_count_ttl_seconds`) is sorted instead, since walking the index for
  it scans most of it.
- Catalog queries have a deadline, `api.issues.query_timeout_ms` (10 s), answered with 503. Signed-out
  requests that miss the cache together share one query instead of each running their own.
- `GET /issues/{id}` returned 404 for any database error; it is now 404 only for a missing issue. A
  failed catalog query, or a failed profile load for a signed-in browse, is logged instead of being
  answered silently.
- Postgres runs with `random_page_cost=1.1` (`PG_RANDOM_PAGE_COST`), since the host's disk is SSD.
  The default of 4 chose a sequential scan of every issue over a few thousand index lookups
  (990 ms vs 14 ms).
- Caddy compresses with zstd as well as gzip.

### Removed
- `cache.InvalidateHotIssues`, which nothing called.

### Security
- Production secrets are stored encrypted in the repo (`secrets/prod.env.sops`, SOPS + age) and
  decrypted on the host at deploy time; CI never sees plaintext. CI fails on a plaintext file under
  `secrets/` or a committed age private key.
- Database backups are encrypted with age to a key held off the server; the old unencrypted local
  `.sql.gz` dumps are no longer written. Required `.env` keys: `BACKUP_AGE_RECIPIENT` and
  `BACKUP_DESTINATION` (`local` keeps encrypted files on the server to be pulled elsewhere; `r2`
  uploads to Cloudflare R2 and also needs `R2_ENDPOINT`, `R2_BUCKET`, `R2_ACCESS_KEY_ID`,
  `R2_SECRET_ACCESS_KEY`).
- Images are signed and verified. The deploy builds, attests provenance for and cosign-signs each
  image by digest; the host verifies the signature (`deploy.yml` on `main`) and runs that digest,
  so a pushed or retagged image does not deploy. **The host needs `cosign`** (DEPLOYMENT.md,
  step 0). Rollback to a tag built before this change fails verification (it is unsigned).
- CI: Actions pinned to commit SHAs, `contents: read` by default, a `govulncheck` job, and
  Dependabot for Go modules, Actions, Dockerfiles and compose files. Base images are pinned by
  digest. go-redis upgraded to 9.7.3 (GO-2025-3540).
- Rate limiting keys an IPv6 client by its /64 and reads `X-Forwarded-For` from the right, past
  `TRUSTED_PROXIES`; an unparseable `TRUSTED_PROXIES` now stops startup.
- CORS answers only `FRONTEND_URL` in production (loopback only otherwise); the
  `*.skillissues.com` wildcard is gone.
- Every outbound call has a deadline (`http.outbound_timeout_seconds`, default 15), replacing
  `http.DefaultClient` and the crawler's fixed 15 s. `GET /issues` `limit`/`page` and
  `GET /recommendations` `limit` are capped (`api.issues.max_limit`, `api.issues.max_page`,
  `api.recommendations.max_limit`); larger requests are clamped.
- Admin and ingestd errors no longer return internal error text, and saving or unsaving an issue
  no longer discards a database error.
- A webhook whose issue `updated_at` is not newer than the stored one is ignored.
- `go-api` and `ingestd` no longer connect as the database owner (a superuser). They use
  `skillissues_api` and `skillissues_ingest`, which have per-table DML grants only (migration
  00029). **New required secrets: `API_DB_PASSWORD` and `INGEST_DB_PASSWORD`.**
- The server no longer holds any user GitHub token. Sign-in uses a no-scope **Login app**
  (`GITHUB_LOGIN_CLIENT_ID`/`_SECRET`); the token is read once for the numeric GitHub id and
  discarded. Removed: PAT login, GitHub App user login, `POST /github/forks`,
  `GET /github/git-token`, the contribution endpoints and their webhook handling, the issue
  body and freshness endpoints, `internal/crypto` and `TOKEN_ENCRYPTION_KEY`.
- Fixed account takeover by username reuse: accounts are matched on GitHub id only.
- Fixed desktop session theft: the desktop ticket flow is gone; desktop signs in through
  `POST /auth/github/device`.
- OAuth callbacks require a browser-bound state cookie and redirect with fixed error codes.
- CSRF protection on cookie-authenticated requests.
- Added `POST /auth/logout-all`; `POST /auth/purge` needs `{"confirm": true}`; expired sessions
  and OAuth states are pruned (`api.session.prune_interval_seconds`).

### Removed
- Contributions, XP, levels and badges, and `GET /me`'s fields for them (migration 00027).
  All users, sessions and saved issues were deleted by that migration.
- Settings `api.contribution.verify_per_hour`, `api.issues.freshness_batch_cap`,
  `api.issue_body.*` and `auth.pat_login_enabled`.

### Added
- Prometheus metrics that report real values: HTTP traffic and latency by route pattern,
  outbound GitHub calls, ingest job outcomes, queue depth, repository coverage, GitHub budget
  per lane, worker and connection pools. `ingestd` now serves `/metrics` too.
- Monitoring stack in both compose files — Prometheus, Grafana with six provisioned dashboards,
  node-exporter, cAdvisor, Postgres and Redis exporters — reachable only over an SSH tunnel in
  production.
- `metrics_ro`, a read-only, column-restricted database role for dashboards.
- Live settings `ingest.metrics_refresh_seconds`, `ingest.claim_interval_seconds`,
  `ingest.lease_seconds`, `ingest.lease_reclaim_interval_seconds`,
  and `ingest.rate_limit_pause_seconds`, replacing values that were hardcoded.
- Expired ingest leases are reclaimed continuously, not only when ingestd starts
  (`ingest_leases_reclaimed_total`).
- CI runs the database and Redis tests against service containers.
- `docs/api.md` and `docs/settings.md`; the ingestion design doc moved to
  `docs/ingestion-architecture.md` with an implementation-status table.

### Changed
- Every ingest job write is fenced on its lease owner. A worker whose lease was reclaimed gets
  `ErrLeaseLost` (outcome `lease_lost`) instead of overwriting the new owner's cursor.
- The deploy script validates `.env` before pulling and stops, naming the missing key, instead
  of failing mid-deploy.

### Fixed
- With Redis down, every API request took about 20 seconds: each Redis call waited out the
  client's dial timeout and retries before failing open. Each call now has a deadline
  (`api.cache.redis_timeout_ms`), and after a failure Redis is skipped, with one call every
  `api.cache.redis_retry_seconds` checking whether it is back. Skipped calls count in
  `cache_skipped_total`. A request cancelled by its own client no longer counts as a Redis failure.
- Grafana was stuck in an out-of-memory restart loop in production (160m limit, 17 plugins each
  running as a process). It now loads only the Prometheus and Postgres datasources it uses,
  installs both synchronously, and has a 384m limit.
- Deploys never applied Caddyfile or `prometheus.yml` changes: `compose up` leaves an unchanged
  container running with the config it loaded at start. The deploy now reloads Caddy and
  Prometheus. Until this ships, `/api/metrics` and `/api/health/detail` were still public in
  production despite the edge 404 rule.
- Seven of the eight exported metrics were never set and always read 0.
- A failed webhook delivery-record insert returned 200, so GitHub never redelivered and the
  merged PR was never credited; it now returns 500.
- Errors from merged-PR application, GitHub budget reads/writes and job release were
  discarded; they are now logged and counted. A data race on ingestd's `/sync/status` is fixed.
- The rate-limit window slid forward on every request, so a steady client's count never reset;
  it is now a fixed window.
- GitHub App users could not open a pull request once their 8-hour token expired: PR submit
  skipped the token refresh.
- When GitHub refused to open a PR, the contribution was marked `PR_OPEN` with a compare-page
  URL and the issue number as the PR number, which could never be verified. It is now left
  unchanged and the compare page is returned. A failed default-branch lookup no longer guesses
  `main`.
- A failed profile read, prior-merge count, badge count or audit-log insert during verification
  was ignored, so a merge could be marked awarded with the XP lost. Each now aborts the award.
- The API logged "redis connected" even when Redis was unreachable. Redis failures now count in
  `cache_errors_total` and are logged when Redis goes down and when it recovers.
- Errors loading or refreshing a user's GitHub token, and from the issue-body cache, were
  discarded; they are now logged, and a retryable failure returns 502 instead of "connect
  GitHub".

### Removed
- `REDIS_URL` and the Redis dependency from the ingestd service; ingestd never used Redis.
- `GITHUB_WEBHOOK_SECRET`, `TOKEN_ENCRYPTION_KEY`, `GITHUB_APP_CLIENT_SECRET` and
  `GITHUB_APP_CALLBACK_URL` from the ingestd service. Each binary now names the secrets it uses
  (`config.Load(required...)`), so ingestd no longer has to be given ones it never reads.
- Settings `ingest.sync_interval_seconds` and `ingest.ondemand_workers`, which no code read.

### Security
- Grafana encrypts its stored secrets with `GRAFANA_SECRET_KEY` (required in production)
  instead of its public built-in default.
- `/api/metrics` and `/api/health/detail` were publicly reachable through Caddy; they now
  return 404 at the edge.
- Reverted a deploy fallback that reused `POSTGRES_PASSWORD` and `ADMIN_SYNC_TOKEN` as the
  monitoring passwords.
- The session cookie is `SameSite=Lax` in every deployment; it was `None` over HTTPS.
- The contribution audit log's JSON was built with `Sprintf` around an unescaped PR URL.
- ingestd's `POST /sync/issues` and `POST /sync/priority/{id}` were unauthenticated, reachable
  from every container on the network (and the LAN in dev); they now require the admin token.

---

## [1.0.0] - 2026-10-13

### 🚀 Production Deployment & Unified Client Architecture

1.0.0 is the release where SkillIssues actually went live, and where the web and desktop
clients stopped drifting apart. The app now runs on a real VPS behind CI-gated deploys, and
both clients share one typed API layer and one set of UI components instead of reimplementing
the product twice.

### Added

#### Production deployment (first real deploy of this project)
- **CI/CD pipeline**: `.github/workflows/ci.yml` gates every push — backend (`vet`, `gofmt`,
  build, test), web/desktop (typecheck **and** a real production build, not just a type check),
  and a `buildx` build of the `api`, `ingestd`, `migrate` and `web` Docker targets plus
  `docker compose config` on both compose files. `.github/workflows/deploy.yml` builds and pushes
  `ghcr.io/dizziedbliss/skillissues-{api,ingestd,migrate,web}` on every push to `main`, gated on
  CI passing, then deploys to the VPS over SSH. Rollback is a `workflow_dispatch` with an older
  immutable `sha-` tag.
- **One root `Dockerfile`** with named targets (`api`, `ingestd`, `migrate`, `web`,
  `repoimport`) built from the repo root, replacing four scattered Dockerfiles, so the `web`
  target can actually see the shared `packages/*` it imports. New root `.dockerignore` keeps
  the build context to tens of MB instead of shipping the multi-GB dataset.
- **`compose.prod.yaml`**: a from-scratch production compose file — Caddy as the only
  internet-facing service (automatic TLS, same-origin `/api/*` routing so there's no CORS and
  no cross-site cookie problem), Postgres/Redis sized for the 4GB/2vCPU VPS, explicit memory and
  CPU limits and log rotation on every service, `DB_MAX_CONNS` made configurable.
- **Disk guardrails**: `ingest.max_pages_per_repo` caps how deep the crawler paginates into any
  one repository; `ingest.max_db_size_mb` pauses enqueuing/claiming once the database crosses a
  configured size. Both live settings, both surfaced on `GET /health/detail` alongside current
  DB size, so the ceiling is visible before it's hit.
- **Automated backups**: a systemd timer running daily `pg_dump` with 14-day retention on the
  host.
- The production repository catalog was populated for the first time: **287,998 eligible
  repositories** imported from the Kaggle metadata snapshot; `ingestd` has been crawling live
  since.

#### Shared client architecture
- **`packages/api-client`**: one typed client for every API endpoint, used by both clients —
  `Issue`/`Contribution`/`Profile` types, `ApiError` that keeps `status` and `detail` so a 401
  is never confused with a 429 or a network failure, `AbortController`/cancellation support,
  pluggable transport (`fetchTransport` for web, an IPC-routed transport for desktop), and the
  recommendations ETag/stale-cache fallback that used to be duplicated and drifting in both
  clients.
- **`packages/ui`**: shared React components — `Modal` (body scroll lock, Escape-to-close,
  focus trap, backdrop click — none of which either client had before), `ConfirmDialog`,
  `Toast`, `IssueCard`/`IssueList`, `WorkingList`/`SubmissionList`, `DifficultyTabs`,
  `SectionTabs`, `Badge`, `ProgressBar`, `Skeleton`.
- **Desktop renderer rewritten in React + TypeScript**, replacing 1,780 lines of plain-JS
  `innerHTML` rebuilds. The old renderer blanked and refetched up to six endpoints from fifteen
  call sites on nearly every interaction (including a single bookmark click) — the rewrite
  moves to per-resource state with per-resource refetch, request sequencing so a fast
  filter→page→search sequence can't let a stale response win, and real React reconciliation, so
  scroll position now survives a list update.
- **Web's `App.tsx` rebuilt** onto the same shared packages, cut from 1,691 to roughly 1,100
  lines of routing and web-specific glue — its hand-rolled issue card, login form, working/
  submission lists, and detail modal are now the shared `packages/ui` components.
- npm workspaces at the repo root (`web`, `desktop`, `packages/*`) so `@skillissues/*` resolves
  as a real `node_modules` symlink, with one lockfile for the whole tree.

#### Issue lifecycle
- **Closed-issue detection**, on demand only (no background sweep): the existing fork/clone
  freshness check and a new GitHub `issues` webhook handler persist a closed/reopened state
  using GitHub's own `updatedAt`. A new batched `POST /issues/freshness` lets a client check
  only the handful of issues it's actually displaying.
- **Sticky saved issues carry their lifecycle** (`none`/`working`/`submitted`/`merged`) as a
  badge; saved issues never disappear regardless of issue or contribution state.
- **Re-solve**: starting a fresh attempt on an already-submitted or already-merged issue works
  end to end in both clients, backed by the existing idempotent fork and partial-unique-index
  contribution model.
- **Real `submitted_at`**, separate from the fork/start time (`created_at`) both clients were
  previously mislabeling as the submission time.
- A "View issue" action was added to working/submission cards on both clients (desktop's
  working cards were missing it entirely).

### Changed
- OAuth is now the only *supported* login path in both clients' UI; the legacy PAT-login
  endpoint stays registered server-side but is disabled by default behind a live setting.
- `FRONTEND_URL` is parsed once as a proper multi-value list instead of being treated as a
  single string everywhere, fixing cookie `Secure` detection, the OAuth redirect target, and the
  CORS `Access-Control-Allow-Origin` header.
- Issue-body "not ready yet" responses now carry a structured `{"status":"queued","reason":...}`
  instead of an ambiguous error shape, and the web client retries with real exponential backoff.
- A failed `/saved`, `/working`, or `/contributions` fetch now keeps the previously loaded list
  and shows a retry affordance instead of silently rendering empty — this previously made
  real data look like it had disappeared whenever the client hit the unauthenticated rate limit.
- `GET /recommendations` now accepts `difficulty`/`page`/`limit` filters through the shared
  client, matching what the backend already supported.

### Removed
- The co-contribution commit-trailer feature (`GET /github/co-contribution-trailer` and both
  clients' UI for it) — removed per product decision, not a regression.
- The PAT login form from both clients' UI.
- `database/migrations/` and `database/schema.sql` as a second, never-applied, out-of-sync
  schema source — the embedded goose migrations under `backend/internal/migrate/migrations/`
  are the only schema source now.
- The stale `k8s/` manifests, which referenced images nothing built and committed a plaintext
  database password.

### Fixed
- The web production Docker image's `HEALTHCHECK` always failed: it resolved `localhost` to
  `::1` while nginx only listens on IPv4, so the check failed even while the container correctly
  served real traffic.
- CI never installed the shared `packages/markdown` module's own dependencies, so every build
  that exercised it was silently relying on a dev machine's leftover `node_modules`.
- The deploy workflow could run and attempt an SSH deploy even when its own CI job had failed,
  caught only by an unrelated empty-tag guard; the condition now requires either an actual CI
  success or an explicit manual rollback dispatch.
- `cmd/ingestd` logging a prefix and length of the GitHub token at startup — removed outright;
  credentials are never logged, not even partially.
- A swallowed `QueryRow` error in `GET /health/detail` and a discarded `UPDATE` error in
  `submitContribution`, both now surfaced.
- Account deletion (`authPurge`) previously discarded its own delete errors and always reported
  success; it is now a single cascading `DELETE` whose error is returned to the caller.
