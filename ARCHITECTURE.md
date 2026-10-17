# Architecture

How SkillIssues is put together, as of the code in this repository. For the rules contributors
must follow, see [CLAUDE.md](CLAUDE.md).

## 1. Components

```mermaid
flowchart LR
    subgraph Clients
        Web["web — React 19 + Vite"]
        Desktop["desktop — Electron 44<br/>React renderer"]
    end

    subgraph Edge["production only"]
        Caddy["Caddy — TLS, /api/* → go-api, /* → web"]
    end

    API["go-api :8000<br/>stdlib ServeMux"]
    Ingestd["ingestd :8002<br/>coordinator + worker pools"]
    PG[("PostgreSQL 16<br/>data, sessions, job queue, settings")]
    Redis[("Redis 7<br/>rate limits, response cache")]
    GH["GitHub GraphQL + REST"]
    Import["repoimport<br/>(run by hand)"]
    Kaggle[/"Kaggle repository<br/>metadata dataset"/]

    Web --> Caddy --> API
    Desktop -->|via preload IPC| API
    Desktop -->|fork, push, PR, issue bodies: the user's own token| GH
    API --> PG
    API --> Redis
    API -->|admin: crawl one repo| Ingestd
    Ingestd --> PG
    Ingestd -->|crawl open issues| GH
    Kaggle --> Import --> PG
```

| Binary (`backend/cmd/`) | Role |
|---|---|
| `api` | The HTTP API ([route list](docs/api.md)). Go standard-library `http.ServeMux` with method patterns; no third-party router. |
| `ingestd` | GitHub crawler. One coordinator loop plus two worker pools (Initial, Deep Diver). Uses Postgres only — not Redis. |
| `migrate` | Applies the embedded goose migrations and exits. Compose runs it before anything else starts. |
| `repoimport` | Streams the ~4.2M-record Kaggle dataset, stores the ~288k eligible repositories. Resumable, idempotent. |
| `healthcheck` | Tiny binary used as the containers' Docker `HEALTHCHECK`. |

The schema lives only in `backend/internal/migrate/migrations/`, embedded into the binaries.

## 2. Ingestion

```text
Kaggle dataset ──repoimport──▶ repositories (eligible only)
                                   │ coordinator cycle (ingest.claim_interval_seconds)
                                   ▼
                     Initial: first page of open issues
                                   │ hasNextPage?
                                   ▼
                     Deep Diver: next page from stored cursor, until done
                                   │
                     Freshness: re-fetch page one once the last sync is stale
```

- **Every eligible repository is crawled.** There is no ranking or priority anywhere in the
  crawl path: jobs are FIFO on `next_retry_at`. Eligibility (`ingest.eligibility.*`) is abuse
  control — anyone can open issues on a throwaway repo — not a quality judgement.
- **Postgres is the queue.** `ingest_jobs` rows are claimed with `FOR UPDATE SKIP LOCKED` and a
  lease. A page's issues, its cursor and `last_synced_at` commit in one transaction, so a crash
  can only cause a page to be fetched twice, never skipped. Expired leases are reclaimed every
  `ingest.lease_reclaim_interval_seconds`, and each job write is fenced on the lease owner, so a stalled worker that wakes up
  after its job was reclaimed has its write rejected rather than overwriting the cursor.
- **One shared GitHub budget.** Both lanes (`crawler`, `interactive`) record GitHub's rate-limit
  telemetry in `github_budget`; the crawler stops below `ingest.rate_limit_floor`.
- **Failures are isolated.** `NOT_FOUND` marks the repository unavailable;
  `RESOURCE_LIMITS_EXCEEDED` halves the page size and retries down to 1; rate limits pause the
  lane for `ingest.rate_limit_pause_seconds`; other errors back off exponentially and land in `failed` after
  `ingest.max_attempts`.
- **Disk guardrails.** `ingest.max_pages_per_repo` caps how deep one repository goes;
  `ingest.max_db_size_mb` pauses new work when the database grows past it.

Design rationale and what is not built yet (the Discovery Sweeper): [docs/ingestion-architecture.md](docs/ingestion-architecture.md).

## 3. Data model (main tables)

| Table | Holds |
|---|---|
| `repositories` | Imported metadata: stars, language, topics, pushed date, `is_eligible`, `last_synced_at` |
| `issues` | One row per `(repository_id, number)`; raw GitHub `labels` kept as-is; `updated_at` is GitHub's |
| `users`, `auth_sessions` | Accounts keyed by GitHub id (no token columns); sessions stored as SHA-256 hashes of the session token |
| `developer_profiles`, `developer_preferences` | Bio, experience and target level; languages, interests |
| `saved_issues` | Bookmarks |
| `ingest_jobs`, `github_budget`, `import_skipped` | Crawl queue, shared rate-limit budget, import skip ledger |
| `settings`, `settings_audit` | Live configuration and every change to it |
| `webhook_events` | GitHub delivery ids, for deduplication |

## 4. How the product logic works

**Difficulty** (`internal/sync/classify.go`) comes from labels only. Beginner-style labels
(`good first issue`, `difficulty/easy`, `size/xs`, `effort/low`, …) → `BEGINNER`; medium →
`INTERMEDIATE`; hard/expert/large → `ADVANCED`. Anything else is `UNCLASSIFIED` with a NULL
score. There is no text or comment-count heuristic.

**Search** (`internal/issue/search.go`). Each match is put in one category — Repository, Title
(Postgres full-text), Language (language, technologies, tags, skills) or Author — and a
`ROW_NUMBER() OVER (PARTITION BY category)` CTE keeps at most `api.search.per_category_cap`
results per category, so one category cannot flood the page.

**Recommendations** (`internal/recommendation/score.go`):
`15 × language match + 8 × technology match + 2 × shared skills + shared interests +
max(0, 5 − |difficulty − target|) + min(3, stars / 25000 + 1)`.

## 5. Auth and credentials

- **The server holds no user GitHub token.** Sign-in uses the **Login app**, a no-scope OAuth
  App: the server reads the numeric id, login and avatar and discards the token. The browser
  flow is bound to the browser that began it by a `__Host-oauth_state` cookie; the desktop app
  runs a device flow on the same app and posts its token once to `POST /auth/github/device`,
  which GitHub confirms was issued to the Login app.
- **Accounts** are identified by GitHub id only. A username is display data and is never used to
  find an account; if another row holds it, that row's username is cleared.
- **Sessions** live in Postgres, so they survive a Redis outage. The cookie is `HttpOnly`;
  always `SameSite=Lax`, and `Secure` when `FRONTEND_URL` is `https://`. A session ends on
  logout, expiry, "log out everywhere" or purge — revoking the Login app on GitHub does not end
  one, because there is no token to invalidate.
- **CSRF**: cross-site mutations are refused (`http.CrossOriginProtection`), and a request that
  carries the session cookie must send `Content-Type: application/json`. Bearer requests are
  exempt.
- **Whose token does what:** crawling uses the server credential (GitHub App installation token,
  or `GITHUB_TOKEN`). Forks, pushes, pull requests and issue-body reads are the desktop client's,
  using a token that never leaves the user's machine. The server cannot act on a user's GitHub
  account.
- **Webhooks** are HMAC-SHA256 verified. **Admin** routes take a bearer token compared in
  constant time.

**Database access.** Migrations run as the database owner. `go-api` and `ingestd` connect as
`skillissues_api` and `skillissues_ingest`, which hold per-table DML grants and nothing else, so
a compromised service cannot drop or alter the schema or run commands on the host.

## 6. Rate limiting and caching

A Redis counter per signed-in user or per client IP (`X-Forwarded-For` honoured only from
`TRUSTED_PROXIES`). Defaults are 180 requests per window signed out and 1,000 signed in, window
3600s — all live settings. The window is fixed: it starts at a client's first request and the
count resets one window later, however busy the client is. `/health`, `/health/detail` and `/metrics` are exempt;
login is not. Redis also caches the default catalog page, unauthenticated searches and
recommendations. If Redis is down, limiting fails open and caching is skipped — nothing else
depends on it. Failing open is fast: each call has a deadline (`api.cache.redis_timeout_ms`), and
after a failure the API stops calling Redis, letting one call through every
`api.cache.redis_retry_seconds` to check whether it is back. Each failed Redis call counts in
`cache_errors_total{op}`, each skipped one in `cache_skipped_total{op}`, and the API logs once when
Redis becomes unreachable and once when it recovers.

## 7. Configuration

Three layers: bootstrap env (`DATABASE_URL`, `REDIS_URL`, `PORT`, `FRONTEND_URL`), secrets in
env, and **everything operational in the `settings` table**, changeable live and validated by a
database trigger. Full list: [docs/settings.md](docs/settings.md).

## 8. Observability

Both Go services serve Prometheus metrics at `/metrics`: HTTP traffic and latency by route
pattern, outbound GitHub calls, ingest job outcomes, queue depth, repository coverage, the GitHub
budget per lane, worker pools, connection pools, and Go runtime stats. The compose stacks run
Prometheus, Grafana (six provisioned dashboards), node-exporter, cAdvisor and Postgres/Redis
exporters. Product panels query Postgres as `metrics_ro`, a read-only role that cannot see session tokens
or OAuth state. See DEPLOYMENT.md → *Monitoring*.

## 9. Clients

Both clients share three npm workspace packages: `@skillissues/api-client` (typed client,
`ApiError` with status and detail, pluggable fetch or IPC transport), `@skillissues/ui` (Modal
with focus trap and scroll lock, issue lists, …) and `@skillissues/markdown`
(markdown-it + DOMPurify — issue bodies are attacker-controlled).

**Web** (`web/`): React 19 single-page app. Signed-out visitors get `LandingPage`; signed-in
users get the catalog.

**Desktop** (`desktop/`, electron-vite): the renderer runs with `contextIsolation`, `sandbox`
and no Node integration, behind a Content Security Policy, and reaches the API only through the
preload bridge. The main process holds a single-instance lock, opens only allow-listed GitHub
URLs, and registers the `skillissues://` protocol. Workspace flow: fork with the user's token,
clone into the chosen workspace folder (default `~/Documents/SkillIssues`; git commands refuse
to run outside it), point `origin` at the fork and `upstream` at the original, create a branch,
push, open VS Code. Git runs as async child processes and every command is logged to
`.skillissues/commands.log` in the clone. Cloned code is never executed.
