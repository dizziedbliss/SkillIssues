# HTTP API

Routes registered by `backend/cmd/api/main.go`. Locally the API is `http://localhost:8000`; in
production it is `https://skillissues-api.dizziedbliss.lol`, with routes at the root. Both clients
call it through `packages/client`, which is the typed reference for request and response shapes.

**Auth column:** *session* = a valid session cookie is required (401 otherwise); *optional* =
works signed out, personalised when signed in; *none* = public; *admin* =
`Authorization: Bearer $ADMIN_SYNC_TOKEN`; *HMAC* = GitHub webhook signature.

**Rate limiting** applies to every public route except `/health`, `/health/detail` and
`/metrics`: a Redis counter per user (signed in) or per IP (signed out), limits from
[`api.rate_limit.*`](settings.md). Responses carry `X-RateLimit-Limit` /
`X-RateLimit-Remaining`; a 429 carries `Retry-After`. Signed-out callers are keyed by client address (an IPv6 client by its /64; `X-Forwarded-For` is read from the right, past `TRUSTED_PROXIES`). If Redis is down the limiter fails open.
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
| GET | `/auth/github/device` | none | `{enabled, client_id}`: what the desktop app needs to start the Login app's device flow itself |
| POST | `/auth/github/device` | none | Desktop sign-in: body `{"access_token"}` from the Login app's device flow; returns a bearer session. The token is checked with GitHub and discarded |
| POST | `/auth/logout` | none | Clear the session |
| POST | `/auth/logout-all` | session | End every session of the user |
| POST | `/auth/purge` | session (checked in the handler) | Body `{"confirm": true}`. Delete the account and everything that cascades from it; the response carries GitHub's authorized-apps URL |

## Catalog

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/issues` | optional | Browse or search (`?search=`, filters). Search is balanced across Repository (name contains the term), Title (full-text on title and labels), Language (exactly the term) and Author (login contains it); terms under 3 characters match Title and Language only |
| GET | `/issues/facets` | none | The filter dropdowns' values, most common first: `{"labels", "topics", "tech", "languages": [{"value", "issues"}], "refreshed_at"}`. Refreshed daily by ingestd |
| GET | `/issues/{id}` | none | One issue |
| GET | `/issues/{id}/body` | optional | The body fallback, for a client GitHub refused for its own rate limit: `{"body"}` from the cache, or fetched by ingestd. `202 {"status": "queued", "reason": "budget" \| "no-credential" \| "disabled"}` when it cannot be fetched now; `429` past `api.bodies.fallback_misses_per_hour` misses (cache hits are free); `503` when the cap cannot be counted |
| GET | `/recommendations` | session | Issues ranked for the user's profile (`difficulty`, `page`, `limit`) |
| GET | `/preferences/options` | none | Selectable languages/interests for the profile form |

`limit` and `page` are clamped, not refused: `/issues` to `api.issues.max_limit` / `api.issues.max_page`, `/recommendations` to `api.recommendations.max_limit` (`docs/settings.md`).

**Filtering `/issues`.** Every list takes comma-separated values, any of which matches; filters
combine with AND. More than `api.issues.filter_max_values` values in one list, or anything
malformed, is a 400.

| Parameter | Matches |
|---|---|
| `difficulty` | `BEGINNER`, `INTERMEDIATE`, `ADVANCED`, `UNCLASSIFIED` (any case) |
| `language` | the repository's main language |
| `tech` | any language GitHub reports in the repository |
| `topics` | the repository's GitHub topics |
| `labels` | the issue's labels, case-insensitively |
| `comments_min`, `comments_max` | the comment count, inclusive; `comments_max=0` is "no comments yet" |
| `sort` | browsing only. `foryou` is the default for cursor paging: the shuffled walk from a point seeded per caller per day, each chunk (`api.foryou.pool_repos`) ordered by a weighted random draw on Fit × Doable × Need (`internal/issue/foryou.go`), issues carrying `for_you` reasons. `shuffle` is each repository's best issue, repositories in a random order redrawn every `ingest.catalog.shuffle_interval_seconds`, the same for everyone in between. `easiest` is difficulty then recently updated, the default for `?page`. `newest` is most recently opened. For you, shuffle and newest page only by cursor: `?page=2&sort=newest` is a 400. A search orders by match category unless `sort` is `easiest` or `newest` |

Values for `labels`, `topics`, `tech` and `language` come from `GET /issues/facets`.

**Paging `/issues`.** Two modes, chosen by whether `?cursor=` is present:

- Without it, the response is a JSON array of issues and `?page` pages through it. Deep pages
  cost more the deeper they go.
- With it, the response is `{"issues": [...], "next_cursor": "…" | null}`. An empty `?cursor=`
  is the first page; pass each page's `next_cursor` back for the next, until it is `null`. Every
  page costs the same however deep. The token is opaque. Filters (`difficulty`, `language`,
  `limit`) must stay the same across one walk.

Both modes list the catalog in the same order: difficulty score ascending (unclassified last), then
most recently updated, then id. A signed-in caller's preferred languages come first. Cursor paging
is for browsing only: `?cursor=` with `?search=` is a 400, and so is a cursor the server did not
issue.

**Caching.** A signed-out `/issues` response carries `Cache-Control: public, max-age=…,
stale-while-revalidate=…` (`api.cache.public_*`) and an `ETag`; `If-None-Match` gets a 304. A
signed-in response is `private, no-store`, since its order depends on the caller.

**Timeouts.** A catalog query that runs past `api.issues.query_timeout_ms` is cancelled and
answered with 503; retry it. `/issues/{id}` is 404 only for an issue that does not exist, and 5xx
for a failure.

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

## flowd (`:8003`; `/stream` public on `STREAM_DOMAIN`)

The live visualizer's stream. Caddy serves only `GET /stream` on `STREAM_DOMAIN`; `/health`
and `/metrics` are internal. A browser request is answered only from an origin in
`FLOW_ALLOWED_ORIGINS` (403 otherwise); a request with no `Origin` (curl) always is. Past
`flow.max_viewers`, a new viewer gets 503 with `Retry-After`.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/stream` | — | Server-Sent Events: one `data:` line per event, `: heartbeat` comments every `flow.heartbeat_seconds` |
| GET | `/stream?format=ndjson` | — | The same, one JSON object per line (`curl -N`); the heartbeat is a `kind: "heartbeat"` line |
| GET | `/health` | — | 200 while Redis answers, 503 while it does not |
| GET | `/metrics` | — | `flow_viewers`, `flow_events_relayed_total`, `flow_events_dropped_total{reason}`, `flow_ambient_errors_total` |

Every line is one object:

```json
{"t":"2026-10-18T12:00:00.123Z","from":"go-api","to":"postgres","kind":"sql","label":"SELECT issues","ms":1.42,"n":1}
```

| Field | Meaning |
|---|---|
| `t` | When the hop happened (UTC) |
| `from`, `to` | Nodes: `users`, `github`, `caddy`, `frontend`, `go-api`, `ingestd`, `flowd`, `postgres`, `redis`, `prometheus`, `grafana`, `postgres-exporter`, `redis-exporter`, `node-exporter`, `cadvisor` |
| `kind` | `http`, `sql`, `redis`, `github`, `crawl`, `discovery` (one hop each); `density`, `ambient`, `viewers`, `heartbeat` (written by flowd) |
| `label` | Route pattern (`GET /issues/{id}`), SQL verb and table, GitHub operation, or what a crawled page brought in (`owner/name +50`) |
| `payload` | Status and size, `error`, `miss`, or a crawled page's first issue title. Omitted when empty |
| `ms` | Duration of the hop; for `density`, the window the count covers |
| `n` | 1 for a hop; events in the window for `density`; bytes/s for `ambient`; open streams for `viewers` |
| `redacted` | Present and `true` when the hop belonged to a signed-in user: `label` and `payload` are random symbols, unrelated to the original text or its length |

No line carries an IP address, token, request id, user id, URL path or SQL value. `label` and
`payload` are public GitHub data or text the code itself wrote, and an issue title is
attacker-controlled: render it as text.

## Browser requests

A request that carries the session cookie and is not `GET`/`HEAD`/`OPTIONS` must send
`Content-Type: application/json` (415 otherwise), including body-less ones such as
`POST /auth/logout` and `DELETE /saved/{id}`. Cross-site mutations are refused with 403 unless
the origin is in `FRONTEND_URL`. Requests authenticated with `Authorization: Bearer` are exempt.
