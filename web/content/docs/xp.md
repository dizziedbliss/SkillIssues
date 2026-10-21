---
title: How XP works
slug: /docs/xp
section: Guides
summary: What earns XP, how much, how levels, streaks and badges follow from it, and what earns nothing.
draft: false
updated: 2026-10-21
---

XP is SkillIssues' record of the work you got merged. It counts merged pull requests to repositories in the catalog, and pays more for the issues that most need fixing. Opening pull requests earns very little, so spamming maintainers gets you nowhere.

## Where it comes from

Once you sign in, SkillIssues reads your public pull requests on GitHub, using its own GitHub credential. It never asks for or stores yours. It keeps the pull requests to repositories in the catalog and ignores the rest.

- The first read goes through your whole history, so past work counts. Merges from before you signed up earn **half** the usual XP. That gives you a real level from day one without outranking people who did the work here.
- After that, it reads again every 15 minutes while you have an open pull request that moved this week, every hour while one is open, and daily otherwise. **Check now** on your profile asks for a read straight away, every few minutes at most.

## What a merge earns

A merge earns a base amount, set by the hardest catalog issue the pull request closes. It counts as closing an issue when GitHub links them, for example with "Fixes #12" in the description.

| The pull request | Base XP |
| --- | --- |
| Closes a beginner issue | 75 |
| Closes an intermediate issue | 140 |
| Closes an advanced issue | 250 |
| Closes an issue of unknown difficulty | 100 |
| Closes no catalog issue | 50 |

On top of the base:

| Bonus | XP |
| --- | --- |
| Your first merge in that repository | +15 |
| The issue had been open six months or more when you fixed it | +50 |
| The repository runs checks and they passed | +15 |
| A maintainer approved it | +10, or +20 in a repository with no checks |
| It merged with no changes requested | +5, or +10 in a repository with no checks |

A repository without checks is never penalised: its review bonuses are bigger, so the most a merge can gain from quality is the same either way. Merge XP is capped at 400 per repository per day.

**For example:** you fix a beginner issue that had been open for a year, in a repository you haven't merged in before. It has checks, they pass, and a maintainer approves it without asking for changes. That's 75 + 15 + 50 + 15 + 10 + 5 = **170 XP**.

## Opening a pull request

Opening a pull request that closes a catalog issue earns **5 XP**. That's once per issue and at most three times a week. If the pull request is closed without being merged, the 5 XP is taken back.

## What earns nothing

- Pull requests to your own repositories, or to an organisation's repositories you're a member of.
- Pull requests to repositories that aren't in the catalog.

They still show on your profile, marked with why.

## Levels

Each level takes 100 XP more than the one before it: level 2 starts at 100 XP, level 3 at 300, level 4 at 600, level 5 at 1,000 and level 10 at 4,500. Your profile also shows a level for each language, from the XP earned in repositories whose main language it is.

## Streaks

A streak counts the weeks in a row (Monday to Sunday, UTC) in which you had at least one merge. It stays alive until the end of the week after your last merge, so you always have a full week to keep it going.

## Badges

Badges mark milestones:
- your first merge, then 5, 10, 25, 50 and 100 merges;
- merges in 3 and 10 repositories, and in 3 and 5 languages;
- a documentation fix, an advanced fix, and a fix for an issue open six months or more;
- a 4-week and a 12-week streak;
- levels 5, 10 and 20.

Each one shows the date you earned it.

## Quests

Quests are one-off goals. Each pays **50 XP**, once, and only merges since you joined count towards them:

| Quest | Done when |
| --- | --- |
| First merge here | A pull request of yours is merged in a catalog repository. |
| Called it | You claim an issue, then close it with a merged pull request. |
| Docs day | You close an issue labelled documentation. |
| Second language | You've merged in repositories of two different main languages. |
| Getting around | You've merged in three different repositories. |
| Busy month | You get three merges in one calendar month. |
| Backlog dive | You close an issue that had been open six months or more. |

## Learning paths

A learning path is five open issues for one kind of work: frontend, backend, documentation, DevOps, data or mobile. You get three for beginners, then two a step up. They come from different projects, and nobody is assigned to or has claimed any of them. Start one from **Learning path** in your profile menu.

- A step is done when a pull request of yours that closes its issue is merged.
- **Swap** replaces a step you haven't done with another issue of the same level, for example if someone else got to it first.
- Finishing all five earns **200 XP**. You can have one path at a time, and **Give up this path** ends it.

## Your public profile

Your profile is private unless you turn on **Show my profile to anyone** in Settings. Then anyone can see your level, XP, badges, languages and merged pull requests at `/app/u/<your username>`, and you can be listed on the [leaderboard](/leaderboard) once it opens. Turn it off and the page goes, at once.

## The numbers can change

These are the current values. They can be tuned as we learn what works. A change applies to XP earned after it, and levels are always worked out from your XP as it stands. Your XP history is kept as a list of what each pull request earned and why, so nothing is recalculated behind your back.
