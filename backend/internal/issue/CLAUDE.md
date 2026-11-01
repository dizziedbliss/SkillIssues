# Catalog queries (`internal/issue`)

Every query here runs against ~8M open issues inside a statement timeout. Most rules below exist
because the simpler query ran past it. Check a plan change with `EXPLAIN (ANALYZE, BUFFERS)` on real
volumes, not a dev database.

## Checks

```bash
cd backend
go test ./internal/issue/...                   # TEST_DATABASE_URL for the database tests
go test ./internal/issue/ -run TestInterleave  # one test
```

## Browsing

- **The catalog order is the index order.** (`difficulty_score`, `updated_at DESC`, `id DESC`) is exactly
  `issues_open_browse_idx` / `issues_open_difficulty_browse_idx` (00035). Change one, change the other,
  or every page sorts every open issue.
- `BrowseIssues` serves the first page and every `?cursor=` page as a run of ordered index ranges,
  **never a UNION** (Postgres sorts a UNION in full). `?page` > 1 is the OFFSET query.
- Browse queries use `pgx.QueryExecModeExec`, so they never run from a cached generic plan.
- A language under `api.issues.rare_language_threshold` open issues is sorted instead of walked: the
  planner can't tell a rare language from a common one.
- Newest with a difficulty filter reads each difficulty's range on its own and merges them
  (`newestByDifficulty`, rated ones via `issues_open_rated_newest_idx`, 00063).

## Filters (`filters.go`, one builder)

- Any-of within a list, all-of across filters. Labels match case-insensitively through
  `catalog_lower_labels()`, tech through the generated `repositories.tech`.
- A rare filtered page (`isRare`, under `api.issues.rare_language_threshold`) is read through the
  filters' own indexes and sorted. The count is bounded by `api.issues.rare_probe_ms`.
- **A difficulty filter is stated on `difficulty_score` alone. Never add `difficulty = ...` beside it.**
  Two conditions for one fact made the planner expect 994 rows where there were 188,970: 25 s.
- Expression statistics on `catalog_lower_labels(labels)` and a larger sample for `repositories.tech`
  (00048) stop the planner guessing 0.5% for a label (bug + Go: 7 s → 41 ms). Don't drop them.
- `?unassigned=true` (`issues.assignees`, 00053; NULL counts as unassigned) is deliberately left out of
  `hasFilters`, so it never triggers the rarity probe.
- Facet dropdown values come from `catalog_facets` (ingestd, every `ingest.catalog.facets_refresh_seconds`).
  Never count them per request.
- go-api runs with `DB_WORK_MEM_MB` (32 in prod) so label bitmaps stay exact; a lossy bitmap re-checked
  ~550k rows (8 s).

## Search (`search_categories.go`)

- Three categories, each **its own indexed query, run concurrently**: Repository (name contains the
  term; skipped under 3 characters, the trigram index can't serve it), Title (full text on title and
  labels, a bounded `api.search.sample_size` sample by stars), Language (main language or tech).
  Authors are not searched. **Never fold them into one OR query across the join**: no index serves it
  and it ran past the timeout.
- `interleave` merges them rank by rank, each issue once. Each category reads at least
  `api.search.per_category_cap` × 3 and twice the page depth (up to `sample_size`), so a shallow read
  stays a prefix of a deep one. Repository and Language take at most `api.search.per_repo_cap` per
  repository. `sort=easiest|newest` sorts the whole read; none keeps match order.
- `?category=` returns one category alone, paged up to `api.search.sample_size`.

## For you (`foryou.go`, the default `sort=foryou`)

- Walks repositories in `repositories.shuffle_key` order, one slot per repository (its best open
  issue), from a start seeded per person per UTC day (`forYouSeed`; person 0 = everyone signed out, so
  pages cache), wrapping once. Read in chunks of `api.foryou.pool_repos`, each ordered by a Gumbel-top-k
  draw on Fit × Doable × Need (`forYouScore`, weights `api.foryou.weight_*_percent`, noise
  `api.foryou.randomness_percent`).
- **Stars never enter the score.** Don't add popularity weighting.
- Drawn chunks and daily serve counts live in Redis (`cache.ForYou`), both failing open. The cursor
  carries seed, start, chunk position and offset, so every page of a walk agrees.
- The shuffled feed (`sort=shuffle`, still used by learning paths) finds labelled issues a batch of
  repositories at a time (`repository_id = ANY(batch)`), and gives up walking after
  `api.issues.shuffle_walk_max_repos` repositories or `api.issues.shuffle_walk_budget_ms`.

## Hidden and claimed issues

Reported-and-hidden targets and claimed issues are held in memory by go-api and dropped at
`Service`'s exported methods **after the cursor is taken**. Never filter them inside the catalog SQL:
it changes the index plans. A page that loses every issue still continues.

## Gotchas

- **A short page with a non-null `next_cursor` is not the end.** Only a null cursor is. Clients and
  tests must keep paging.
- Signed-in responses are ordered by the caller's languages: `private, no-store`. Only signed-out
  responses are public, with `ETag` and a shared Redis copy (`singleflight`).
