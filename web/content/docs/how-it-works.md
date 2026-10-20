---
title: How SkillIssues works
slug: /docs/how-it-works
section: Resources
summary: Where the issues come from, how they stay current, and how they are ordered and rated.
draft: false
updated: 2026-10-20
---

SkillIssues gathers open issues from active GitHub projects in one place and keeps them current, so you can find one you can solve.

## Where the issues come from

We list repositories that are public, active and have a minimum number of stars (25 today). The star floor isn't a measure of quality. It keeps anyone from flooding the catalog with throwaway projects that would use up the crawler's limited GitHub quota. The full rules are in [For maintainers](/docs/for-maintainers).

The catalog started from a public dataset of repository metadata (see [Licenses and attributions](/docs/licenses-and-attributions)). Every listed repository is then read directly from GitHub's API, starting with its most recently updated open issues.

New repositories are found in two ways:

- a monthly sweep of GitHub's repository search;
- the public GitHub event stream, where any repository with fresh issue activity is checked and listed if it qualifies.

We collect open issues only. We don't collect every comment, reaction or closed issue.

## How it stays current

Every hour we read GH Archive, the public record of GitHub events. An issue opened, edited or closed on GitHub shows the change here within about two hours.

A slower pass re-reads every repository directly from GitHub each month, to repair anything the event stream missed. If the event stream falls behind, the daily direct pass resumes by itself until it catches up.

We only record an issue as closed when GitHub says it closed. An issue that simply isn't seen in a pass is not treated as closed. Issues you saved are checked directly, more often.

A closed issue is removed after about a week, unless someone saved it.

## How issues are ordered

By default the catalog is shuffled. Every repository gets one slot, showing its easiest open issue, and the order of repositories is redrawn each day. A project with ten thousand issues gets the same single slot as one with three. The goal is the whole backlog, not the biggest projects. In between redraws the order is the same for everyone.

You can sort by easiest or newest instead, and filter by labels, languages, the technologies a repository uses, its topics, difficulty and how much discussion an issue has. If you're signed in, your preferred languages come first. See [Finding issues](/docs/finding-issues).

## How difficulty is rated

Difficulty comes from the labels maintainers put on issues, like "good first issue" or "difficulty/hard". An issue with no such label is unrated. We never guess that an unlabelled issue is easy. See [Difficulty ratings](/docs/difficulty-ratings).

## What we don't do

- We don't rank repositories by popularity or "quality". There is no priority queue for big projects.
- We don't hold your GitHub token. See [Security](/docs/security).
- We don't read issue descriptions on the server for display. Your browser or the desktop app reads them from GitHub. If GitHub refuses because of your own rate limit, the server serves a cached copy as a fallback.
