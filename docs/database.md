# Database reference

Every table and column in production, what it means, where its value comes from, and how
fresh it is. Checked against prod data on 2026-10-16 and updated for the schema at **migration 33** (the
last migration in `backend/internal/migrate/migrations/`). Row counts below are from that
date's prod data and will have moved.

**Where values come from** — the single most useful thing to know when a number looks wrong:

| Source | Written by | Freshness |
|---|---|---|
| **dataset** | `cmd/repoimport`, from the Kaggle repository-metadata JSON | a **snapshot** — only as new as the dataset file (prod's is ~Sep 2025). Stars, forks, watchers, archived and last push are overwritten by the crawl once a repo is crawled after #30 (`metadata_refreshed_at` says when). |
| **crawl** | `ingestd` (GitHub GraphQL) | refreshed every time the repository/issue is crawled |
| **on-demand** | `go-api`, when a user acts (freshness check, webhook) | at the moment of the action |
| **app** | `go-api`, from user activity | live |

---

## repositories — 287k rows

One row per repository that passed the import filter. The crawl only reads from it; nearly
every column is a **dataset snapshot**.

| Column | Meaning | Source | Notes |
|---|---|---|---|
| `id` | Our id | — | |
| `full_name` | `owner/name` | dataset | Unique (case-insensitive). How everything refers to a repo. |
| `url` | `https://github.com/owner/name` | dataset | |
| `description` | Repo description | dataset | |
| `language` | Primary language | dataset | Empty → ineligible (`no_primary_language`). |
| `languages` | All languages, JSON `[{name,size}]` | dataset | Used by recommendations. |
| `topics` | GitHub topics | dataset | |
| `license` | SPDX license id | dataset | |
| `stars` | Stargazers | dataset → crawl | Live once crawled. Drives the star-floor eligibility rule. |
| `forks` | Fork count | dataset → crawl | The snapshot could be **higher** than GitHub today (forks get deleted); live once crawled. |
| `watchers` | Watchers | dataset → crawl | Live once crawled. |
| `issue_count` | **Open + closed** issues ever opened | dataset | Snapshot only, never refreshed, and it counts closed ones. Used only by eligibility (`0` → `no_issues`). |
| `open_issue_count` | Open issues on GitHub right now | crawl | What `issue_count` is usually mistaken for. NULL until crawled after #30. |
| `is_archived` | Archived on GitHub | dataset → crawl | Archived → ineligible. |
| `forking_allowed` | Repo allows forking | dataset | Needed before a user can fork to contribute. |
| `created_at` | When the repo was created on GitHub | dataset | |
| `pushed_at` | Last push to any branch | dataset → crawl | Drives the `stale` rule (`ingest.eligibility.pushed_within_days`). |
| `metadata_refreshed_at` | When the crawl last wrote the live stats above | crawl | NULL = the row still holds the dataset snapshot. |
| `is_eligible` | Allowed into the crawl | import + crawl | Recomputed live from the thresholds (`RecomputeEligibility`). |
| `eligibility_reason` | Why not eligible; `''` when eligible | import + crawl | `;`-separated: `is_fork`, `archived`, `no_issues`, `no_primary_language`, `stale`, `below_star_floor`, `excluded`, or `not_found` (crawler got NOT_FOUND from GitHub — deleted/renamed/private; never reset by a re-import, cleared when the Discovery Sweeper finds the repository again) or `duplicate` (another row holds this repository's GitHub id; never reset by a recompute or re-import). Prod today: 284,198 eligible, 2,203 `not_found`, 864 `stale`. |
| `last_synced_at` | Last time the crawler finished a page for this repo | crawl | NULL = never crawled yet (52k on 2026-10-16; 0 since the first full crawl finished on 2026-10-17 — see `ingestion-architecture.md` §9). Freshness is due once this is older than `ingest.freshness_interval_seconds`. |
| `issues_synced_through` | Watermark: every open/closed change up to here is in `issues` | crawl | The enqueue time of the last **completed** pass. The next pass fetches only changes after it. NULL = no pass completed; next one is a full crawl. |
| `github_database_id` | GitHub's numeric repository id | crawl / discovery | Stable across renames, unlike `full_name`; unique. Filled by the first page of every pass and by the Discovery Sweeper (migration 32). NULL = not seen by either yet. A row whose id another row already holds is the id-less twin of that repository and is marked `duplicate`. |

## issues — 5.5M rows

Issues of eligible repositories: open ones, plus closed ones until they are pruned
(`ingest.closed_issue_retention_days`, default 7) — except closed issues someone saved, which are
kept forever. Identity is `(repository_id, number)`.

| Column | Meaning | Source | Notes |
|---|---|---|---|
| `id` | Our id | — | |
| `repository_id` | → `repositories.id` | crawl | |
| `number` | Issue number in its repo (`#123`) | crawl | |
| `github_id` | GitHub's numeric issue id (`databaseId`) | crawl | Returned by the API; not unique-constrained. |
| `title` | Title | crawl | Sanitised. Feeds full-text search. |
| `url` | Link to the issue on GitHub | crawl | |
| `state` | `OPEN` / `CLOSED` | crawl / on-demand | `CLOSED` comes from: a freshness pass (GitHub's "closed since last pass" list), the hourly check of saved/in-progress issues, the desktop "start challenge" check, or a webhook. Going `OPEN`→`CLOSED` notifies users who saved / are working on it. |
| `labels` | Raw GitHub labels, as-is | crawl | Never derived; the source for everything below. |
| `difficulty` | `BEGINNER` / `INTERMEDIATE` / `ADVANCED` / `UNCLASSIFIED` | derived from labels | No difficulty label → `UNCLASSIFIED` (unknown, not easy). |
| `difficulty_score` | Numeric difficulty | derived from labels | NULL when `UNCLASSIFIED`. Used for sorting. |
| `required_skills` | Skills inferred from labels | derived | Search + recommendations. |
| `technologies` | Technologies inferred from labels + repo language | derived | Search + recommendations. |
| `learning_tags` | Tags like `good-first-issue` | derived | Search + shown in UI. |
| `comments` | Comment count | crawl | |
| `author` | Who opened it (login) | crawl | `open-source` when GitHub returns none (ghost user). |
| `created_at` | When it was opened **on GitHub** | crawl | |
| `updated_at` | Last change **on GitHub** (edit, comment, label, close…) | crawl | Equal to `created_at` for an issue nobody has touched since opening (~20%). Not our fetch time. For a closed issue it is the closing time, which the prune uses. |
| `first_seen_at` | When **we** first stored it | crawl | Never changes. NULL for issues stored before migration 19 (2026-10-14). |
| `last_synced_at` | When **we** last wrote GitHub data to it | crawl / on-demand | NULL for issues not re-fetched since migration 19. |
| `search_vector` | Full-text index of title + labels | trigger | Maintained automatically. |

## ingest_jobs — the crawl queue

One row per unit of crawl work. At most one `pending`/`running` job per repository.

| Column | Meaning |
|---|---|
| `id` | |
| `repository_id` | → `repositories.id` |
| `job_type` | `initial` (first page of a pass), `deep` (remaining pages). `freshness` and `discovery` are allowed but nothing writes them — a freshness pass is an `initial` job with `since` set, and discovery has its own tables (`discovery_sweeps`, `discovery_windows`). |
| `status` | `pending` → `running` → `done` / `failed` |
| `attempt_count` | Failed attempts so far; past `ingest.max_attempts` → `failed` |
| `lease_owner` / `lease_expires_at` | Which worker holds it and until when. Expired leases are reclaimed. |
| `next_retry_at` | Earliest time it may be claimed (backoff) |
| `cursor` / `open_done` | Position in the open-issues list, and whether it is exhausted |
| `closed_cursor` / `closed_done` | Position in the closed-since-watermark list, and whether it is exhausted |
| `since` | The watermark this pass fetches changes after; NULL = full crawl |
| `pass_started_at` | When the pass's first job was queued; becomes `issues_synced_through` when the pass completes |
| `pages_done` | Pages fetched (capped by `ingest.max_pages_per_repo`) |
| `has_next_page` | The pass has more pages to fetch |
| `last_error_class` / `last_error_detail` | Why the last attempt failed (`not_found`, `rate_limited`, …) |
| `created_at` / `updated_at` | Row bookkeeping |

## discovery_sweeps / discovery_windows — the Discovery Sweeper

`discovery_sweeps`: one pass of GitHub repository Search over a pushed-date range — `kind`
(`full`: the whole `ingest.eligibility.pushed_within_days` range; `incremental`: since the last
completed sweep), `pushed_from` / `pushed_to`, `min_stars` (the star floor its queries use),
`status` (`running` → `done` / `failed`; at most one `running`), `started_at`, `completed_at`.
`pushed_to` of the latest `done` sweep is the discovery watermark; a sweep with a failed window
ends `failed` and does not move it.

`discovery_windows`: the sweep's queue, leased and fenced like `ingest_jobs` — `sweep_id`,
`pushed_from` / `pushed_to`, `status`, `cursor`, `pages_done`, `repository_count` (GitHub's match
count on the first page), `repos_seen`, `split` (replaced by its two halves: over the 1,000-result
search cap), `truncated` (over the cap at `ingest.discovery.min_window_seconds`, so only the first
1,000 were read), `attempt_count`, `lease_owner` / `lease_expires_at`, `next_retry_at`,
`last_error_class` / `last_error_detail`, `created_at` / `updated_at`.

## github_budget

The shared GitHub API budget, read from GitHub's rate-limit replies.

| Column | Meaning |
|---|---|
| `identity` | Which credential (server App installation / token) |
| `lane` | `crawler` (ingestd) or `interactive` (reserved: nothing uses it since issue bodies and freshness moved to the clients) |
| `rate_limit` / `remaining` / `reset_at` | GitHub's primary rate limit for this lane |
| `secondary_until` | Paused until this time after a secondary (abuse) rate limit |
| `updated_at` | Last refresh |

## users

One row per GitHub account. The server stores **no GitHub token of any kind**; the numeric
`github_id` is the only identity.

| Column | Meaning |
|---|---|
| `id` | |
| `github_id` | GitHub's numeric user id (text, `NOT NULL`, unique). The only thing an account is matched on. |
| `username` | GitHub login, display data only; NULL when another account has taken the name (unique, case-insensitive) |
| `name` / `avatar_url` | From the GitHub profile |
| `created_at` | Sign-up time |

## auth_sessions

| Column | Meaning |
|---|---|
| `token` | Session cookie / bearer value (secret; not readable by `metrics_ro`) |
| `user_id` | → `users.id` |
| `provider` | How they signed in (`github_oauth` for web, `github_device` for desktop) |
| `expires_at` / `created_at` | Session lifetime (`api.session.ttl_hours`); expired rows are pruned (`api.session.prune_interval_seconds`) |

## oauth_states

Short-lived rows for an in-flight GitHub sign-in: `state`, PKCE `code_verifier`,
`redirect_uri`, `expires_at`, `created_at`. Secret; not readable by `metrics_ro`. Abandoned
ones are pruned with the sessions.

## saved_issues

A user's bookmarked issues: `user_id`, `issue_id`, `created_at` (when saved).

## developer_profiles

`user_id`, `bio`, `experience_level` / `target_level` (self-declared
`BEGINNER`/`INTERMEDIATE`/`ADVANCED`) and `demonstrated_skills`. XP, level and badges were
removed with contributions (migration 00027).

## developer_preferences

`user_id`, `interests`, `preferred_languages` — feed recommendations.

## notifications

Written by the database (trigger `issues_notify_closed`), read by `GET /notifications`.

| Column | Meaning |
|---|---|
| `id` | |
| `user_id` | Who to tell |
| `kind` | `issue_closed` — an issue they saved closed |
| `issue_id` | → `issues.id` |
| `created_at` | When it was raised (bumped if the issue reopens and closes again) |
| `read_at` | When they marked it read; NULL = unread |

## webhook_events

GitHub webhook deliveries already processed (dedupe): `delivery_id`, `event_name`,
`processed_at`. Empty today — webhooks only arrive from repos with the GitHub App installed. A delivery whose issue `updated_at` is not newer than the stored one changes nothing.

## settings / settings_audit

Live operational settings (`docs/settings.md`). `settings`: `key`, `value` (JSON),
`value_type`, `min_value`/`max_value` (enforced by trigger), `description`, `updated_at`,
`updated_by`. `settings_audit`: every change — `key`, `old_value`, `new_value`, `changed_at`,
`changed_by`. `settings` is the only table `skillissues_flow` (flowd) can read.

## excluded_repositories

Repositories that pass every threshold and are still never crawled (`full_name`, `reason`,
`created_at`; unique on `lower(full_name)`). Applied by the eligibility recompute, the importer
and job claiming. Add one with `INSERT INTO excluded_repositories (full_name, reason) VALUES (...)`.

## import_skipped

Dataset records the importer rejected: `full_name`, `reason` (same codes as
`eligibility_reason`), `skipped_at`. Empty in prod.

## goose_db_version

The migration tool's own bookkeeping — which migrations have run.

---

## Known gaps (2026-10-16)

- `issue_count` is still the dataset's open+closed snapshot; use `open_issue_count` once a repo has been crawled after #30.
- Issues stored before migration 19 have no `first_seen_at`.
