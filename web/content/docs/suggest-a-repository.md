---
title: Suggest a repository
slug: /docs/suggest-a-repository
section: Community
summary: How to find out whether a repository is listed, why it might not be, and what to do about it.
draft: false
updated: 2026-10-20
---

There is no form to submit a repository, because there is nothing to sign up for. A public repository is listed automatically once it qualifies. The quickest way to get a clear answer is to look it up.

## Check whether it is listed

The home page has a repository lookup. You can also ask the [public API](/docs/public-api) directly:

```bash
curl 'https://skillissues.dizziedbliss.lol/api/repositories/lookup?name=owner/repo'
```

If the repository is listed, the answer says so and tells you how many open issues it has. If it is stored but not listed, the answer includes the reason.

## Why a repository might not be listed

A repository is listed when it passes all of these checks:

- It is public.
- It has at least 25 stars.
- It was pushed to within the last two years (730 days).
- GitHub reports a main language for it.
- It is not archived and is not a fork.
- It has, or has had, issues.

The lookup names the reason when one of these fails:

| Reason | Meaning |
|---|---|
| `below_star_floor` | Fewer than 25 stars. |
| `stale` | Not pushed to within two years, or the push date isn't known. |
| `no_primary_language` | GitHub reports no main language. |
| `archived` | The repository is archived. |
| `is_fork` | The repository is a fork. |
| `no_issues` | The repository has never had an issue. |
| `excluded` | The repository is on a short list of projects that are never crawled, such as an issue tracker used as a database. |

The star floor is not a measure of quality. It exists because anyone can open an issue on a throwaway repository to waste the crawler's limited GitHub quota.

These thresholds can change, and tightening them is applied within the hour. They are the current defaults.

## If it qualifies but isn't there

New repositories are found in two ways: a monthly sweep of GitHub's repository search, and the public GitHub event stream, where a repository with fresh issue activity is checked and listed if it qualifies. A repository that qualifies today may therefore take a little while to appear.

If it has been a while, or the lookup says it is listed but you don't see its issues, [open an issue](https://github.com/dizziedbliss/SkillIssues/issues/new) with the repository's name (`owner/repo`) and what the lookup returned.

## If you want a repository removed

Maintainers can ask for their repository to be excluded. See [For maintainers](/maintainers).
