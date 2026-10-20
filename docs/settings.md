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
| `api.search.sample_size` | 2000 | 100–100000 | Title matches a search reads before ordering them by stars, and the deepest one category of a search pages; bounds the work for a common word |
| `api.search.per_repo_cap` | 3 | 1–100 | Most issues one repository contributes to a search's Repository or Language category |
| `api.notifications.page_size` | 50 | 1–200 | Notifications returned by `GET /notifications` |
| `api.session.ttl_hours` | 168 | 1–8760 | Session lifetime; applies to newly issued sessions |
| `api.session.prune_interval_seconds` | 3600 | 60–86400 | How often expired sessions and abandoned OAuth handshakes are deleted |
| `api.cache.hot_issues_ttl_seconds` | 120 | 0–3600 | Cache for the default unauthenticated catalog page (0 = off) |
| `api.cache.search_ttl_seconds` | 60 | 0–3600 | Cache for unauthenticated search results (0 = off) |
| `api.cache.recommendations_ttl_seconds` | 300 | 0–3600 | Cache for per-user recommendations (0 = off) |
| `api.cache.public_max_age_seconds` | 60 | 0–3600 | `Cache-Control` max-age on catalog and search responses to signed-out callers (0 = `no-cache`) |
| `api.cache.public_stale_seconds` | 300 | 0–86400 | `stale-while-revalidate` on the same responses |
| `api.stats.max_age_seconds` | 10 | 0–3600 | Public cache lifetime of `GET /stats` (the landing page polls it) |
| `api.cache.redis_timeout_ms` | 250 | 10–5000 | Deadline for one Redis call, connecting included; a call that misses it fails open |
| `api.cache.redis_retry_seconds` | 5 | 1–300 | After a failed Redis call, how long Redis is skipped before one call probes for recovery |
| `api.http.max_body_bytes` | 1048576 | 1024–104857600 | Largest request body on any API route except the webhook; larger gets 413 |
| `api.webhook.max_body_bytes` | 26214400 | 1024–104857600 | Largest GitHub webhook body accepted; larger gets 413 |
| `http.outbound_timeout_seconds` | 15 | 1–120 | Deadline for each outbound call to GitHub (sign-in, installation token, the crawl's GraphQL) and to ingestd; read per call. Used by both binaries |
| `api.issues.max_limit` | 100 | 1–500 | Largest `?limit` on `GET /issues`; larger is clamped |
| `api.issues.max_page` | 200 | 1–10000 | Deepest `?page` on `GET /issues`; deeper is clamped |
| `api.recommendations.max_limit` | 20 | 1–100 | Largest `?limit` on `GET /recommendations`; larger is clamped |
| `api.recommendations.candidate_pool` | 200 | 10–1000 | Open issues `GET /recommendations` reads off the shuffled feed (caller's languages and level) before scoring |
| `api.themes.max_per_user` | 20 | 1–200 | Most themes one account may keep |
| `api.themes.max_bytes` | 65536 | 1024–524288 | Largest theme (TOML + stylesheet) in bytes |
| `api.bodies.fallback_misses_per_hour` | 30 | 0–10000 | Issue bodies one caller may have fetched from GitHub per hour through `GET /issues/{id}/body`; cached bodies do not count (0 = cache only) |
| `api.issues.filter_max_values` | 20 | 1–100 | Most values one `GET /issues` filter may name; more is a 400 |
| `api.issues.query_timeout_ms` | 10000 | 100–25000 | Deadline for the queries behind one `GET /issues` or `GET /issues/{id}`; a query that misses it is cancelled and the caller gets 503. Keep it below the server's 30 s write timeout |
| `api.issues.rare_language_threshold` | 20000 | 0–1000000 | Browsing a language (`?language=`, or a signed-in caller's preferred languages) with fewer open issues than this sorts that language's issues instead of walking the catalog index (0 = always walk) |
| `api.issues.rare_probe_ms` | 500 | 0–10000 | Time limit on counting a filter combination to choose how to read it; slower = read in index order |
| `api.issues.shuffle_walk_max_repos` | 3000 | 0–1000000 | Repositories one shuffled-feed page walks before reading the rest from the matching issues |
| `api.issues.shuffle_walk_budget_ms` | 2000 | 0–9000 | Time one shuffled-feed page may look before it is returned short, with a cursor to continue from |
| `api.issues.language_count_ttl_seconds` | 600 | 0–86400 | How long each language's counted open issues are reused for that decision (0 = count every request) |

## Ingestion

| Key | Default | Bounds | What it does |
|---|---:|---|---|
| `ingest.eligibility.min_stars` | 25 | 0–100000 | Minimum stars to be crawled (abuse control, not ranking) |
| `ingest.eligibility.pushed_within_days` | 730 | 1–3650 | Repository must have been pushed within this many days |
| `ingest.eligibility.recheck_seconds` | 3600 | 60–86400 | How often stored repositories are re-tested against the thresholds |
| `ingest.routine_workers` | 2 | 1–64 | Concurrent Initial/Freshness workers |
| `ingest.priority_workers` | 1 | 1–64 | Concurrent Deep Diver workers |
| `ingest.graphql_page_size` | 50 | 1–100 | Issues per GraphQL page |
| `ingest.rate_limit_floor` | 100 | 0–5000 | Stop below this many remaining GitHub points. On the crawler lane it is measured above the interactive reserve; the interactive lane stops here |
| `ingest.interactive_reserve_percent` | 30 | 0–90 | Share of the GitHub rate limit the crawler lane (crawl, discovery) never spends, kept for interactive work such as tracked-issue checks (0 = no reserve) |
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
| `ingest.discovery.enabled` | false | bool | Run the Discovery Sweeper (GitHub repository Search for repositories the dataset lacks) |
| `ingest.discovery_workers` | 1 | 1–16 | Concurrent Discovery Sweeper workers (one search page each) |
| `ingest.discovery.window_hours` | 24 | 1–720 | Pushed-date span of each window a new sweep starts with |
| `ingest.discovery.min_window_seconds` | 60 | 1–86400 | Smallest span a window over the 1,000-result search cap is halved to; past it the window is read truncated |
| `ingest.discovery.page_size` | 100 | 1–100 | Repositories per search page |
| `ingest.discovery.rate_limit_floor` | 1000 | 0–12500 | Discovery's floor on the shared budget, above the interactive reserve; above `ingest.rate_limit_floor` so discovery yields first |
| `ingest.discovery.incremental_interval_seconds` | 86400 | 3600–2592000 | How often an incremental sweep covers repositories pushed since the last completed sweep. Not run while the archive reader is current |
| `ingest.discovery.full_interval_seconds` | 2592000 | 86400–31536000 | How often a full sweep covers the whole `pushed_within_days` range |
| `ingest.catalog.shuffle_interval_seconds` | 86400 | 3600–2592000 | How often the shuffled feed's repository order is redrawn |
| `ingest.catalog.facets_refresh_seconds` | 86400 | 3600–2592000 | How often the filter dropdowns' values and counts (`catalog_facets`) are recomputed |
| `ingest.catalog.facets_max_values` | 300 | 10–5000 | Values kept per filter kind, most common first |
| `ingest.stats.rollup_seconds` | 5 | 1–3600 | How often ingestd folds the issue-change deltas into the live counts |
| `ingest.stats.recount_seconds` | 3600 | 60–86400 | How often the open-issue and repository totals are recounted exactly |
| `ingest.stats.activity_rows` | 50 | 0–1000 | Recent crawl/archive commits `GET /stats` lists as activity |
| `ingest.bodies.cache_max_mb` | 512 | 0–100000 | Size the issue body cache is trimmed back to, least recently read first; saved issues' bodies are always kept (0 = keep only those) |
| `ingest.bodies.prune_interval_seconds` | 3600 | 60–86400 | How often the issue body cache is trimmed |
| `ingest.archive.enabled` | false | — | Read GH Archive's hourly public event files and apply their issue events to stored repositories |
| `ingest.archive.workers` | 1 | 1–8 | Archive hours processed at once |
| `ingest.archive.poll_seconds` | 300 | 30–3600 | How often newly ended hours are queued |
| `ingest.archive.retry_seconds` | 600 | 60–86400 | Wait before retrying an hour not published yet, or whose processing failed |
| `ingest.archive.missing_after_hours` | 24 | 1–720 | An hour still unpublished this long after it ended is marked `missing` |
| `ingest.archive.max_backfill_hours` | 168 | 1–2160 | How far back the first run goes, at most (it starts at the oldest completed crawl pass) |
| `ingest.archive.download_timeout_seconds` | 300 | 10–3600 | Deadline for downloading and reading one hour |
| `ingest.archive.max_file_mb` | 512 | 1–4096 | Largest compressed file accepted; a larger one fails its hour |
| `ingest.archive.apply_batch_size` | 500 | 1–10000 | Issues written per transaction |
| `ingest.archive.max_lag_seconds` | 21600 | 3600–604800 | Further behind than this, the archive is not current: the freshness crawl returns to `ingest.freshness_interval_seconds` and the incremental sweep resumes |
| `ingest.archive.repair_interval_seconds` | 2592000 | 86400–31536000 | The freshness crawl's interval while the archive is current |
| `ingest.archive.candidate_batch_size` | 50 | 1–100 | Archive candidates (repositories not stored) looked up per GitHub call |
| `ingest.archive.candidate_recheck_days` | 30 | 1–3650 | A candidate that was not stored is looked up again after this long, if active since |

Raising the eligibility thresholds takes effect on the next recheck. **Lowering** them only
affects repositories already stored — anything that failed the old threshold was never imported,
so re-run `cmd/repoimport` (see `README.md`).

If `ingest_job_outcomes_total{outcome="lease_lost"}` climbs, jobs are outliving
`ingest.lease_seconds` (usually a long pool queue during a GitHub slowdown) — raise it. A lost
lease is safe: the stale worker's write is rejected, and the job is retried by its new owner.

## Flow (live visualizer)

The tap settings are read by go-api and ingestd on every event; the rest by flowd.

| Key | Default | Bounds | What it does |
|---|---:|---|---|
| `flow.enabled` | true | bool | Publish flow events from go-api and ingestd. Off, nothing is buffered or published and flowd serves an idle stream |
| `flow.buffer_size` | 1024 | 64–65536 | Events a publisher holds while Redis catches up, and lines flowd holds for one slow viewer; past it, events drop (`flow_events_dropped_total`) |
| `flow.publish_timeout_ms` | 250 | 10–5000 | Deadline for one `PUBLISH` of a batch |
| `flow.max_label_chars` | 80 | 8–500 | Longest public label or payload (an issue title, a repository name); longer text is truncated |
| `flow.max_events_per_second` | 40 | 1–1000 | Labelled events flowd relays per second; every event still counts towards its link's density |
| `flow.max_viewers` | 200 | 1–5000 | Concurrent `/stream` viewers; past it, 503 |
| `flow.density_interval_ms` | 1000 | 100–60000 | Window flowd counts events per link over before sending each link's density |
| `flow.heartbeat_seconds` | 15 | 1–300 | Keep-alive interval on an idle stream; a write that cannot finish in two of these drops the viewer |
| `flow.ambient_refresh_seconds` | 5 | 1–300 | How often flowd reads cAdvisor network rates from Prometheus for uninstrumented links |
| `flow.ambient_rate_window_seconds` | 120 | 30–3600 | `rate()` window for those; must span at least two cAdvisor scrapes (30s) |

## Adding a setting

Add the key to `settings.Registry` **and** seed it in a new migration (see
`00016_ingest_metrics_refresh_setting.sql`). A key in one but not the other falls back to its
default, and a key missing from the table is logged at every load. Read values through `live.Current()` and hold the
snapshot for the duration of an operation.
