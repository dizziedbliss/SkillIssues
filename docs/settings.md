# Live settings

Every operational value lives in the `settings` table and can be changed on a running system —
no restart, no redeploy. The keys, defaults and bounds below come from
`backend/internal/settings/registry.go`; the bounds are enforced by a database trigger, so a
bad value is rejected whether it arrives through the API or `psql`. Every change is recorded in
`settings_audit`.

```bash
# read everything
curl localhost:8000/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"
# change one value
curl -XPUT localhost:8000/admin/config/ingest.routine_workers \
     -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 4}'
```

In production the API is under `https://<domain>/api/…`. Secrets never go in this table — it is
readable through the endpoint above and copied into backups.

## API

| Key | Default | Bounds | What it does |
|---|---:|---|---|
| `api.rate_limit.unauth_per_hour` | 180 | 1–100000 | Requests per window for unauthenticated callers (per IP) |
| `api.rate_limit.auth_per_hour` | 1000 | 1–1000000 | Requests per window for signed-in users (per user) |
| `api.rate_limit.window_seconds` | 3600 | 60–86400 | Rate-limit window length |
| `api.issues.default_page_size` | 20 | 1–100 | Issues per page when browsing without a search term |
| `api.issues.search_page_size` | 50 | 1–200 | Issues returned for a search query |
| `api.search.per_category_cap` | 25 | 1–100 | Maximum search results per match category |
| `api.notifications.page_size` | 50 | 1–200 | Notifications returned by `GET /notifications` |
| `api.session.ttl_hours` | 168 | 1–8760 | Session lifetime; applies to newly issued sessions |
| `api.session.prune_interval_seconds` | 3600 | 60–86400 | How often expired sessions and abandoned OAuth handshakes are deleted |
| `api.cache.hot_issues_ttl_seconds` | 120 | 0–3600 | Cache for the default unauthenticated catalog page (0 = off) |
| `api.cache.search_ttl_seconds` | 60 | 0–3600 | Cache for unauthenticated search results (0 = off) |
| `api.cache.recommendations_ttl_seconds` | 300 | 0–3600 | Cache for per-user recommendations (0 = off) |
| `api.cache.redis_timeout_ms` | 250 | 10–5000 | Deadline for one Redis call, connecting included; a call that misses it fails open |
| `api.cache.redis_retry_seconds` | 5 | 1–300 | After a failed Redis call, how long Redis is skipped before one call probes for recovery |
| `api.http.max_body_bytes` | 1048576 | 1024–104857600 | Largest request body on any API route except the webhook; larger gets 413 |
| `api.webhook.max_body_bytes` | 26214400 | 1024–104857600 | Largest GitHub webhook body accepted; larger gets 413 |
| `api.contribution.verify_per_hour` | 5 | 1–1000 | PR verification attempts per user per contribution per hour |
| `api.issue_body.cache_max_rows` | 50000 | 100–10000000 | Rows kept in `issue_body_cache`; least recently read pruned beyond it |
| `api.issue_body.prune_interval_seconds` | 600 | 60–86400 | How often the body cache is pruned |
| `api.issue_body.max_concurrent_fetches` | 8 | 1–64 | Concurrent GitHub fetches for issue bodies, across all users |
| `api.issue_body.installation_floor` | 100 | 0–5000 | Server token stops serving body reads below this many remaining points |
| `auth.pat_login_enabled` | false | bool | Whether `POST /auth/github/token` (PAT login) is reachable |

## Ingestion

| Key | Default | Bounds | What it does |
|---|---:|---|---|
| `ingest.eligibility.min_stars` | 25 | 0–100000 | Minimum stars to be crawled (abuse control, not ranking) |
| `ingest.eligibility.pushed_within_days` | 730 | 1–3650 | Repository must have been pushed within this many days |
| `ingest.eligibility.recheck_seconds` | 3600 | 60–86400 | How often stored repositories are re-tested against the thresholds |
| `ingest.routine_workers` | 2 | 1–64 | Concurrent Initial/Freshness workers |
| `ingest.priority_workers` | 1 | 1–64 | Concurrent Deep Diver workers |
| `ingest.graphql_page_size` | 50 | 1–100 | Issues per GraphQL page |
| `ingest.rate_limit_floor` | 100 | 0–5000 | Crawler stops below this many remaining GitHub points |
| `ingest.max_attempts` | 3 | 1–10 | Attempts before a job is left `failed` |
| `ingest.backoff_base_seconds` | 1 | 1–60 | Base of the exponential retry backoff |
| `ingest.freshness_interval_seconds` | 86400 | 300–2592000 | Age of a repository's last sync before a freshness pass (changes since its last completed pass) is queued |
| `ingest.enqueue_batch_size` | 500 | 1–50000 | Jobs enqueued per coordinator cycle |
| `ingest.max_pages_per_repo` | 100 | 0–100000 | Deep Diver page cap per repository (0 = no cap) |
| `ingest.max_db_size_mb` | 30000 | 0–1000000 | Pause new ingestion work above this database size (0 = no cap) |
| `ingest.closed_page_size` | 40 | 1–100 | Issues closed since a repository's last completed pass, fetched with each page of a freshness pass |
| `ingest.tracked_issue_check_seconds` | 3600 | 60–86400 | How stale a saved or in-progress open issue may get before its state is re-checked on GitHub |
| `ingest.tracked_issue_batch_size` | 50 | 1–100 | Tracked issues checked per GitHub request |
| `ingest.closed_issue_retention_days` | 7 | 0–3650 | Days after closing before an issue nobody saved is deleted (0 = keep all) |
| `ingest.closed_issue_prune_interval_seconds` | 3600 | 60–86400 | How often closed issues past their retention, and the issues of excluded or archived repositories, are deleted |
| `ingest.closed_issue_prune_batch_size` | 5000 | 100–100000 | Issues deleted per statement by either prune |
| `ingest.metrics_refresh_seconds` | 30 | 5–3600 | How often ingestd refreshes its Postgres-backed `/metrics` gauges |
| `ingest.claim_interval_seconds` | 5 | 1–300 | Coordinator cycle: enqueue, reclaim, claim, submit |
| `ingest.lease_seconds` | 300 | 60–3600 | How long a claimed job may wait in a pool queue and run before it can be reclaimed |
| `ingest.lease_reclaim_interval_seconds` | 60 | 5–3600 | How often expired leases are returned to `pending` |
| `ingest.rate_limit_pause_seconds` | 60 | 5–3600 | Crawler pause after a GitHub rate limit; retry delay for a job released for budget |

Raising the eligibility thresholds takes effect on the next recheck. **Lowering** them only
affects repositories already stored — anything that failed the old threshold was never imported,
so re-run `cmd/repoimport` (see `README.md`).

If `ingest_job_outcomes_total{outcome="lease_lost"}` climbs, jobs are outliving
`ingest.lease_seconds` (usually a long pool queue during a GitHub slowdown) — raise it. A lost
lease is safe: the stale worker's write is rejected, and the job is retried by its new owner.

## Adding a setting

Add the key to `settings.Registry` **and** seed it in a new migration (see
`00016_ingest_metrics_refresh_setting.sql`). A key in one but not the other falls back to its
default, and a key missing from the table is logged at every load. Read values through `live.Current()` and hold the
snapshot for the duration of an operation.
