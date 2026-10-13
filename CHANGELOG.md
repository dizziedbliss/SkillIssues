# Changelog

All notable changes to **SkillIssues** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
<!-- New features added in unreleased development -->

### Changed
<!-- Changes in existing functionality -->

### Deprecated
<!-- Features soon to be removed -->

### Removed
<!-- Features removed in this release -->

### Fixed
<!-- Bug fixes -->

### Security
<!-- Vulnerabilities and security remediations -->

---

## [3.0.0] - 2026-10-13

### 🚀 Production Deployment & Unified Client Architecture

3.0.0 is the release where SkillIssues actually went live, and where the web and desktop
clients stopped drifting apart. The app now runs on a real VPS behind CI-gated deploys, and
both clients share one typed API layer and one set of UI components instead of reimplementing
the product twice.

### Added

#### Production deployment (first real deploy of this project)
- **CI/CD pipeline**: `.github/workflows/ci.yml` gates every push — backend (`vet`, `gofmt`,
  build, test), web/desktop (typecheck **and** a real production build, not just a type check),
  and a `buildx` build of all five Docker targets plus `docker compose config` on both compose
  files. `.github/workflows/deploy.yml` builds and pushes
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
  success; it now wraps the deletes in a transaction and returns the real error on failure.

---

## [2.0.0] - 2026-09-18

### 🚀 Production Release: Full Go Architecture & Lavender Developer Experience

The 2.0.0 release represents a comprehensive rewrite and unification of SkillIssues into a high-performance open-source issue discovery platform and desktop contribution engine.

### Added

#### Pure Go High-Performance Backend
- **Go API Daemon (`backend/cmd/api`)**: Replaced the legacy Python FastAPI service with an authoritative Go REST API serving all 28 routes with sub-millisecond response latency.
- **Sync Coordinator (`backend/cmd/syncd`)**: Dedicated GitHub GraphQL ingestion engine capable of indexing hundreds of thousands of repository issues with automatic pagination and label extraction.
- **Background Worker (`backend/cmd/worker`)**: Independent daemon performing periodic priority catalog refreshes, sync coordination, and health monitoring.
- **Redis Cache & Session Store**: Distributed session management using SHA-256 token hashing, token-bucket sliding-window rate limiting (180 req/min unauthenticated, 1,000 req/min authenticated), and hot-path issue caching.
- **Dynamic Catalog Telemetry (`GET /health/detail`)**: Real-time database telemetry reporting live issue counts, indexed repositories, active contributions, and cache status.

#### Desktop Companion Application
- **Native Electron 32 App (`desktop/`)**: Cross-platform desktop application for Windows, macOS, and Linux.
- **Automated Git Workspace Flow**: 1-click repository fork via GitHub API, local clone into isolated workspace, automatic remote configuration (`origin` & `upstream`), branch generation, and instant VS Code launching.
- **Deep Linking Protocol**: `skillissues://` custom protocol handler for initiating workspace actions directly from the browser.
- **Direct Installer Artifacts**: Packaged `.exe` installer (Windows), `.dmg` (macOS), and `.AppImage` (Linux).

#### Web Experience & Design System
- **Lavender & Plum Theme (`web/src/styles.css`)**: Complete design system with Noto Sans and JetBrains Mono typography, lavender surfaces (`#f8f5fe`), deep plum foregrounds (`#30183b`), vibrant purple accents (`#8241bf`), and subtle paper-grid hero styling.
- **Dynamic Landing Page (`web/src/LandingPage.tsx`)**: Replicating modern developer tooling landing aesthetics with live database issue counters, dynamic difficulty filtering, curated real repository challenges, and official logo branding.
- **Multi-Category Partitioned Search**: Unified instant search separating results across Repositories, Programming Languages, Curated Issues, and Repository Authors/Organizations.
- **Verified Progression Engine**: Contributor level calculation, XP reward tiers (75 XP Beginner, 140 XP Intermediate, 250 XP Advanced), and achievement badges awarded exclusively after merged pull requests.

### Security
- **Strict Parameter Sanitization**: Hardened Git workspace commands preventing shell injection and argument injection across `git clone`, `checkout`, and `remote`.
- **Timing-Safe Authentication**: Constant-time token comparisons (`subtle.ConstantTimeCompare`) on all session and token checks.
- **GitHub Webhook Verification**: Cryptographic HMAC-SHA256 payload verification for GitHub webhook events.
- **Scoped Token Storage**: Encryption and token masking ensuring raw GitHub Personal Access Tokens are never persisted unhashed.

### Changed
- Refactored repository structure to clean standard layout: `backend/` (Go services), `web/` (React dashboard), `desktop/` (Electron companion), and `deploy/` (Docker and production configs).
- Upgraded Docker Compose configuration with independent health checks and resilient service dependencies.
