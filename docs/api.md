# HTTP API

Routes registered by `backend/cmd/api/main.go`. Locally the API is `http://localhost:8000`; in
production it is `https://<domain>/api` (Caddy strips the `/api` prefix — go-api's own routes
have none). Both clients call it through `packages/api-client`, which is the typed reference for
request and response shapes.

**Auth column:** *session* = a valid session cookie is required (401 otherwise); *optional* =
works signed out, personalised when signed in; *none* = public; *admin* =
`Authorization: Bearer $ADMIN_SYNC_TOKEN`; *HMAC* = GitHub webhook signature.

**Rate limiting** applies to every public route except `/health`, `/health/detail` and
`/metrics`: a Redis counter per user (signed in) or per IP (signed out), limits from
[`api.rate_limit.*`](settings.md). Responses carry `X-RateLimit-Limit` /
`X-RateLimit-Remaining`; a 429 carries `Retry-After`. If Redis is down the limiter fails open.
`/admin/*` and `/webhooks/*` are outside the public chain and are not rate limited.

## Health and metrics

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | none | Liveness |
| GET | `/health/detail` | none | Row counts, Redis status, DB size vs `ingest.max_db_size_mb`. **404 through Caddy in prod** |
| GET | `/metrics` | none | Prometheus exposition. **404 through Caddy in prod** |

## Auth and session

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/auth/session` | none | Current session's user, or signed out |
| GET | `/auth/github` | none | Start the OAuth App login |
| GET | `/auth/github/callback` | none | OAuth App callback |
| GET | `/auth/github/poll` | none | Desktop polls for a completed browser login |
| GET | `/auth/github-app` | none | Start the GitHub App login |
| GET | `/auth/github-app/callback` | none | GitHub App callback |
| POST | `/auth/github/token` | none | PAT login — **disabled unless `auth.pat_login_enabled`** |
| POST | `/auth/logout` | none | Clear the session |
| POST | `/auth/purge` | session (checked in the handler) | Delete the account and everything that cascades from it |

## Catalog

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/issues` | optional | Browse or search (`?search=`, filters); search is balanced across Repository / Title / Language / Author |
| GET | `/issues/{id}` | none | One issue |
| GET | `/issues/{id}/body` | optional | Issue body, fetched on demand from GitHub and cached; may answer `{"status":"queued"}` |
| POST | `/issues/{id}/freshness` | session | Re-check one issue's open/closed state on GitHub |
| POST | `/issues/freshness` | session | Same, for up to `api.issues.freshness_batch_cap` ids |
| GET | `/recommendations` | session | Issues ranked for the user's profile (`difficulty`, `page`, `limit`) |
| GET | `/preferences/options` | none | Selectable languages/interests for the profile form |

## User

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/me` | session | Profile, XP, level, badges |
| PUT | `/profile` | session | Update profile and preferences |
| GET | `/saved` | session | Saved issues, with each one's contribution lifecycle |
| POST | `/saved` | session | Save an issue |
| DELETE | `/saved/{id}` | session | Unsave |

## Contributions

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/contributions` | session | Start a contribution (state `STARTED`) |
| GET | `/contributions` | session | The user's contributions |
| GET | `/working` | session | Contributions in progress |
| GET | `/contributions/{id}` | session | One contribution |
| PUT | `/contributions/{id}` | session | Update it (e.g. PR URL) |
| POST | `/contributions/{id}/submit` | session | Mark submitted (`submitted_at`) |
| POST | `/contributions/{id}/verify` | session | Ask GitHub whether the PR merged and the author matches; awards XP once. Limited by `api.contribution.verify_per_hour` |
| POST | `/github/forks` | session | Fork the issue's repository **with the user's own GitHub token** |
| GET | `/github/git-token` | session | The user's own token for `git push` from the desktop app — never the server's |

States: `STARTED` → `PR_OPEN` → `PR_MERGED` → `VERIFIED`, or `PR_CLOSED`.

## Webhooks and admin

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/webhooks/github` | HMAC | `pull_request` merged → verify and award XP; `issues` closed/reopened → update state. Deduplicated by delivery id |
| POST | `/admin/sync/repository/{id}` | admin | Ask ingestd to crawl one repository now |
| GET | `/admin/config` | admin | All live settings |
| PUT | `/admin/config/{key}` | admin | Change a live setting — see [settings.md](settings.md) |

## ingestd (internal only, `:8002`)

Not reachable from outside the Docker network and **not authenticated** — never publish it.

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness |
| GET | `/metrics` | Prometheus exposition (ingest, GitHub budget, worker pools) |
| GET | `/sync/status` | Worker pool stats, DB size, paused flag |
| POST | `/sync/issues` | Enqueue never-ingested eligible repositories now |
| POST | `/sync/priority/{id}` | Make one repository's job due immediately |
