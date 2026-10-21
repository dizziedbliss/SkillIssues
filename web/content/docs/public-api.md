---
title: Public API
slug: /docs/public-api
section: Guides
summary: The read-only endpoints anyone can call: issues, facets, stats, repository lookup and public themes.
draft: false
updated: 2026-10-21
---

SkillIssues has a public, read-only HTTP API. It is the same API the website uses. You don't need a key or an account for the endpoints on this page.

## Basics

**Base URL.** The API is served under `/api` on the SkillIssues site, for example `https://skillissues.dizziedbliss.lol/api/issues`. The examples below use a shell variable:

```bash
API=https://skillissues.dizziedbliss.lol/api
```

**Format.** Responses are JSON. An error is `{"error": "message"}` with a status code that says what went wrong:

| Status | Meaning |
|---|---|
| 400 | A parameter is malformed, or a combination isn't allowed. The message says which. |
| 404 | The thing doesn't exist, or you can't see it. |
| 429 | You've made too many requests. See Rate limits. |
| 503 | The catalog was busy and the query was cancelled. Try again shortly. |

**Browsers.** Calls from another website's JavaScript are refused by the browser, except for `GET /stats`, which any site may read. Scripts, servers and `curl` aren't affected.

**Caching.** Responses to signed-out callers can be cached. They carry `Cache-Control` and an `ETag`, and a request with `If-None-Match` gets a `304` when nothing changed. Please use them.

## Rate limits

Requests are limited per client address when you're not signed in, over a fixed window that starts at your first request. The defaults are 180 requests per hour. The limits are live settings, so they can change.

Every response carries:

| Header | Meaning |
|---|---|
| `X-RateLimit-Limit` | The number of requests allowed in the window. |
| `X-RateLimit-Remaining` | How many are left. |
| `X-RateLimit-Reset` | Seconds until the window resets. |

When you're past the limit you get a `429` and a `Retry-After` header with the seconds to wait. Respect it.

Be considerate. Page with a cursor, cache what you get, and don't poll faster than the data changes. The catalog is redrawn daily and changes from GitHub arrive within about two hours.

## GET /api/issues

Browse or search the catalog.

```bash
curl "$API/issues?cursor=&difficulty=beginner&language=Go&limit=10"
```

### Parameters

| Parameter | Meaning |
|---|---|
| `search` | A search term. See [Finding issues](/docs/finding-issues) for how search works. |
| `category` | With `search`: `repository`, `title` or `language` returns that kind of match alone, paged on its own with `page`. |
| `sort` | `foryou`, `easiest`, `newest` or `shuffle`. Browsing, and `easiest` or `newest` on a search (otherwise a search is in match order). See Sorting. |
| `difficulty` | `BEGINNER`, `INTERMEDIATE`, `ADVANCED` or `UNCLASSIFIED`, in any case. `UNCLASSIFIED` means unranked. |
| `language` | The repository's main language. |
| `tech` | Any language GitHub reports for the repository. |
| `topics` | The repository's GitHub topics. |
| `labels` | The issue's labels, not case sensitive. |
| `repo` | A repository as `owner/name`, not case sensitive. |
| `unassigned` | `true` leaves out issues someone is assigned to on GitHub. An issue the catalog hasn't re-read since it started recording assignees counts as unassigned. |
| `owner` | Everything from one user or organisation (the part of `owner/name` before the slash), not case sensitive. For you still shows one issue per repository; sort by `easiest` or `newest` to see them all. |
| `comments_min` | The fewest comments, inclusive. A whole number, 0 or more. |
| `comments_max` | The most comments, inclusive. `0` means no comments yet. |
| `limit` | Issues per page. The default is 20 for browsing and 50 for a search. Anything over 100 is lowered to 100. |
| `page` | A page number, starting at 1. Over 200 is lowered to 200. |
| `cursor` | Switches to cursor paging. See Paging. |

**Filters.** Each list filter (`difficulty`, `language`, `tech`, `topics`, `labels`) takes comma-separated values. Within one filter any value matches. Across filters every one must match. A filter can name up to 20 values, and more than that is a `400`. Valid values for `labels`, `topics`, `tech` and `language` come from `GET /api/issues/facets`.

### Sorting

| Sort | Order |
|---|---|
| `foryou` | The default for cursor paging. Each repository's best open issue, walked from a point seeded per caller per day, each chunk ordered by a weighted random draw on fit (signed in), how doable it looks and how much it needs someone. Issues carry `for_you`, the reasons, most telling first. Signed out, one draw a day for everyone. |
| `shuffle` | Each repository's best open issue, with repositories in a random order that is redrawn daily and is the same for everyone in between. Kept for existing callers. |
| `easiest` | By difficulty score ascending (unranked last), then most recently updated, then id. The default for page numbers. |
| `newest` | Most recently opened first. |

### Paging

There are two modes. Which one you get depends on whether `cursor` is in the request.

**Page numbers** (no `cursor`). The response is a JSON array of issues. Use `page` to move through it. Deep pages cost more. Past page 1, page numbers work with `sort=easiest` only, so `?page=2&sort=newest`, `?page=2&sort=foryou` and `?page=2&sort=shuffle` are `400`s. A search is always paged by `page`.

**Cursor paging** (with `cursor`). The response is an object:

```json
{
  "issues": [ { "...": "..." } ],
  "next_cursor": "opaque-token-or-null"
}
```

Send an empty `cursor` (`?cursor=`) for the first page. For each next page, send the previous response's `next_cursor`. Stop when `next_cursor` is `null`. Every page costs the same however deep you go.

A few rules for cursors:

- The token is opaque. Don't parse it or build one.
- Keep your filters (`difficulty`, `language`, `limit` and the rest) the same through one walk.
- A cursor can't be combined with `search`. That's a `400`, and so is a cursor the server didn't issue.

**A short page is not the end.** Only a `null` `next_cursor` means the end. The server may stop looking after a couple of seconds and return a short page, or even an empty one, together with a non-null `next_cursor`. This happens most with `foryou` and `shuffle` when your filters match few repositories. Keep following `next_cursor` until it's `null`. If you want to give up sooner, set your own limit on how many empty pages in a row you'll read. The website reads up to five before it concludes that nothing matches.

### An issue

```json
{
  "id": 123456,
  "github_id": 2000000001,
  "number": 42,
  "title": "Handle empty config files",
  "url": "https://github.com/owner/repo/issues/42",
  "state": "OPEN",
  "updated_at": "2026-10-19T08:30:00Z",
  "created_at": "2026-10-01T12:00:00Z",
  "difficulty": "BEGINNER",
  "difficulty_score": 2,
  "required_skills": ["Go"],
  "technologies": ["Go"],
  "learning_tags": ["good first issue", "bug"],
  "labels": ["good first issue", "bug"],
  "tech": ["Go", "Shell"],
  "assignees": 0,
  "contributing_url": "https://github.com/owner/repo/blob/main/CONTRIBUTING.md",
  "claimed": false,
  "repository_health": { "sampled": 20, "outside": 7, "median_merge_hours": 41, "welcoming": true, "slow": false },
  "comments": 3,
  "author": "octocat",
  "repository": "owner/repo",
  "repository_id": 4321,
  "repository_url": "https://github.com/owner/repo",
  "repository_description": "A small tool.",
  "language": "Go",
  "stars": 512,
  "license": "MIT"
}
```

| Field | Notes |
|---|---|
| `id` | SkillIssues' own id for the issue. Use it with `GET /api/issues/{id}`. |
| `github_id`, `number`, `url` | The issue's identity on GitHub. |
| `difficulty` | `BEGINNER`, `INTERMEDIATE`, `ADVANCED` or `UNCLASSIFIED`. |
| `difficulty_score` | A number, or `null` for an unranked issue. Never `0` or a default. |
| `updated_at` | GitHub's own last-updated time, not when we fetched it. |
| `created_at` | When the issue was opened on GitHub. Empty when unknown. |
| `labels` | The issue's labels exactly as GitHub has them. |
| `repository_id` | SkillIssues' own id for the repository, as a report names it. |
| `assignees` | How many people are assigned on GitHub, or `null` if the catalog hasn't read it yet. |
| `contributing_url` | The repository's contributing guide on GitHub (its own, or its owner's default), or `null` when it has none or the catalog hasn't read it yet. Always a `https://github.com/` page. |
| `claimed` | `true` when someone on SkillIssues claimed it. Who claimed it is never shown. |
| `repository_health` | The repository's last merged pull requests: `sampled` (how many were read), `outside` (how many came from people outside the project), `median_merge_hours`, and the flags `welcoming` and `slow`. `null` until enough were read. |
| `tech` | The repository's three largest languages, largest first, by GitHub's names. Empty when unknown. The `tech` filter takes the same names in any case. |
| `match_category` | Search results only: `Repository`, `Title` or `Language`. |

`recommendation_score` and `why_recommended` appear only on personalised results, which need an account, so you won't see them on public calls.

A signed-out response is the same for everyone. A signed-in caller's results are ordered with their preferred languages first and are never cached.

## GET /api/issues/{id}

One issue, as above. `404` means it doesn't exist.

## GET /api/issues/facets

The values for the filter dropdowns, most common first, each with the number of open issues that have it.

```bash
curl "$API/issues/facets"
```

```json
{
  "labels":    [ { "value": "bug", "issues": 120345 } ],
  "topics":    [ { "value": "cli", "issues": 8123 } ],
  "tech":      [ { "value": "Shell", "issues": 90210 } ],
  "languages": [ { "value": "Go", "issues": 54321 } ],
  "refreshed_at": "2026-10-20T02:00:00Z"
}
```

The lists are recomputed once a day, and each is cut off at the most common few hundred values, so a rare value may not appear. `refreshed_at` is `null` before the first computation.

## GET /api/stats

Live counts for the catalog. Any website may read this one from a browser.

```bash
curl "$API/stats"
```

```json
{
  "open_issues": 7000000,
  "closed_since": 120000,
  "tracking_since": "2026-10-15T00:00:00Z",
  "arrived_today": 5400,
  "closed_today": 4900,
  "issues_indexed": 7900000,
  "repositories_indexed": 284000,
  "developers": 12,
  "activity": [
    { "at": "2026-10-20T09:00:00Z", "repository": "owner/repo", "arrived": 3, "closed": 1 }
  ],
  "updated_at": "2026-10-20T09:00:05Z"
}
```

| Field | Meaning |
|---|---|
| `open_issues` | Open issues in the catalog. |
| `closed_since` | Issues seen closing since `tracking_since`. |
| `tracking_since` | When counting closures began. `null` if it hasn't. |
| `arrived_today`, `closed_today` | Issues that arrived or closed today, counting the day in UTC. |
| `issues_indexed` | Issues stored, open and closed. |
| `repositories_indexed` | Repositories in the catalog. |
| `developers` | Accounts signed up on SkillIssues. |
| `activity` | Recent batches that brought issues in or saw them close, newest first. |
| `updated_at` | When the counts were last updated. `null` before the first update. |

The numbers in the examples on this page are made up. The response is cached for a few seconds, so polling more often than every ten seconds gains you nothing.

## GET /api/repositories/lookup

Whether a repository is in the catalog, and how many open issues it has. Pass `name` as `owner/repo`. A full `https://github.com/owner/repo` address works too.

```bash
curl "$API/repositories/lookup?name=owner/repo"
```

A listed repository:

```json
{
  "found": true,
  "full_name": "owner/repo",
  "url": "https://github.com/owner/repo",
  "eligible": true,
  "stars": 512,
  "open_issues": 14
}
```

A repository we don't have:

```json
{ "found": false, "eligible": false, "stars": 0, "open_issues": 0 }
```

A repository we know about but don't list has `found: true`, `eligible: false` and a `reason`, such as `below_star_floor`, `stale`, `no_primary_language`, `archived`, `is_fork`, `no_issues` or `excluded`. See [Suggest a repository](/docs/suggest-a-repository). A name that isn't `owner/repo` is a `400`.

## GET /api/themes/{id}

A public [theme](/docs/themes), by its id. Themes that aren't public look the same as missing ones. You can read your own private themes when you're signed in.

```bash
curl "$API/themes/00000000-0000-4000-8000-000000000000"
```

```json
{
  "id": "00000000-0000-4000-8000-000000000000",
  "name": "Midnight Red",
  "toml": "[colors]\naccent = \"#ff2b2b\"\n",
  "css": "",
  "public": true,
  "author": "octocat",
  "mine": false,
  "created_at": "2026-10-19T10:00:00Z",
  "updated_at": "2026-10-20T10:00:00Z"
}
```

The response isn't cached, because the author can change the theme at any time. Treat `toml` and `css` as untrusted input and validate them before you use them. See [Themes](/docs/themes) for the rules. A theme id that isn't a valid UUID, or doesn't exist, is a `404`.

## GET /api/themes/public

The community listing: public themes, one page at a time. No sign-in needed.

| Parameter | Meaning |
|---|---|
| `sort` | `popular` (the default: most used first) or `newest`. |
| `search` | Only themes whose name contains this. |
| `page` | A page number, starting at 1. |

```json
{ "themes": [{ "id": "…", "name": "Midnight Red", "toml": "…", "css": "", "public": true, "author": "octocat", "mine": false, "created_at": "…", "updated_at": "…", "uses": 12 }], "page": 1, "more": true }
```

`uses` is how many accounts have the theme applied. `more` says there may be another page. Themes several people have reported are left out until they're reviewed.

## POST /api/themes/validate

Checks a theme as saving would, without storing it. No sign-in needed. See [Themes](/docs/themes).

## Not covered here

Everything that needs an account (saved issues, your profile, your themes, recommendations, reporting) is outside this page. Those routes are for the SkillIssues apps. Routes under `/api/admin` and `/api/metrics` aren't reachable from the internet.
