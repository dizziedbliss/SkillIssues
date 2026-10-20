---
title: Report a bug
slug: /docs/report-a-bug
section: Support
summary: How to write a bug report that can be fixed.
draft: false
updated: 2026-10-21
---

Bugs are reported as GitHub issues, each kind on its own form, so the report asks for what we need:

- [Something broken on the website or in the API](https://github.com/dizziedbliss/SkillIssues/issues/new?template=bug_report.yml)
- [Something broken in the desktop app](https://github.com/dizziedbliss/SkillIssues/issues/new?template=desktop.yml)
- [Something hard or impossible to use with your setup](https://github.com/dizziedbliss/SkillIssues/issues/new?template=accessibility.yml)
- [A wrong difficulty, or an issue that differs from GitHub](https://github.com/dizziedbliss/SkillIssues/issues/new?template=data_quality.yml)
- [A theme that won't apply, or a built-in that looks wrong](https://github.com/dizziedbliss/SkillIssues/issues/new?template=theme.yml)
- [A page or doc that says something untrue](https://github.com/dizziedbliss/SkillIssues/issues/new?template=documentation.yml)

If the problem is a security vulnerability, don't use a public issue. See [Security](/security).

## What to include

A report we can act on has:

1. **What you expected to happen.**
2. **What happened instead.** Include the exact error message if there is one.
3. **Steps to reproduce.** The shorter and more exact, the better. For a catalog problem, include the filters, sort and search term you used.
4. **Where it happened.** The website or the desktop app. Your browser, or your operating system and the desktop app's version.
5. **When it happened,** if it was a one-off. Something that fails once and then works is a different bug from something that always fails.
6. **A link,** if it is about one issue or repository. For example the issue's GitHub link, or the `owner/repo` name.

Remove anything private, such as tokens, cookies and personal details, from logs and screenshots before you post them.

## Before you report

- **Data looks out of date.** Changes on GitHub usually take up to about two hours to show. Wait before reporting. See [How it works](/how-it-works).
- **An issue says "unranked".** That isn't a bug. It means the maintainers haven't labelled its difficulty. See [Difficulty ratings](/docs/difficulty-ratings).
- **A repository is missing.** Check it with the lookup first. See [Suggest a repository](/docs/suggest-a-repository).
- **You got "too many requests".** The API limits how often one caller can ask. Wait for the time in the `Retry-After` header. See [Public API](/docs/public-api).
- **The catalog was busy.** A request that takes too long is cancelled with a message to try again shortly. Retrying usually works.

## After you report

Maintainers read new issues as they can. Add detail if you're asked for it, and tell us if you find the cause or the problem goes away.
