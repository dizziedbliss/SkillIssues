---
title: Difficulty ratings
slug: /docs/difficulty-ratings
section: Guides
summary: How an issue gets a difficulty rating from its labels, and why issues with no rating are never assumed to be easy.
draft: false
updated: 2026-10-20
---

SkillIssues doesn't judge how hard an issue is. It reads the labels that maintainers have put on it. If a maintainer has marked an issue as a good first issue, it shows as Beginner. If nobody has said anything about difficulty, the issue is Unranked.

## The four ratings

| Rating | Meaning |
|---|---|
| Beginner | A maintainer labelled the issue as easy, or as suitable for newcomers. |
| Intermediate | A maintainer labelled the issue as medium difficulty, or as open to contributions. |
| Advanced | A maintainer labelled the issue as hard, large or specialised. |
| Unranked | None of the issue's labels says anything about difficulty. |

## Unranked is not easy

An issue with no difficulty label is Unranked, never Beginner. Not knowing how hard an issue is isn't the same as it being easy, and sending a newcomer to a hard issue because it lacked a label would be worse than not rating it. Unranked issues have no difficulty score and sort after rated ones when you sort by easiest.

SkillIssues reads only labels. It doesn't read the issue text or count comments to guess difficulty.

## Which labels count

Labels are compared without regard to capital letters or surrounding spaces.

### Beginner

These exact labels:

`good first issue`, `beginner-friendly`, `easy`, `starter`, `starter-bug`, `first-timers-only`, `newbie`, `up-for-grabs`, `low-hanging-fruit`, `bitesize`, `trivial`, `easy-fix`, `good-for-beginner`, `good-first-pr`, `documentation`, `docs`, `typo`

Labels that start with:

`difficulty/easy`, `difficulty/beginner`, `size/small`, `size/xs`, `effort/low`, `exp/beginner`

### Intermediate

These exact labels:

`help wanted`, `contributions-welcome`, `seeking-contributors`, `medium`, `intermediate`, `size/m`, `effort/medium`, `difficulty/medium`, `enhancement`, `feature`, `refactoring`, `unit-test`

Labels that start with:

`difficulty/medium`, `size/m`, `effort/medium`, `exp/intermediate`

### Advanced

These exact labels:

`hard`, `advanced`, `complex`, `size/l`, `size/xl`, `difficulty/hard`, `breaking-change`, `architecture`, `security`, `performance`, `optimization`, `critical`, `high-priority`, `blocker`, `p1`

Labels that start with:

`difficulty/hard`, `difficulty/expert`, `size/l`, `size/xl`, `effort/high`, `exp/expert`

## When an issue has labels from more than one group

The easiest match wins. The labels are checked for Beginner first, then Intermediate, then Advanced. An issue labelled both `good first issue` and `hard` shows as Beginner.

## Things to know

- Some labels in the lists are not difficulty labels in the strict sense. `documentation`, `docs` and `typo` count as Beginner because such changes are usually small. `enhancement` and `feature` count as Intermediate, and `security` and `performance` as Advanced. A maintainer using them for another purpose can produce a rating that doesn't fit the issue.
- Many projects label difficulty in ways SkillIssues doesn't recognise, so their issues stay Unranked even though they are easy or hard. Unranked means "unknown".
- Ratings follow the labels. If a maintainer changes a label on GitHub, the rating follows once the change reaches SkillIssues, usually within about two hours.
- A rating is a hint, not a promise. Read the issue before you start.

## For maintainers

If you want your newcomers to find the right issues, label them. See [For maintainers](/maintainers).
