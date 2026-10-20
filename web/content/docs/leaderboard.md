---
title: Leaderboard
slug: /leaderboard
section: Community
summary: Levels and a community leaderboard are coming soon, built from public GitHub data without storing your token.
draft: false
updated: 2026-10-20
---

The leaderboard doesn't exist yet. This page explains what is planned and why it isn't here already.

## What happened to XP and levels

The first version of SkillIssues awarded XP, levels and badges for merged pull requests. To check that a pull request was really yours, it stored your GitHub token on the server. We removed that: the server now holds no user GitHub tokens at all, and the XP system went with it. See [Security](/security).

## The plan

Levels and a community leaderboard for the people closing issues across open source, rebuilt without stored tokens:

- A merged pull request counts the moment GitHub shows it, checked against public GitHub data.
- The check happens on the server using public information, with no token from you.
- The work that needs your own GitHub credentials stays in the [desktop app](/download), on your machine.

We haven't set a date, and the details of levels and badges aren't decided. Nothing on this page is a promise about how points will be counted.

## Until then

Find an issue and fix it. A merged pull request is a public record on GitHub whether or not a leaderboard exists.

- [Getting started](/docs/getting-started)
- [Finding issues](/docs/finding-issues)
