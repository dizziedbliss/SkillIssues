# Ingestion (`internal/ingest`, run by `cmd/ingestd`)

Design of record: `docs/ingestion-architecture.md`. Read it before changing anything here.
**Collect broadly first; analyse later.** Scope is open issues, most recently updated first: not every
comment, reaction, event or closed issue.

## Checks

```bash
cd backend
go test ./internal/ingest/...                    # needs TEST_DATABASE_URL for the database tests
go test ./internal/ingest/ -run TestClaimJobs    # one test
```

## The cycle (`coordinator.go`, every `ingest.claim_interval_seconds`)

Recompute eligibility (throttled) → reclaim expired leases (throttled) → seed never-ingested
repositories → re-queue stale ones → check tracked (saved / in-progress) issues on GitHub → claim and
submit Initial and Deep work → if `ingest.discovery.enabled`, start a due sweep and claim its windows.
If `ingest.archive.enabled`, the same cycle (before the disk-guard early return) queues newly ended
archive hours, claims due ones, and looks up one batch of repository candidates.

Initial = a repository's first page of open issues; Deep = every remaining page from the stored cursor;
Freshness = once `last_synced_at` is older than `ingest.freshness_interval_seconds`, only what changed
since the last completed pass (open issues updated since, upserted; issues closed since, marked CLOSED).

## Rules

- **Every eligible repository enters the pipeline.** No priority, ranking, activity, demand or star
  score anywhere in the crawl path. Ordering is FIFO on `next_retry_at`; freshness orders by oldest
  successful sync. That's fairness, not a judgement of value.
- **Eligibility is abuse control and pacing, not quality.** The star floor exists because anyone can
  open issues on a throwaway repository to burn crawl budget.
- **`ingest.RecomputeEligibility` and `ingest.Eligible` apply the same rule.** Change one, change both.
  Recompute never re-judges `not_found` or `duplicate` rows. Tightening is live (recompute); loosening
  means re-running `cmd/repoimport` (~16 min, idempotent), because only eligible repositories are stored.
- **Excluded repositories** (`excluded_repositories`, migration 00028) are ineligible (`excluded`):
  recompute applies the list, `cmd/repoimport` checks it before `eligible()`, and `ClaimJobs` claims only
  eligible repositories' jobs, so a queued job stays `pending` (parked, `ingest_jobs_parked`). The
  coordinator deletes unsaved issues of excluded, archived and `duplicate` repositories on the
  closed-issue prune interval. Add one with
  `INSERT INTO excluded_repositories (full_name, reason) VALUES ('owner/name', 'why')`.
- **Unknown is not a negative.** A NULL metadata field means "not imported yet", never "fails the check".
- **`github_database_id` is a repository's identity; `full_name` is display data.** Found by id under a
  new name → renamed. An id-less row that is the same repository as one holding its id → `duplicate`
  (never crawled or re-judged, unsaved issues pruned, nothing deleted). A name now held by a different
  repository is reported (`repository_identity_conflicts_total`) and left alone.
- **Cursors advance only after the rows they describe are committed**, in the same transaction, with
  `repositories.last_synced_at`. Never mark a repository complete on a truncated run.
- **The watermark only ever trails the truth.** `repositories.issues_synced_through` is the *enqueue*
  time (`ingest_jobs.pass_started_at`) of the last completed pass, set in the transaction of its last
  page. NULL means the next pass is a full crawl. A pass pages two connections in lockstep
  (`cursor`/`open_done`, `closed_cursor`/`closed_done`) and is done only when both are exhausted or the
  page cap is hit.
- **Closed issues are detected, never inferred.** Only issues GitHub reports closed since the watermark
  are marked `CLOSED`; "not seen in this pass" is not evidence (the page cap hides many open issues).
  Saved / in-progress issues are re-checked directly every `ingest.tracked_issue_check_seconds`.
- **Closed issues are pruned after `ingest.closed_issue_retention_days`, never one in `saved_issues`**
  (it would cascade).
- **Metadata is live once crawled.** A pass's first page writes current stars, forks, watchers,
  open-issue count, archived flag and last push (`metadata_refreshed_at`; NULL = dataset snapshot).
  `issue_count` stays the dataset's open+closed total (the import's `no_issues` rule reads it);
  `open_issue_count` is the live count. `contributing_url` keeps only an https github.com page.
- **One failing repository never blocks the others.** `NOT_FOUND` → unavailable;
  `RESOURCE_LIMITS_EXCEEDED` (and a 502/504 on search) → halve the page size, down to 1; secondary
  rate limit → pause the lane; anything else → exponential backoff, `failed` past `ingest.max_attempts`.
- **One GitHub budget** (`github_budget`, lanes `crawler` and `interactive`), claimed atomically with
  `FOR UPDATE SKIP LOCKED` plus a lease. Never assume a worker owns the whole budget. The crawler lane
  stops while `ingest.interactive_reserve_percent` is left; `interactive` (work a user waits on)
  spends down to `ingest.rate_limit_floor`. New user-facing GitHub work goes on `interactive`, new
  background work on `crawler`. One GitHub App installation: never add an identity for more limit.
- **Every job write is fenced on its lease.** A write that finishes a job matches
  `status = 'running' AND lease_owner = <claimer>` and treats zero rows as `ErrLeaseLost` (rolling back
  the page's transaction). Never a bare `WHERE id = $1`. Leases are reclaimed every
  `ingest.lease_reclaim_interval_seconds`, so a stalled worker's job can be re-claimed while it runs.
- **Postgres is the queue.** `ingest_jobs` is the durable state; Redis is not involved.
- **Disk guards are live settings:** `ingest.max_db_size_mb` (stop enqueuing and claiming) and
  `ingest.max_pages_per_repo` (cap the Deep Diver regardless of `hasNextPage`). The host is 40 GB.

## Discovery (`discovery.go`, off by default)

Searches `stars:>=min_stars fork:false archived:false pushed:<window>`; stores what passes
`ingest.Eligible` and isn't excluded (`upsertDiscoveredTx`). Full sweep every
`ingest.discovery.full_interval_seconds`, incremental (since the last `done` sweep) every
`ingest.discovery.incremental_interval_seconds`. A window over GitHub's 1,000-result cap is replaced by
its halves down to `ingest.discovery.min_window_seconds`. `discovery_windows` is a leased, fenced queue;
a sweep with a failed window ends `failed`, doesn't advance the watermark, and is reopened after
`ingest.discovery.failed_retry_seconds` (`RetryFailedSweep`). Discovery yields first: it checks the
`crawler` budget against its own higher `ingest.discovery.rate_limit_floor`. `POST /sync/discovery`
(ingestd, admin) starts a full sweep.

## GH Archive reader (`archive.go`, off by default)

One gzipped hourly file, streamed. IssuesEvent and IssueCommentEvent for stored, eligible repositories go
through `writeIssueTx`: inserted, or updated only if newer. A closed issue never stored is not inserted;
nothing is inserted while the disk guard is on; pull requests are skipped. `archive_hours` is a leased,
fenced queue. While every hour up to `ingest.archive.max_lag_seconds` ago is done or `missing`,
freshness drops to `ingest.archive.repair_interval_seconds` and the incremental discovery sweep stops; a
`failed` hour brings the daily crawl back (`archive_fallback`). Archive writes never touch
`last_synced_at` or `issues_synced_through`. Reset a failed hour with
`UPDATE archive_hours SET status = 'pending', attempts = 0 WHERE hour = ...`.

Unknown repositories become `repo_candidates`, looked up with `github.LookupRepositories` (≤100 per call,
crawler lane at discovery's floor) and stored through `upsertDiscoveredTx`. One not stored is re-looked
up after `ingest.archive.candidate_recheck_days` if active since, and deleted then if quiet
(`PruneRepoCandidates`).

## Not built

- **`'discovery'` job type:** permitted by `ingest_jobs.job_type`, never written (discovery has its own
  tables). Tests use it as a type no coordinator claims.
- **`'freshness'` job type:** stale repositories are re-queued as `initial`.
- **Pull requests from the archive:** `PullRequestEvent` is skipped; archive bodies only refresh bodies
  already cached.

## Gotchas

- An ingestion "stoppage" has been a finished crawl before. Check `ingest_jobs` states and
  `issues_synced_through` before debugging workers.
