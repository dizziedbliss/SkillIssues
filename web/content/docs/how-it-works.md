---
title: How SkillIssues works
slug: /how-it-works
section: Resources
summary: Where the issues come from, how they stay current, and how they are ordered and rated.
draft: false
updated: 2026-10-21
---

SkillIssues gathers open issues from active GitHub projects in one place and keeps them current, so you can find one you can solve.

## Where the issues come from

We list repositories that are public, active and have a minimum number of stars (25 today). The star floor isn't a measure of quality. It keeps anyone from flooding the catalog with throwaway projects that would use up the crawler's limited GitHub quota. The full rules are in [For maintainers](/maintainers).

The catalog started from a public dataset of repository metadata (see [Licenses and attributions](/licenses)). Every listed repository is then read directly from GitHub's API, starting with its most recently updated open issues.

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

By default you see **For you**. Every repository gets one slot, showing its best open issue for your filters, so a project with ten thousand issues gets the same single slot as one with three. Each issue is scored on how well it fits you, how likely it is to get merged, and how long it has waited with few people looking, and each page is a weighted random draw on that score: good matches come first more often, but different people see different issues. The goal is the whole backlog, not the biggest projects.

You can sort by easiest or newest instead, and filter by labels, languages, the technologies a repository uses, its topics, difficulty and how much discussion an issue has. Signed in, Explore starts with your profile's languages and level. See [Finding issues](/docs/finding-issues).

## How difficulty is rated

Difficulty comes from the labels maintainers put on issues, like "good first issue" or "difficulty/hard". An issue with no such label is unranked. We never guess that an unlabelled issue is easy. See [Difficulty ratings](/docs/difficulty-ratings).

## What we don't do

- We don't rank repositories by popularity or "quality". There is no priority queue for big projects.
- We don't hold your GitHub token. See [Security](/security).
- We don't read issue descriptions on the server for display. Your browser or the desktop app reads them from GitHub. If GitHub refuses because of your own rate limit, the server serves a cached copy as a fallback.
