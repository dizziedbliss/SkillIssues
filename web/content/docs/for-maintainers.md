---
title: For maintainers
slug: /maintainers
section: Guides
summary: How your repository gets listed, how to help contributors find the right issues, and how to ask to be excluded.
draft: false
updated: 2026-10-20
---

If you maintain an open-source project, SkillIssues can send you contributors who picked your issue on purpose. This page explains how your repository gets listed, what you can do to make the match better, and how to opt out.

## Getting listed

There is nothing to sign up for. A public repository is listed automatically once it meets all of these:

- It has at least 25 stars.
- It was pushed to within the last two years (730 days).
- GitHub reports a main language for it.
- It isn't archived.
- It isn't a fork.
- It has, or has had, issues.

These are the current settings, and they can change. If the bar is raised, repositories that no longer meet it stop being crawled within the hour.

The star floor is not a measure of quality. Anyone can open an issue on a throwaway repository, and each repository we list costs a share of a limited GitHub quota. The floor protects that quota.

We find new repositories in two ways: a monthly sweep of GitHub's repository search, and the public GitHub event stream, where a repository with fresh issue activity is checked and listed if it qualifies.

Check whether yours is listed with the repository lookup on the home page, or see [Suggest a repository](/docs/suggest-a-repository) for the reasons a repository might not be.

### What stays current

Once your repository is listed, its stars, forks, last push and open-issue count are refreshed from GitHub each time we crawl it, so eligibility follows the live numbers. If a repository stops qualifying, for example because it is archived, we stop crawling it and remove its issues from the catalog. If it qualifies again later, it comes back.

## What we show

For each open issue, SkillIssues shows its title, labels, comment count, author's login, a link to the issue and basic details of your repository. It links to GitHub for everything else. Issue descriptions are read from GitHub by the person's browser or app, and a bounded cache serves them if GitHub refuses.

An issue that closes on GitHub is marked closed when GitHub says so, and removed from the catalog after about a week.

## Help contributors pick well

### Label difficulty

SkillIssues rates difficulty from your labels. Issues with no difficulty label are shown as unranked, and we never guess that one is easy. Labelling a few issues puts them in front of the people who want them.

Labels that work for each rating:

- **Beginner:** `good first issue`, `difficulty/easy`, `beginner-friendly`, `easy`, `first-timers-only`, `up-for-grabs`, `good-first-pr`, `size/xs`, `effort/low`.
- **Intermediate:** `difficulty/medium`, `intermediate`, `medium`, `help wanted`, `size/m`, `effort/medium`.
- **Advanced:** `difficulty/hard`, `advanced`, `hard`, `complex`, `size/l`, `size/xl`, `effort/high`.

The full list is in [Difficulty ratings](/docs/difficulty-ratings). Three things to know:

- If an issue has labels from more than one group, the easiest one wins. Don't put `good first issue` on something hard.
- A few general labels count as difficulty labels: `documentation`, `docs` and `typo` as Beginner; `enhancement` and `feature` as Intermediate; `security` and `performance` as Advanced. If you use these labels in other ways, expect some mismatches.
- Matching ignores capitals and surrounding spaces.

### Write issues a newcomer can start

- Say what's wrong, where in the code it lives, and how you'd check a fix.
- Say what you'd accept, and what you wouldn't.
- Keep the issue open and unassigned until someone is actually working on it, and close it when it's done.

### Keep a CONTRIBUTING file

Explain how to run the project and its tests, how you like changes made, and how long reviews usually take. Contributors from SkillIssues are told to read this file before they start. See the [contribution guide](/docs/contributing).

## Not listed, or want out?

- **Should be listed and isn't.** Check the reason with the lookup first, then [open an issue](https://github.com/dizziedbliss/SkillIssues/issues/new) with the repository's `owner/repo` name.
- **Want your repository excluded.** [Open an issue](https://github.com/dizziedbliss/SkillIssues/issues/new) with the `owner/repo` name and say you'd like it excluded. We keep a list of repositories that are never crawled. Once yours is on it, it is dropped from the catalog and its issues are removed, except any that a signed-in user has saved.
- **You want us to stop mentioning a particular issue.** Close it or remove its labels on GitHub, and the change follows. We don't keep issues that GitHub says are closed past a short retention period.

## Related

- [How it works](/how-it-works)
- [Difficulty ratings](/docs/difficulty-ratings)
- [Contact](/contact)
