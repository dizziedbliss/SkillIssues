---
title: The desktop app
slug: /download
section: SkillIssues
summary: What the desktop app is for, where to get it, and why your GitHub token stays on your computer.
draft: false
updated: 2026-10-20
---

The desktop app handles the Git side of fixing an issue, so you don't juggle terminals and browser tabs. It is built for Windows, macOS and Linux.

## What it is for

Finding an issue happens in the catalog on the website. The desktop app picks up from there:

- It forks the project and clones your fork to a workspace folder you choose.
- It creates a branch for your change.
- It opens the project in your editor.
- It pushes your branch to your fork when you're ready.

The website can't do any of this, on purpose. See the next section.

## Why a desktop app

Forking, pushing and opening pull requests need a GitHub token that can write to your account. SkillIssues servers never hold one. The desktop app signs in to GitHub with a token that stays on your computer and talks to GitHub directly from there.

For the website, sign-in only reads who you are (your GitHub id, username and avatar), and the token is discarded straight away. See [Security](/security).

## Where to get it

Builds are published on the [GitHub releases page](https://github.com/dizziedbliss/SkillIssues/releases). Download the file for your system from the newest release.

## The app is being rebuilt

The desktop app is being rebuilt alongside the website. Builds are published as they are released, and what each build can do is described in its release notes. If something on this page doesn't match the build you have, trust the release notes and tell us through [Report a bug](/docs/report-a-bug).

## Without the desktop app

You can fix an issue without it. Use the website to find the issue, then fork, clone and push with Git, the GitHub website or any tool you like. [Getting started](/docs/getting-started) walks through the steps.
