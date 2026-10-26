---
title: Changelog
slug: /docs/changelog
section: SkillIssues
summary: Where releases are published, and a summary of the notable changes made recently.
draft: false
updated: 2026-10-27
---

## Releases

Desktop builds and release notes are published on the [GitHub releases page](https://github.com/dizziedbliss/SkillIssues/releases), numbered like `1.3.0`. The website and API deploy every change merged to the main branch once it passes its checks, so they're versioned by date instead: **website `2026.10.26`** is what was merged on 26 October (UTC).

The full, detailed history is in [CHANGELOG.md](https://github.com/dizziedbliss/SkillIssues/blob/main/CHANGELOG.md) in the repository. This page summarises the parts you are most likely to notice.

## October 2026

### Website 2026.10.26 · Desktop 1.5.0: a waiting issue, and a calmer app

- **The home page leads with a real issue that has been waiting**, and how long: "Someone opened this 260 days ago. Nobody has fixed it yet." Pick a language and a level to swap it, or **[Find an issue]** to open Explore with those choices. It never shows an issue from a repository that's slow to merge, or one over a year old.
- **Your profile, rebuilt.** Your level large at the top; **Next up** with your pull requests in review, issues you've claimed and your learning path, then the quests you're closest to finishing; one list of your pull requests with the XP each earned; earned badges, with how you earned each on hover.
- **Your contributions**: your numbers, your year, every pull request with All, Merged and Open, where your XP came from, and every quest.
- **Quicker to find things.** One search bar everywhere; filters that say what they mean when you open them; the levels and your languages in one row; `/` opens search as a palette. The top bar holds where you work, the Profile menu what's about you.
- **Calmer colours.** Red marks what matters: the main button, where you are, what you've earned, and headings. Light mode is warmer and a little dimmer, easier over a long read.
- **These docs** have an *On this page* index and previous and next pages.
- **The live numbers no longer lock you out.** Leaving the home page open used to use up the requests a visitor may make in an hour; they don't count any more.

### Website 2026.10.24 · Desktop 1.3.0: a new home page and calmer colours

- **The Backlog Clock.** The home page opens on every open issue in the catalog as one live number, what arrived and closed today, and the closed-against-open bar.
- **Find your issue in ten seconds.** Pick a language and a level and get three of today's picks, one per repository and nobody assigned, with no account.
- **Got a project in mind?** Search the catalog's repositories by name, or paste `owner/name` or a GitHub link to see whether it's listed and, if not, why. When it isn't there, **Ask us to add it** opens the repository form with the details filled in. See [Suggest a repository](/docs/suggest-a-repository).
- **Warm paper and ink.** The site and the Default and Midnight themes moved off pure white and black, with a red set per mode so every text pairing passes WCAG AA.
- **Easier reading.** These docs and the legal pages set their text in a proportional face, larger and with a shorter line, and lists show their bullets again.
- **A living background**: a quiet grid where issues light up as they arrive and close, in today's real proportions.

### Website 2026.10.21: For you

- **For you replaces Shuffle** as Explore's default. It picks issues that suit you (your languages, your level, what you've merged before), that look likely to get merged (labelled, a short discussion, recently active, a welcoming repository), and that have waited, with few people shown them lately. Each page is a weighted draw, so good matches come first more often but everyone sees different issues. Each card says why it's there. See [Finding issues](/docs/finding-issues).
- **Explore starts from your profile.** Signed in, your preferred languages and level are the starting filters, shown under the search box with **Show every language**.
- **Coming back keeps your place.** Opening an issue and returning, by Back or the link, keeps Explore's filters and page.

### Website 2026.10.21 · Desktop 1.1.0 to 1.2.1: polish from review

- **Working on, in the desktop app.** Every issue you've started in the app, and what's next for each. Commit (a title and an optional description), push (commits only) and open the pull request (you write it, and choose whether it closes the issue) are separate steps, each offered once there's something to do.
- **Asked before it's done.** Giving up a learning path, releasing a claim and deleting a theme ask first.
- **Learning path swaps** never offer an issue you've swapped away on that path again.
- **One list of your pull requests**, merged ones marked merged, with All, Merged and Open.
- **Search finds people** by their GitHub username, if their profile is public, and pages further on Explore. Newest and Easiest sort a search.
- **Issue pages** show images GitHub hosts as attachments, colour code blocks in your theme's colours, offer Try again when the description didn't load, and offer nothing to work on once an issue is closed.
- **Closed, assigned and claimed issues** are coloured in lists.
- **The download page** offers the newest version for your system, the others, and older versions.
- **Smaller desktop app**, and copying the sign-in code works.

### Website 2026.10.20: the app, from its designs

- **Explore.** The catalog as designed: search, a filter panel where every filter explains itself, difficulty buttons that start from your profile's level, and pages you can go back through. Click a card anywhere to open its issue, or its repository's name to see that repository's issues.
- **Search, everywhere.** Press `/` or Ctrl+K on any page in the app. Results come as you type, sorted into repositories, titles and labels, and languages and tech, each with View more. See all results on the search page. See [Finding issues](/docs/finding-issues).
- **Issue pages.** The description (with its images), labels, the repository's details, links to more issues from the repository and its owner, and a button that opens the desktop app. If GitHub says the issue has just been closed, the page tells you.
- **Your profile and contributions.** Your public pull requests, merged ones and the repositories you've worked on, a graph of your year, and achievements. Your browser reads these from GitHub's public search, so we still hold no GitHub token for you.
- **Saved issues and updates.** Your saved issues, with a note when one closes on GitHub, and a red dot on Profile until you've read it.
- **Settings.** Your profile, themes, the desktop app, your account and support, each on its own page.
- **Dark by default.** The whole site starts dark. The footer's theme button switches the landing page and docs to light.
- **Ready for translations.** Every word of the interface can now be translated. English is the only language so far.
- **Docs search.** Press `/` on any docs page.
- **Is this a good first issue?** Each issue page checks the issue as GitHub has it now: marked for beginners, described, unassigned, no open pull request, recently active, a short discussion. A new guide explains what to look for. See [Is this a good first issue?](/docs/good-first-issues).
- **XP, levels, streaks and badges.** Merged pull requests to repositories in the catalog earn XP, more for closing harder and older issues. SkillIssues reads them from public GitHub data with its own credential, never yours, and past merges count at half XP. Your profile shows your level, a level per language, your weekly streak, badges, and every pull request earning XP. See [How XP works](/docs/xp).
- **How a project treats contributions.** Cards say when a repository merges work from people outside the project, or is slow to merge, from its latest merged pull requests. See [Finding issues](/docs/finding-issues#reading-an-issue).
- **Public profiles.** Turn on **Show my profile to anyone** in Settings, and anyone can see your level, badges and merges. It's off unless you turn it on. The leaderboard is built, and opens later for people who show their profile.
- **Quests and learning paths.** Seven one-off quests that pay XP, and learning paths: five issues for the kind of work you want to get better at, from beginner to a step up. See [How XP works](/docs/xp#learning-paths).
- **Claims.** Press **I'm working on this** on an issue, and other SkillIssues users see it's taken for a week, or while your pull request for it is open. See [Claims](/docs/good-first-issues#claims).
- **Reporting.** Signed in, report an issue, a repository or a shared theme. Reports are private, and enough of them take it out of lists until it's reviewed. See [Reporting](/docs/reporting).
- **Community themes.** Browse the themes people have shared, most used or newest, in **Settings → Themes → Community**.

### Website 2026.10.14 to 2026.10.20: the groundwork

#### Catalog and search

- **Filters.** Filter the catalog by labels, main language, technologies a repository uses, repository topics, difficulty, and how many comments an issue has. See [Finding issues](/docs/finding-issues).
- **A fair default feed.** The default order gives every repository one slot, showing its easiest open issue, in an order that is redrawn daily. Sorting by easiest or newest is still available.
- **Faster search.** Search had started timing out on the full catalog. Each kind of match is now looked up separately and the results are interleaved. Search now looks at repository names, titles and labels, and languages and tech; it no longer matches the people who opened issues.
- **Live counts.** The landing page reads live catalog counts from a new public `GET /stats` endpoint.
- **Cursor paging.** The API can page through the catalog by cursor, which costs the same however deep you go. See [Public API](/docs/public-api).

#### Freshness

- **Hourly updates from GH Archive.** Issues opened, edited, commented on or closed on GitHub now show up here within about two hours, without spending GitHub API quota. A slower monthly pass re-reads every repository directly to repair anything missed.
- **Discovery.** A sweep of GitHub's repository search finds new repositories that qualify, so the catalog is no longer limited to the initial dataset.
- **Issue descriptions.** The website reads issue descriptions from GitHub directly. If GitHub refuses because of your own rate limit, SkillIssues serves a cached copy as a fallback.

#### Themes

- **User themes.** Restyle the signed-in app with a theme file of design tokens and an optional stylesheet. Themes sync to your account and can be shared. See [Themes](/docs/themes).
- **Built-in themes.** Twelve built-ins, each in light and dark, including Catppuccin, Dracula, Nord, Gruvbox, Solarized, Tokyo Night and Rosé Pine. Use one as it is, or customise your own copy.
- **An editor with a live preview.** Problems are shown as you type, with their line. A preview shows the app in your draft, with a map of which token paints what.
- **More for theme authors.** Light and dark halves in one theme, more fonts, more tokens, page transitions, `@property` and scroll-driven animation, stable hooks for stylesheets, and `POST /themes/validate` to check a theme without saving it.
- **Appearance.** The profile menu lists your recent themes to switch between.
- **Light and dark in every theme.** Every built-in has a light and a dark half, and your own themes can too. The footer's theme button switches between them.

#### Website

- **A new landing page and a rebuilt website**, being rolled out alongside the rebuilt desktop app.

#### Accounts and security

- **No user tokens on the server.** Signing in uses a GitHub app that asks for no permissions. We read your numeric id, username and avatar, then discard the token. Anything that writes to GitHub moved into the desktop app. See [Security](/security).
- **Verified releases.** Every image is built by public CI, signed, and verified on the server before it runs.
- **Encrypted secrets and backups.** Production secrets are stored encrypted, and database backups are encrypted with a key that is not on the server.
- **Cloudflare in front.** The site is set up to sit behind Cloudflare without changing how visitors are counted or limited.

#### Live visualizer

- **SkillIssues Live.** A public, real-time view of data moving through the system, with anything tied to a signed-in person shown as random symbols. See [SkillIssues Live](/docs/skillissues-live).

#### Removed

- **XP, levels and badges.** The first version awarded these from pull requests verified with a stored user token. They were removed with the no-stored-tokens redesign, and came back on 21 October as a ledger of merges the server verifies itself. See [How XP works](/docs/xp).

## Website 2026.10.13 · Desktop 1.0.0

The first production release. The site went live on a real server with automated, gated deploys, and the web and desktop apps began sharing one set of code for talking to the API and drawing the interface.
