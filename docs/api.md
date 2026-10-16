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
`/admin/*` and `/webhooks/*` are outside the public chain and are not rate limited. Request bodies are capped (`api.http.max_body_bytes`; `api.webhook.max_body_bytes` for the webhook); over the cap is `413`. In production Caddy also answers `404` for `/admin/*`.

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
| GET | `/auth/github` | none | Start Login app sign-in (sets the `__Host-oauth_state` cookie) |
| GET | `/auth/github/callback` | none | Login app callback; needs the state cookie; errors redirect with a fixed `?error=` code |
| POST | `/auth/github/device` | none | Desktop sign-in: body `{"access_token"}` from the Login app's device flow; returns a bearer session. The token is checked with GitHub and discarded |
| POST | `/auth/logout` | none | Clear the session |
| POST | `/auth/logout-all` | session | End every session of the user |
| POST | `/auth/purge` | session (checked in the handler) | Body `{"confirm": true}`. Delete the account and everything that cascades from it; the response carries GitHub's authorized-apps URL |

## Catalog

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/issues` | optional | Browse or search (`?search=`, filters); search is balanced across Repository / Title / Language / Author |
| GET | `/issues/{id}` | none | One issue |
| GET | `/recommendations` | session | Issues ranked for the user's profile (`difficulty`, `page`, `limit`) |
| GET | `/preferences/options` | none | Selectable languages/interests for the profile form |

## User

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/me` | session | Profile and preferences |
| PUT | `/profile` | session | Update profile and preferences |
| GET | `/saved` | session | Saved issues (a closed issue stays listed) |
| POST | `/saved` | session | Save an issue |
| DELETE | `/saved/{id}` | session | Unsave |

## Webhooks and admin

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/webhooks/github` | HMAC | `issues` closed/reopened → update state. Deduplicated by delivery id |
| POST | `/admin/sync/repository/{id}` | admin | Ask ingestd to crawl one repository now |
| GET | `/admin/config` | admin | All live settings |
| PUT | `/admin/config/{key}` | admin | Change a live setting — see [settings.md](settings.md) |

## ingestd (internal only, `:8002`)

No public port in production. The `GET` routes are unauthenticated (read-only telemetry); the
`POST` routes take `Authorization: Bearer $ADMIN_SYNC_TOKEN`, which go-api's
`POST /admin/sync/repository/{id}` forwards. The dev stack publishes `:8002` on all interfaces.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | — | Liveness |
| GET | `/metrics` | — | Prometheus exposition (ingest, GitHub budget, worker pools) |
| GET | `/sync/status` | — | Worker pool stats, DB size, paused flag |
| POST | `/sync/issues` | admin | Enqueue never-ingested eligible repositories now |
| POST | `/sync/priority/{id}` | admin | Make one repository's job due immediately |

## Browser requests

A request that carries the session cookie and is not `GET`/`HEAD`/`OPTIONS` must send
`Content-Type: application/json` (415 otherwise), including body-less ones such as
`POST /auth/logout` and `DELETE /saved/{id}`. Cross-site mutations are refused with 403 unless
the origin is in `FRONTEND_URL`. Requests authenticated with `Authorization: Bearer` are exempt.
