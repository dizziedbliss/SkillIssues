---
title: Getting started
slug: /docs/getting-started
section: Resources
summary: Four steps from the catalog to your first pull request.
draft: false
updated: 2026-10-27
---

You don't need an account to browse. Open the catalog and start looking.

## 1. Find an issue

Browse the catalog, or narrow it to the languages you know and a difficulty you're comfortable with. Beginner issues are ones maintainers marked as a good place to start. Issues with no difficulty label show as unranked, which means nobody has said how hard they are.

Each issue page has a **Good place to start?** checklist; see [Is this a good first issue?](/docs/good-first-issues) for what it checks and what to do when it warns you. Read the issue and its comments on GitHub before you begin. Someone may already be on it, or the issue may have changed since you saw it here.

See [Finding issues](/docs/finding-issues) for sorts, filters and search, and [Difficulty ratings](/docs/difficulty-ratings) for what the ratings mean.

If you sign in with GitHub, you can save issues and set the languages and level you want. Signing in only tells us who you are. It gives us no access to your repositories. See [Security](/security).

Around the app:

- **Explore** is the catalog. **Search** (or `/`, or Ctrl+K) searches from anywhere.
- **Learning path** gives you five issues for one kind of work, beginner first.
- **Profile** opens a menu: your profile, saved issues, your contributions (your pull requests and your year), settings, themes (Appearance) and language.
- **Saved issues** tells you when a saved issue is closed on GitHub.

## 2. Say you're taking it

Leave a short comment on the issue saying you'd like to work on it. Maintainers often reply with pointers, and it saves two people doing the same work.

## 3. Fork, clone and fix

Fork the repository on GitHub, clone your fork, and create a branch for your change. Read the project's CONTRIBUTING file first. It says how to run the tests and how the maintainers like changes made.

The [desktop app](/download) does the fork, clone and branch for you. You can also do it yourself:

```bash
git clone https://github.com/<you>/<repository>.git
cd <repository>
git checkout -b fix-issue-123
```

Make the change, run the project's tests, and commit.

## 4. Open a pull request

Push your branch and open a pull request that links the issue. Write "Fixes #123" in the description, using the issue's number, so GitHub connects the two.

Answer review comments. Needing a few rounds is normal and doesn't mean you did something wrong.

## What next

- [Contribution guide](/docs/contributing): habits that make reviews quicker.
- [FAQ](/faq)
- If you get stuck on SkillIssues itself, see [Contact](/contact).
