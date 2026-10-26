---
title: The desktop app
slug: /download
section: SkillIssues
summary: What the desktop app is for, where to get it, and why your GitHub token stays on your computer.
draft: false
updated: 2026-10-27
---

The desktop app handles the Git side of fixing an issue, so you don't juggle terminals and browser tabs. It is built for Windows, macOS and Linux.

## What it is for

The desktop app is the whole SkillIssues app (Explore, search, issue pages, your profile and XP, Settings and themes) plus the Git side. On an issue page, **Work on it here** picks up from there:

- It forks the project and clones your fork to a workspace folder you choose.
- It creates a branch for your change.
- It opens the project in VS Code, or shows you the folder.
- **Commit:** once you've changed files, you give the commit a one-line title and, if you like, a description.
- **Push:** sends your commits to your fork. It only pushes commits, so changes you haven't committed stay on your computer.
- **Pull request:** once your commits are on your fork, you write the pull request's title and description. Tick **Close #number when it's merged** if your change fixes the issue: GitHub then closes the issue on merge, and SkillIssues links the two, which is how a learning path step counts as done. Then **Head over to your pull request** opens it on GitHub, where maintainers review it.

**Working on**, in the app's menu, lists every issue you've started here and what's next for each: changes to commit, ready to push, ready for a pull request, or a pull request open.

The website can't do any of this, on purpose. See the next section.

**Open in the desktop app** on the website opens the same issue in the app, if it's installed. Docs and legal pages open in your browser.

## Why a desktop app

Forking, pushing and opening pull requests need a GitHub token that can write to your account. SkillIssues servers never hold one. The desktop app signs in to GitHub with a token that stays on your computer and talks to GitHub directly from there.

For the website, sign-in only reads who you are (your GitHub id, username, profile name and avatar), and the token is discarded straight away. See [Security](/security).

## Where to get it

The download above is the newest version for your system. Every version is also on the [GitHub releases page](https://github.com/dizziedbliss/SkillIssues/releases).

## Updates

On Windows, install it with `SkillIssues-Installer.exe` from the [download page](/download): agree to the terms, press **Install**, and it downloads the newest version from GitHub, checks it against the release's checksum, and installs it, for you or for everyone on the PC. Run it again later to **Update**, **Repair** or **Remove** SkillIssues. Without internet while installing, use the full offline installer, `SkillIssues-Setup-<version>.exe`, from the same page.

On Windows and Linux the app checks for a new version when it starts and every few hours, but never downloads one on its own: the title bar says **Update available**, and the update shows what's new and downloads when you choose. After it downloads, restart into it, or it installs when you close the app. The macOS build isn't signed yet, so it can't update itself: the button opens this page instead. The first time you open it on a Mac, right-click it and choose **Open**.

What each version changed is in its release notes. If something on this page doesn't match the version you have, tell us through [Report a bug](/docs/report-a-bug).

## Without the desktop app

You can fix an issue without it. Use the website to find the issue, then fork, clone and push with Git, the GitHub website or any tool you like. [Getting started](/docs/getting-started) walks through the steps.
