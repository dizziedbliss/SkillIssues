---
title: About SkillIssues
slug: /about
section: SkillIssues
summary: What SkillIssues is, why it exists and how it is run.
draft: false
updated: 2026-10-20
---

Open source has millions of open issues: bugs, improvements and questions that pile up faster than any group of maintainers can work through them. Plenty of developers want to help but don't know where to start.

SkillIssues puts those issues in one place, keeps them current, and helps you find one that fits what you know. Then you go to GitHub and fix it.

## What it does

- **Gathers open issues** from public GitHub repositories that are active and have at least 25 stars.
- **Rates difficulty from maintainer labels.** An issue labelled "good first issue" is beginner. An issue with no difficulty label is unrated, and we never guess that it is easy. See [Difficulty ratings](/docs/difficulty-ratings).
- **Spreads the feed across the whole backlog.** By default every repository gets one slot, so a project with ten thousand issues doesn't crowd out one with three. See [Finding issues](/docs/finding-issues).
- **Keeps the data current.** Changes on GitHub usually show up within about two hours. See [How it works](/how-it-works).

## What it doesn't do

SkillIssues doesn't hold your GitHub token. Signing in only tells us who you are, and we throw the token away. Anything that writes to GitHub, such as forking, pushing or opening a pull request, happens on your own computer in the [desktop app](/download), with a token that never leaves your machine.

It also doesn't replace GitHub. You read the issue, discuss it and send your pull request there.

## Who runs it

SkillIssues is built in the open by [@dizziedbliss](https://github.com/dizziedbliss), for the open-source community. The code is MIT licensed and lives at [github.com/dizziedbliss/SkillIssues](https://github.com/dizziedbliss/SkillIssues). It runs on a single small server, which is why it is careful about how much it asks of GitHub.

## Get involved

- Solve an issue. Start with [Getting started](/docs/getting-started).
- Help build SkillIssues. See [Contribute](/contribute).
- Tell us something is wrong. See [Report a bug](/docs/report-a-bug).
