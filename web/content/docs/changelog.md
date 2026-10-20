---
title: Changelog
slug: /docs/changelog
section: SkillIssues
summary: Where releases are published, and a summary of the notable changes made recently.
draft: false
updated: 2026-10-20
---

## Releases

Desktop builds and release notes are published on the [GitHub releases page](https://github.com/dizziedbliss/SkillIssues/releases). The website and API are not versioned that way: every change that is merged to the main branch passes its automated checks and is deployed.

The full, detailed history is in [CHANGELOG.md](https://github.com/dizziedbliss/SkillIssues/blob/main/CHANGELOG.md) in the repository. This page summarises the parts you are most likely to notice.

## October 2026

### Catalog and search

- **Filters.** Filter the catalog by labels, main language, technologies a repository uses, repository topics, difficulty, and how many comments an issue has. See [Finding issues](/docs/finding-issues).
- **A fair default feed.** The default order gives every repository one slot, showing its easiest open issue, in an order that is redrawn daily. Sorting by easiest or newest is still available.
- **Faster search.** Search had started timing out on the full catalog. Each kind of match (repository, title, language, author) is now looked up separately and the results are interleaved.
- **Live counts.** The landing page reads live catalog counts from a new public `GET /stats` endpoint.
- **Cursor paging.** The API can page through the catalog by cursor, which costs the same however deep you go. See [Public API](/docs/public-api).

### Freshness

- **Hourly updates from GH Archive.** Issues opened, edited, commented on or closed on GitHub now show up here within about two hours, without spending GitHub API quota. A slower monthly pass re-reads every repository directly to repair anything missed.
- **Discovery.** A sweep of GitHub's repository search finds new repositories that qualify, so the catalog is no longer limited to the initial dataset.
- **Issue descriptions.** The website reads issue descriptions from GitHub directly. If GitHub refuses because of your own rate limit, SkillIssues serves a cached copy as a fallback.

### Themes

- **User themes.** Restyle the signed-in app with a theme file of design tokens and an optional stylesheet. Themes sync to your account and can be shared. See [Themes](/docs/themes).

### Website

- **A new landing page and a rebuilt website**, being rolled out alongside the rebuilt desktop app.

### Accounts and security

- **No user tokens on the server.** Signing in uses a GitHub app that asks for no permissions. We read your numeric id, username and avatar, then discard the token. Anything that writes to GitHub moved into the desktop app. See [Security](/docs/security).
- **Verified releases.** Every image is built by public CI, signed, and verified on the server before it runs.
- **Encrypted secrets and backups.** Production secrets are stored encrypted, and database backups are encrypted with a key that is not on the server.
- **Cloudflare in front.** The site is set up to sit behind Cloudflare without changing how visitors are counted or limited.

### Live visualizer

- **SkillIssues Live.** A public, real-time view of data moving through the system, with anything tied to a signed-in person shown as random symbols. See [SkillIssues Live](/docs/skillissues-live).

### Removed

- **XP, levels and badges.** The first version awarded these from pull requests verified with a stored user token. They were removed with the no-stored-tokens redesign. The plan for a replacement is on the [Leaderboard](/docs/leaderboard) page.

## Version 1.0.0, 13 October 2026

The first production release. The site went live on a real server with automated, gated deploys, and the web and desktop apps began sharing one set of code for talking to the API and drawing the interface.
