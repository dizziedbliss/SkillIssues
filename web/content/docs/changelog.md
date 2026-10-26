---
title: Changelog
slug: /docs/changelog
section: SkillIssues
summary: What changed in each version of the website and the desktop app, newest first.
draft: false
updated: 2026-10-27
---

Every version of the website and the desktop app, newest first. The full, technical history is in [CHANGELOG.md](https://github.com/dizziedbliss/SkillIssues/blob/main/CHANGELOG.md) in the repository, and desktop downloads are on the [releases page](https://github.com/dizziedbliss/SkillIssues/releases).

Both follow [Semantic Versioning](https://semver.org/): the first number changes when something you relied on works differently, the second for new features, the third for fixes. The website and the API share one version, since they're deployed together every time a change is merged; the desktop app has its own. This page follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Website 2.7.1 and desktop 1.5.1, 26 October 2026

### Security

- **Safer issue descriptions.** The sanitiser that cleans what people write on GitHub before it's shown here is updated for two serious bypasses.
- **The desktop app runs on Electron 44.9.3**, with its security fixes.

## Website 2.7.0 and desktop 1.5.0, 26 October 2026

### Added

- **A new installer for Windows**: one clean window that downloads SkillIssues from GitHub, checks it, and installs it, with clear terms, a progress view, and Update, Repair or Remove when it's already installed. The download page offers it, with the full offline installer beside it.

### Changed

- **The desktop app gets everything from website 2.6.0**: the rebuilt profile and contributions, one search bar, self-explaining filters, calmer colours and a softer light mode.

## Website 2.6.0, 26 October 2026

### Added

- **The home page leads with a real issue that has been waiting**, and how long: "Someone opened this 260 days ago. Nobody has fixed it yet." Pick a language and a level to swap it, or **[Find an issue]** to open Explore with those choices. It never shows an issue from a repository that's slow to merge, or one over a year old.
- **Next up on your profile**: your pull requests in review and where they stand, issues you've claimed and your learning path, then the quests you're closest to finishing.
- **Badges explain themselves.** Hover one to see how you earned it.
- **These docs** have an *On this page* index and previous and next pages.

### Changed

- **Your profile, rebuilt.** Your level large at the top with its bar; one list of your pull requests, each once, with the XP it earned; your languages and badges beside them.
- **Your contributions**: your numbers, your year, every pull request with All, Merged and Open, where your XP came from, and every quest.
- **Quicker to find things.** One search bar everywhere; filters that say what they mean when you open them; the levels and your languages in one row; `/` opens search as a palette. The top bar holds where you work, the Profile menu what's about you.
- **Calmer colours.** Red marks what matters: the main button, where you are, what you've earned, and headings. Light mode is warmer and a little dimmer, easier over a long read.
- **A new footer**, and Settings, the learning path, reports and the theme gallery redrawn to match.

### Fixed

- **The live numbers no longer lock you out.** Leaving the home page open used up the requests a visitor may make in an hour; they don't count any more.

## Website 2.5.0 and desktop 1.3.0, 24 October 2026

### Added

- **The Backlog Clock.** The home page opened on every open issue in the catalog as one live number, what arrived and closed today, and the closed-against-open bar.
- **Find your issue in ten seconds.** Pick a language and a level and get three of today's picks, one per repository and nobody assigned, with no account.
- **Got a project in mind?** Search the catalog's repositories by name, or paste `owner/name` or a GitHub link to see whether it's listed and, if not, why. When it isn't there, **Ask us to add it** opens the repository form with the details filled in. See [Suggest a repository](/docs/suggest-a-repository).
- **A living background**: a quiet grid where issues light up as they arrive and close, in today's real proportions.

### Changed

- **Warm paper and ink.** The site and the Default and Midnight themes moved off pure white and black, with a red set per mode so every text pairing passes WCAG AA.
- **Easier reading.** These docs and the legal pages set their text in a proportional face, larger and with a shorter line, and lists show their bullets again.

## Website 2.4.1, 22 October 2026

### Fixed

- **Recommendations ignore stars**, so a popular repository isn't pushed ahead of a small one that suits you better.
- **Errors from GitHub are handled by kind**, so a repository GitHub can't find or a request that's too large no longer stalls the crawl.

## Website 2.4.0 and desktop 1.1.0 to 1.2.1, 21 October 2026

### Added

- **For you** replaces Shuffle as Explore's default. It picks issues that suit you (your languages, your level, what you've merged before), that look likely to get merged (labelled, a short discussion, recently active, a welcoming repository), and that have waited, with few people shown them lately. Each card says why it's there. See [Finding issues](/docs/finding-issues).
- **XP, levels, streaks and badges.** Merged pull requests to repositories in the catalog earn XP, more for closing harder and older issues. SkillIssues reads them from public GitHub data with its own credential, never yours, and past merges count at half XP. See [How XP works](/docs/xp).
- **Quests and learning paths.** One-off quests that pay XP, and learning paths: five issues for the kind of work you want to get better at, from beginner to a step up. See [How XP works](/docs/xp#learning-paths).
- **Claims.** Press **I'm working on this** on an issue, and other SkillIssues users see it's taken for a week, or while your pull request for it is open. See [Claims](/docs/good-first-issues#claims).
- **Is this a good first issue?** Each issue page checks the issue as GitHub has it now: marked for beginners, described, unassigned, no open pull request, recently active, a short discussion. See [Is this a good first issue?](/docs/good-first-issues).
- **How a project treats contributions.** Cards say when a repository merges work from people outside the project, or is slow to merge. See [Finding issues](/docs/finding-issues#reading-an-issue).
- **Public profiles.** Turn on **Show my profile to anyone** in Settings, and anyone can see your level, badges and merges. It's off unless you turn it on.
- **Reporting.** Signed in, report an issue, a repository or a shared theme. Reports are private, and enough of them take it out of lists until it's reviewed. See [Reporting](/docs/reporting).
- **Community themes.** Browse the themes people have shared in **Settings → Themes → Community**.
- **Search finds people** by their GitHub username, if their profile is public.
- **Desktop: Working on.** Every issue you've started in the app, and what's next for each. Commit, push and the pull request are separate steps, each offered once there's something to do.
- **Desktop: updates ask first**, and the app shows what changed after one installs. A proper Windows installer with the license and a choice of folder.

### Changed

- **Explore starts from your profile.** Signed in, your preferred languages and level are the starting filters.
- **Coming back keeps your place.** Opening an issue and returning keeps Explore's filters and page.
- **Asked before it's done.** Giving up a learning path, releasing a claim and deleting a theme ask first.
- **One list of your pull requests**, merged ones marked merged, with All, Merged and Open.
- **Issue pages** show images GitHub hosts as attachments and colour code blocks in your theme's colours.
- **Closed, assigned and claimed issues** are coloured in lists.
- **The download page** offers the newest version for your system, the others, and older versions.

### Fixed

- **Learning path swaps** never offer an issue you've swapped away on that path again.
- **Issue pages** offer Try again when the description didn't load, and nothing to work on once an issue is closed.
- **Desktop:** copying the sign-in code works, and the app is smaller to download.

## Website 2.3.0, 20 October 2026

### Added

- **The app, from its designs.** Explore with search, a filter panel and pages you can go back through; issue pages with the description, the repository's details and a button that opens the desktop app; your profile and contributions, read from GitHub's public search in your browser so we hold no token for you; saved issues, with a note when one closes on GitHub; Settings.
- **Search, everywhere.** Press `/` or Ctrl+K on any page in the app. Results come as you type, sorted into repositories, titles and labels, and languages and tech.
- **Filters** by labels, main language, technologies, topics, difficulty and how many comments an issue has, and a **fair default feed** that gives every repository one slot.
- **User themes.** Restyle the app with a theme file of design tokens and an optional stylesheet, with twelve built-ins (Catppuccin, Dracula, Nord, Gruvbox, Solarized, Tokyo Night, Rosé Pine and more), an editor with a live preview, and a light and a dark half in every theme. See [Themes](/docs/themes).
- **Live counts** from a public `GET /stats`, and **cursor paging** that costs the same however deep you go. See [Public API](/docs/public-api).
- **Docs on the site**, with search: press `/` on any docs page.
- **Ready for translations.** Every word of the interface can be translated. English is the only language so far.

### Changed

- **A new landing page and a rebuilt website.**
- **Dark by default.** The footer's theme button switches to light.
- **Faster search.** Each kind of match is looked up separately and the results are interleaved; search no longer matches the people who opened issues.

## Website 2.2.0, 19 October 2026

### Added

- **Hourly updates from GH Archive.** Issues opened, edited, commented on or closed on GitHub show up here within about two hours, without spending GitHub API quota.
- **Issue descriptions** are read from GitHub in your browser, with a cached copy as a fallback when GitHub refuses because of your own rate limit.

### Changed

- **The catalog is read in order** instead of sorted on every page, so a deep page is as quick as the first.
- **Cloudflare in front.** The site can sit behind Cloudflare without changing how visitors are counted or limited.

## Website 2.1.1, 18 October 2026

### Fixed

- **A few repositories that aren't really projects** (issue trackers used as databases, and the like) left the catalog.

## Website 2.1.0, 17 October 2026

### Added

- **Discovery.** A sweep of GitHub's repository search finds new repositories that qualify, so the catalog is no longer limited to the first dataset.
- **SkillIssues Live.** A public, real-time view of data moving through the system, with anything tied to a signed-in person shown as random symbols. See [SkillIssues Live](/docs/skillissues-live).

## Website 2.0.0, 16 October 2026

### Changed

- **Breaking: no user tokens on the server.** Signing in uses a GitHub app that asks for no permissions. We read your numeric id, username and avatar, then discard the token. Anything that writes to GitHub moved into the desktop app. See [Security](/security).

### Removed

- **XP, levels and badges**, which relied on a stored user token. They came back in 2.4.0 as a ledger of merges the server verifies itself.

### Security

- **Verified releases.** Every image is built by public CI, signed, and verified on the server before it runs.
- **Encrypted secrets and backups.** Production secrets are stored encrypted, and database backups are encrypted with a key that isn't on the server.

## Website 1.2.0, 15 October 2026

### Added

- **Closed issues are noticed.** SkillIssues asks GitHub what closed since it last looked and marks exactly those, instead of guessing.
- **A note when a saved issue closes** on GitHub.

## Website 1.1.0, 14 October 2026

### Added

- **When each issue was first seen and last checked** is recorded, so the catalog can say how fresh it is.

## Website 1.0.0 and desktop 1.0.0, 13 October 2026

The first production release. The site went live on a real server with automated, gated deploys, and the web and desktop apps began sharing one set of code for talking to the API and drawing the interface.
