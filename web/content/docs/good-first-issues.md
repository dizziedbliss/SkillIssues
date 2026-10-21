---
title: Is this a good first issue?
slug: /docs/good-first-issues
section: Guides
summary: How to tell an issue you can finish from one that will stall, what the checklist on each issue page looks at, and what to do when it warns you.
draft: false
updated: 2026-10-21
---

A good first issue is one you can understand, finish and get merged. A label helps, but it isn't enough on its own. Before you start, spend five minutes checking the issue. It saves you a week of work that never lands.

## Signs of a good one

- **The maintainers marked it as a place to start.** Labels such as "good first issue", "beginner", "easy", "first-timers-only" or "help wanted" mean they expect someone new to take it, and they'll help.
- **It says what's wrong and what "done" looks like.** Steps to reproduce, the error, the file or function involved, or a sketch of the change.
- **Nobody is assigned, and no pull request is open for it.** Otherwise someone is already on it.
- **It's recent, or the project is.** A maintainer who replied this month will probably review your pull request this month.
- **The discussion is short, or settled.** You can read it in a few minutes and know what to do.
- **The project has a contributing guide.** It tells you how to build it, test it and send a change.

## Warning signs

- **No description, or one line.** Ask on the issue what's wanted before you write code.
- **Someone is assigned.** Explore leaves these out unless you ask for them, and the checklist checks again on GitHub when you open an issue. Ask them, politely, whether they're still working on it. If there's no answer after a week, ask a maintainer whether you can take it.
- **A pull request already points at it.** Read it. If it's open and active, pick another issue. If it was abandoned months ago, ask whether you can finish it.
- **Nothing has happened for six months.** The maintainers may have moved on, or the issue may no longer matter. Check the repository's recent commits and merged pull requests before you start.
- **A long, heated discussion.** People don't agree on what the fix should be. That's not a beginner's problem to solve.
- **It needs a decision, not code.** "Should we support X?" is a question for the maintainers.
- **The difficulty is unranked.** Nobody has said how hard it is. Read the code it touches before you commit. See [Difficulty ratings](/docs/difficulty-ratings).

## The checklist on each issue page

Every issue page has a **Good place to start?** box. It reads the issue as GitHub has it right now, in your browser, and marks each point **+** (fine), **!** (look before you start) or **?** (it couldn't tell, usually because GitHub limited how often your browser can ask).

| Point | Fine when |
|---|---|
| Marked as a place to start | A label such as "good first issue", "beginner", "easy", "starter", "first-timers-only", "newcomer" or "low-hanging fruit", or a Beginner rating. |
| A real description | At least 150 characters, not counting an issue template's hidden comments. |
| Nobody assigned | GitHub shows no assignee. |
| No open pull request | No open pull request references the issue on its timeline. |
| Active recently | Updated in the last 180 days. |
| A short discussion | 10 comments or fewer. |

The box says "Looks like a good place to start" only when every point is fine. A **!** isn't a "no". It's a thing to check, and the warning says how.

## When you've picked one

1. **Say you're working on it.** A comment such as "I'd like to work on this. I plan to change X in Y." lets the maintainers stop you early if your plan is wrong.
2. **Read the contributing guide**, and build and test the project before you change anything.
3. **Keep the change small.** Fix the issue, and nothing else. A small pull request is reviewed sooner.
4. **Link the issue** in your pull request ("Fixes #42"), so it closes when your change is merged.

See [Getting started](/docs/getting-started) for the whole route from the catalog to your first pull request.

## If an issue shouldn't be here

If an issue is spam, already fixed, not really an issue, or rated wrongly, report it from its page. See [Reporting](/docs/reporting).
