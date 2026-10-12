# SkillIssues Ingestion Architecture

**Updated:** 11 October 2026  
**Purpose:** Build a broad, reliable issue dataset first. Data-science ranking and repository prioritization come later, after there is enough data to analyze.

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

## Decisions for this phase

- **Fetch broadly first; analyze later.**
- **No repository priority scores yet.** Use fair/ FIFO processing to cover the catalog.
- **Initial/Freshness worker fetches the first page only.**
- **Deep Diver owns subsequent pagination.**
- **Parallel workers are allowed, with shared rate limits and atomic claims.**
- **PostgreSQL is the durable queue/state store for now.** Redis is not required unless measurement later shows PostgreSQL coordination is a bottleneck.
