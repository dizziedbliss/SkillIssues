# SkillIssues Ingestion Architecture

**Updated:** 11 October 2026 (design) · implementation status checked 17 October 2026  
**Purpose:** Build a broad, reliable issue dataset first. Data-science ranking and repository prioritization come later, after there is enough data to analyze.

This is the design of record for `backend/internal/ingest/`. It describes intent; the table
below says what the code actually does today. Where they differ, the code is the truth and
this table is how you find out.

## Implementation status

| Design element | Status | Where / how it differs |
|---|---|---|
| Repository catalog seeded from the dataset | **Built** | `cmd/repoimport`; only eligible repositories are stored |
| Eligibility (stars, pushed-within) re-checked live | **Built** | `ingest.RecomputeEligibility`, every `ingest.eligibility.recheck_seconds` |
| Initial worker — one page per eligible repository | **Built** | `ingest.Worker`, `job_type='initial'` |
| Freshness — the incremental window when stale | **Built** | re-enqueued as `job_type='initial'`, not `'freshness'` (the CHECK constraint allows it; nothing writes it). Fetches only open issues updated since `repositories.issues_synced_through` (the last completed pass), continuing into Deep Diver only while that window has more pages |
| Closed issues marked `CLOSED` | **Built** | the same request also pages issues closed since the watermark; issues users saved or are working on are re-checked directly every `ingest.tracked_issue_check_seconds` |
| Deep Diver — remaining pages from the stored cursor | **Built** | `job_type='deep'`; capped by `ingest.max_pages_per_repo` |
| Atomic claims with `FOR UPDATE SKIP LOCKED` + lease | **Built** | expired leases reclaimed continuously; job writes fenced on `lease_owner` |
| Cursor + issues committed in one transaction | **Built** | `CompleteOrRequeue` runs in the page's transaction |
| Shared GitHub budget (`github_budget`, crawler/interactive lanes) | **Built** | one counter read across both lanes; the crawler lane leaves `ingest.interactive_reserve_percent` for interactive work (tracked-issue checks) |
| `RESOURCE_LIMITS_EXCEEDED` recovery | **Built, differently** | no GraphQL alias batching: one repository per call, and the *page size* is halved down to 1 (§4 describes splitting alias batches) |
| Secondary rate limits | **Built** | both lanes pause for `ingest.rate_limit_pause_seconds`, or as long as GitHub asks (`Retry-After`, or the primary limit's reset when none is left). A 403 that is not a rate limit (a permissions refusal) backs off like any other failure. Errors are typed (`github.IsRateLimited`, `IsResourceLimit`, `IsNotFound`), never matched on their text |
| `NOT_FOUND` isolates the repository | **Built** | job `failed`, repository marked ineligible |
| NUL-byte sanitisation before persistence | **Built** | `ingest.SanitizeText` |
| Metrics distinguishing retries from failures | **Built** | `ingest_job_outcomes_total{outcome}` on ingestd's `/metrics` |
| GH Archive reader (hourly public events) | **Built** (off by default) | `ingest/archive.go`; its own queue (`archive_hours`). Applies issue events to stored repositories when newer; while it is current, freshness drops to `ingest.archive.repair_interval_seconds` and the incremental discovery sweep stops. See *GH Archive reader* below |
| Discovery Sweeper (GitHub Search over date windows) | **Built** (off by default) | `ingest/discovery.go`; its own queue (`discovery_sweeps`, `discovery_windows`), not `ingest_jobs` — nothing writes `job_type='discovery'`. Windows are over **pushed** date (not created), deduplication is by `repositories.github_database_id`. See *Discovery Sweeper* below |

Sections 4 and 6 record observations from the Python v6 prototype (still in `benchmark/`);
they are history, not a description of the Go worker.

## 1. Current goal

For the current phase, **coverage comes before prioritization**. Ingest the in-scope issues for every eligible repository in the catalog as quickly and safely as the GitHub API allows. Do not rank repositories by activity, clicks, demand, stars, or a hand-written score to decide which repositories receive deeper ingestion.

The initial scope follows the current importer query: **open issues**, ordered by most recently updated. It does not mean fetching every comment, reaction, event, or closed issue. Expand that scope deliberately if the product later needs those records.

Once we have a sufficiently broad dataset, we can analyze it and use evidence to design data-science-based ranking. That is a later phase, not part of the ingestion policy today.

## 2. Pipeline overview

```text
Repository catalog / snapshot
          |
          v
Discovery Sweeper -----> new eligible repositories
          |                         |
          +-------------------------+
                                    v
                         Initial / Freshness Worker
                         fetch first issue page
                                    |
                         persist issues + cursor
                                    |
                          hasNextPage = true?
                                    |
                                    v
                              Deep Diver Queue
                                    |
                                    v
                               Deep Diver
                         fetch next page, persist
                         cursor, requeue if needed
                                    |
                                    v
                           PostgreSQL issue index
```

All stages should be able to run concurrently, with shared rate-limit coordination and safe queue claims. The queue is for tracking work and progress, **not for ranking repository value at this stage**.

### Discovery Sweeper

- Seed the catalog from the existing repository dataset.
- Use GitHub repository Search with date windows to discover newly created or recently pushed repositories.
- Split dense search windows into smaller time ranges when needed; persist completed work before advancing the discovery watermark.
- Deduplicate by stable repository ID. New eligible repositories enter the initial-ingestion queue.
- Keep repository discovery separate from issue pagination.

As built (migration 00032, `internal/ingest/discovery.go`), with `ingest.discovery.enabled` on:

- **Query.** `stars:>=<min_stars> fork:false archived:false pushed:<from>..<to>` through GraphQL
  `search(type: REPOSITORY)`. It uses the eligibility thresholds, so it finds brand-new
  repositories, repositories that crossed the star floor since the snapshot, and ones the
  dataset never had.
- **Sweeps.** A `full` sweep covers the whole `ingest.eligibility.pushed_within_days` range
  every `ingest.discovery.full_interval_seconds` (30 days). An `incremental` sweep covers
  `pushed_to` of the last `done` sweep up to now, every
  `ingest.discovery.incremental_interval_seconds` (1 day). Only one sweep runs at a time. A
  repository that crosses the star floor without a push is found only by the next full sweep.
- **Windows and the 1,000-result cap.** A sweep starts as `ingest.discovery.window_hours`
  slices. If GitHub's `repositoryCount` for a slice is over 1,000, the slice is replaced by its
  two halves in one transaction. This repeats down to `ingest.discovery.min_window_seconds`.
  Below that, only the first 1,000 are read: the window is marked `truncated` and
  `discovery_windows_truncated_total` goes up.
- **Queue.** One search page per claim, with `FOR UPDATE SKIP LOCKED`, a lease, and every write
  fenced on `lease_owner`. Each page's repository writes commit together with its cursor. The
  sweep closes once no window is pending or running. It closes `failed` if any window failed,
  and a failed sweep does not move the watermark.
- **Budget.** Discovery shares the `crawler` lane but checks it against
  `ingest.discovery.rate_limit_floor` (1,000), which is above the crawl's own floor, so
  discovery stops first when points run low. Both floors sit above the interactive reserve
  (`ingest.interactive_reserve_percent`, 30%): with a 12,500-point limit, the crawl stops at
  3,850 remaining and discovery at 4,750.
- **What it stores.**
  - New repository: inserted only if `ingest.Eligible` passes and it is not excluded, the same
    rule `cmd/repoimport` uses. `EnqueueInitialJobs` then queues it.
  - Known repository: gets the live stats, the same ones a crawl pass's first page writes, and
    has its `not_found` flag cleared.
- **Identity.** `repositories.github_database_id` is matched first, then `lower(full_name)`:
  - An id match under a new name renames the row.
  - If an id-less row already holds the new name, it is the same repository imported again,
    and it is marked `duplicate`: not crawled, nothing deleted.
  - A name held by a row with a different id is reported (`repository_identity_conflicts_total`)
    and left alone.
  - The crawl's first page of every pass fills the id for rows that lack it and applies the
    same `duplicate` rule.

### GH Archive reader

GH Archive publishes every public GitHub event as one gzipped JSON-lines file per hour,
`https://data.gharchive.org/YYYY-MM-DD-H.json.gz` (UTC, hour not zero-padded), roughly an hour
after the hour ends. Reading it costs no GitHub API budget.

- **What a file holds** (checked against 2026-10-18 15:00 UTC: 9.3 MiB, 19,226 events).
  - `IssuesEvent` (1,009) and `IssueCommentEvent` (1,240) carry the whole issue: labels, state,
    `state_reason`, `updated_at`, assignees and body. Only `author_association` is gone (removed
    from event payloads on 2025-10-07).
  - `PullRequestEvent` is slimmed to id, number and head/base repository, with a distinct
    `merged` action.
  - Every event carries the numeric `repo.id`.
- **Queue.** `archive_hours` has one row per hour, leased and fenced like `ingest_jobs`:
  - Every `ingest.archive.poll_seconds` the coordinator queues each hour that has ended. The
    first run starts at the oldest `issues_synced_through` of an eligible repository, but no
    earlier than `ingest.archive.max_backfill_hours` ago.
  - A 404 means not published yet: the hour is retried after `ingest.archive.retry_seconds`.
    After `ingest.archive.missing_after_hours` it is marked `missing`.
  - A processing failure counts an attempt, and the hour is `failed` after `ingest.max_attempts`.
- **Reading.**
  - The file is streamed, never held whole, and capped at `ingest.archive.max_file_mb`.
  - Only the newest state of each issue in the hour is kept.
  - Pull requests are skipped (issue events report them too).
- **Applying.**
  - An issue in a stored, eligible repository (matched by `github_database_id`, or by name for a
    row the crawl has not recorded an id for) goes through the crawl's `writeIssueTx`: inserted,
    or updated only if newer than what is stored. So hours can be read in any order, and again.
  - A closed issue that was never stored is not inserted, and while the disk guard is on nothing
    is.
  - Writes are `ingest.archive.apply_batch_size` per transaction.
  - Repository watermarks are never touched.
- **Coverage.**
  - Coverage ends at the first hour that is not `done` or `missing` (`archive_lag_seconds`).
  - Within `ingest.archive.max_lag_seconds`, the archive is *current*. The freshness crawl then
    runs every `ingest.archive.repair_interval_seconds` (30 days), to catch what events cannot
    show, and the incremental discovery sweep stops.
  - Otherwise both return to their normal cadence by themselves (`archive_fallback`). A `failed`
    hour keeps it that way until it is set pending again.
- **Candidates.**
  - A repository with issue activity that is not stored goes to `repo_candidates`.
  - Each coordinator cycle looks up one batch (`ingest.archive.candidate_batch_size`, aliased
    `repository(owner, name)` queries) on the crawler lane at discovery's floor.
  - Results are stored through the Discovery Sweeper's `upsertDiscoveredTx`: same eligibility,
    exclusion and identity rules.
  - One that was not stored is looked up again after `ingest.archive.candidate_recheck_days`, if
    it was active since. A quiet one is deleted then; if it turns active again, it is queued afresh.
  - Lookups continue each cycle while batches come back full; otherwise they wait
    `ingest.archive.poll_seconds`.
  - The 2026-10-18 15:00 hour named about 950 such repositories.

### Initial / Freshness Worker

- Claim an eligible repository that has not completed its initial fetch.
- Fetch one page of open issues ordered by `UPDATED_AT DESC` using a lean GraphQL query.
- Upsert the returned issue records.
- Save the cursor and `hasNextPage` state only after the matching database write succeeds.
- If `hasNextPage` is true, enqueue the repository for Deep Diver. Otherwise mark that traversal complete.
- On later freshness runs, re-fetch the first page or the configured incremental window to discover updates. Do not use this worker to crawl every page.

### Deep Diver

- Pull repositories with unfinished pagination from the Deep Diver queue.
- Fetch the next page from the stored cursor, upsert its issues, and atomically commit the new cursor and progress.
- If `hasNextPage` remains true, put the repository back into the queue. If false, mark the traversal complete.
- Make forward progress across **all queued eligible repositories**. Use fair scheduling or FIFO for now; no activity, popularity, product-demand, or staleness score is required.
- Use bounded batches and retry handling. A persistently expensive repository should be isolated or deferred instead of blocking other repositories.

## 3. Parallel execution

Yes, Discovery Sweeper, Initial/Freshness workers, and Deep Diver workers can run at the same time. The safe design needs these controls:

1. **Shared GitHub budget coordination:** workers must share the current rate-limit state and pause/backoff status. They must not each assume they own the full API budget.
2. **Atomic job claims:** use PostgreSQL transactions and a lease/claim mechanism, such as `FOR UPDATE SKIP LOCKED`, so two workers do not claim the same job simultaneously.
3. **One active pagination job per repository:** avoid two workers advancing the same repository cursor concurrently.
4. **Atomic progress:** issue upserts and cursor advancement must succeed together, or neither should be considered committed.
5. **Bounded concurrency:** increase worker count only after measuring successful throughput, response latency, errors, and rate-limit behavior.

Parallelism helps process different repositories concurrently. It does not make an oversized GraphQL query safe, nor does it increase the API budget.

## 4. GraphQL batching and recovery

The current Python v6 script is a **benchmark and import prototype**, not the production Deep Diver. Its `--max-pages-per-repo` and JSON checkpoint behavior should not be repurposed as the production queue design.

Observed behavior so far:

- GraphQL alias batches of 10 repositories can intermittently fail with `RESOURCE_LIMITS_EXCEEDED`.
- Splitting a failed batch to 5 repositories has succeeded in recent tests. A successful batch of 5 pages returned about 500 issues when each page returned 100 issues.
- The importer can return to the configured batch size of 10 after a smaller retry succeeds, but some batches still need fallback.
- `RESOURCE_LIMITS_EXCEEDED` is different from exhausting the primary points budget. Secondary rate limits must also be handled independently.
- One repository, `GrowingGit/GitHub-Chinese-Top-Charts`, returned `NOT_FOUND` and should be marked unavailable/deferred rather than allowed to block valid repositories.

Initial runtime behavior to implement:

- Start with a configurable alias batch size of 10.
- On a resource-limit error, split the batch (10 → 5 → smaller if needed) and retry the pieces independently.
- If a one-repository request still fails, simplify the query or reduce page size; record and defer the repository if it remains unprocessable.
- After successful fallback, resume the configured batch size for subsequent work.
- On secondary rate limits, honor `Retry-After` when present; otherwise wait at least one minute and back off further if the limit persists. Pause shared scheduling when the limit is likely caused by aggregate worker traffic.
- Sanitize nested strings, including NUL bytes, immediately before database persistence.
- Upserts must be idempotent, with a stable unique key such as `(repository_id, number)`.

**Do not treat page size 100 as universally safe.** It has worked for some five-repository batches, but resource limits depend on query shape and the repositories involved. Keep page size configurable, record failures, and benchmark with a fixed sample.

## 5. Database and job state

The production implementation should keep issue data and pagination state in PostgreSQL. A sync/job table (or equivalent) should track at least:

- repository ID and job type (`initial`, `freshness`, or `deep`)
- status, attempt count, lease expiry, and next retry time
- pagination cursor and completed-page count
- `has_next_page` / traversal status
- last successful sync time, separate from the cursor
- last error class and a short error detail

A successful page operation should follow this order in one transaction:

1. Validate and sanitize the response.
2. Upsert its issue records.
3. Save the new cursor and page progress.
4. Mark the job complete or requeue it if more pages remain.
5. Commit.

If the process crashes before the transaction commits, the same page can be retried safely. Do not advance a cursor before its issue rows have persisted.

## 6. Current validation and benchmark notes

Latest database integrity check reported:

| Metric | Result |
|---|---:|
| Rows in `issues` | 71,062 |
| Distinct `(repository_id, number)` pairs | 71,062 |

That confirms there were no duplicate composite keys in the table at the time of the check. It does **not** prove that all 100 pilot repositories have been fully paginated or that the whole repository catalog is complete.

Recent v6 experiments also show:

- A first-page pilot with 50 repositories, batch size 10, and page size 100 fetched/upserted 3,092 issue records in one run.
- In a 100-repository run, one unavailable repository interrupted a batch until `NOT_FOUND` handling was added.
- In a later run using a total page cap of 3, batches of 10 repeatedly hit resource limits and retries with batches of 5 succeeded. This is evidence that increasing the page cap makes the prototype do deeper work, but it is not a reason to make the first-pass importer act as the Deep Diver.
- `rows upserted` counts upsert operations, not necessarily newly inserted rows. Use database counts to determine stored coverage.
- A run reporting zero pages on resume means the prototype selected no eligible work under its current checkpoint/cap logic. It does not by itself prove every repository is fully ingested.

Use benchmark outputs as observations, not universal performance promises. Compare configurations on the same repository sample, query shape, and pagination state. Track successful pages/issues per minute, latency, response bytes, database time, retries, and rate-limit waits.

## 7. Scope and implementation order

### Phase 1: Stabilize the data baseline

- Keep the current catalog and the existing PostgreSQL issue records.
- Verify repository coverage, unique issue keys, open/closed state, nulls, and metadata completeness.
- Keep the Python v6 importer as a test harness while the production worker design is implemented.

### Phase 2: Implement initial ingestion and the Deep Diver queue

- Initial worker fetches one page per eligible repository.
- Repositories with `hasNextPage = true` enter the Deep Diver queue.
- Deep Diver processes all queued cursors without priority scores and requeues each repository until its traversal ends.
- Use durable PostgreSQL job state rather than treating a local JSON checkpoint as the production queue.

### Phase 3: Add shared rate control and parallel workers

- Coordinate primary points, secondary backoff, resource-limit retries, and concurrency centrally.
- Test with a small fixed repository set, including a repository that triggers a resource limit.
- Verify no pages are skipped, duplicated work remains safe to retry, and one bad repository does not block unrelated work.

### Phase 4: Scale ingestion and discovery

- Increase worker concurrency gradually while measuring successful throughput and error rates.
- Run discovery in parallel and add newly discovered repositories to initial ingestion.
- Continue until all eligible repositories and their in-scope open issues have been traversed, subject to GitHub availability and API limits.

### Phase 5: Analyze the collected dataset

Only after broad ingestion is established, use the collected data to investigate issue freshness, repository activity, languages, labels, contribution outcomes, and other potential signals. Design and validate data-science ranking from those results later. Do not make today's crawler selectively skip repositories based on an unvalidated score.

## 8. Completion criteria

The first full-coverage milestone is complete when:

- Every eligible repository has an initial-ingestion status.
- Every repository with more pages is represented in the Deep Diver queue until its traversal is complete.
- All in-scope open-issue pages have either been processed or explicitly recorded as unavailable/deferred with a reason.
- Checkpoints/cursors advance only with committed database writes.
- Rerunning jobs does not create duplicate issue keys or skip pages.
- Workers can run concurrently without double-claiming a repository or exceeding shared rate controls.
- Metrics distinguish recovered retries from unrecovered failures.

## 9. First full crawl (measured 17 October 2026)

The first-coverage milestone in §8 is met: every eligible repository has completed at least
one pass. Measured on production from `ingest_jobs` (rows are never deleted, so the first
`initial` job of each repository is still there) and `repositories`:

| Measure | Value |
|---|---:|
| First `initial` job queued | 2026-10-13 10:19:46 UTC |
| Last repository to finish its first pass | 2026-10-17 09:39:36 UTC |
| **Duration** | **3 days 23 h 20 min (~95 h)** |
| Repositories that completed a first pass | 284,478 |
| Eligible repositories now | 278,062 |
| Eligible repositories never synced / with no completed pass | 0 / 0 |
| Open issues stored | 7,188,572 |
| Average throughput | ~75k issues/h (~21/s), ~3,000 repositories/h |

How to read it:

- **284,478 finished, 278,062 eligible.** The import stored ~288k repositories. Since then the
  first page of each pass has overwritten the dataset snapshot with live metadata, and
  `RecomputeEligibility` has taken out ~10k that are now archived, stale, below the star
  floor, excluded or `not_found`. Completed repositories that later became ineligible still
  count in the first figure.
- **The duration is set by the long tail.** It ends when the *last* repository finished, which
  is a repository with many pages working through the Deep Diver. It does not mean the
  crawl ran at a constant rate.
- **Job counts at the time:** `initial` done 432,255, failed 2,656, pending 318; `deep` done
  28,852, pending 31. `initial` done exceeds the repository count because freshness passes
  are also queued as `initial` (§ Implementation status), so ~148k freshness passes had
  already run. ~28.9k passes needed more than one page. The 2,656 failed `initial` jobs are
  `not_found` repositories plus any that exhausted `ingest.max_attempts`.

The queries, for re-measuring:

```sql
-- start, completion span, and how many repositories finished a first pass
WITH first_start AS (
  SELECT repository_id, min(created_at) AS started
  FROM ingest_jobs WHERE job_type = 'initial' GROUP BY repository_id
), first_done AS (
  SELECT j.repository_id, min(j.updated_at) AS finished
  FROM ingest_jobs j JOIN first_start s USING (repository_id)
  WHERE j.status = 'done' AND j.has_next_page = false AND j.created_at >= s.started
  GROUP BY j.repository_id
)
SELECT count(*), min(s.started), max(d.finished), max(d.finished) - min(s.started)
FROM first_done d JOIN first_start s USING (repository_id);

-- finished when both are 0
SELECT count(*) FILTER (WHERE last_synced_at IS NULL),
       count(*) FILTER (WHERE issues_synced_through IS NULL)
FROM repositories WHERE is_eligible;
```

## Decisions for this phase

- **Fetch broadly first; analyze later.**
- **No repository priority scores yet.** Use fair/ FIFO processing to cover the catalog.
- **Initial/Freshness worker fetches the first page only.**
- **Deep Diver owns subsequent pagination.**
- **Parallel workers are allowed, with shared rate limits and atomic claims.**
- **PostgreSQL is the durable queue/state store for now.** Redis is not required unless measurement later shows PostgreSQL coordination is a bottleneck.
