---
title: Privacy policy
slug: /privacy
section: Legal & Trust
summary: What SkillIssues collects, why, and what it doesn't collect.
updated: 2026-10-25
---

## Who is responsible

SkillIssues is run by @dizziedbliss on GitHub, an individual based in Bengaluru, Karnataka, India. For the purposes of India's Digital Personal Data Protection Act, 2023 and similar laws such as the GDPR, we are the data fiduciary (controller) of the personal data described here. Privacy contact and grievance contact: privacy@dizziedbliss.lol.

## If you only browse

You don't need an account to browse. We don't ask for your name or email, we don't set cookies for browsing, and we don't use third-party analytics or advertising.

Your browser keeps a few of your choices in its own storage: light or dark, your language, your theme and the themes you used recently, and for half an hour a copy of the public pull request numbers your profile shows. They stay on your device and never reach us; clearing your browser's site data removes them.

To deliver pages and to stop one address using up the service, our infrastructure sees your IP address:

- Vercel serves the site's pages and handles your request for them first.
- Cloudflare, our network provider, sits in front of the API the pages call.
- Our API server keeps access logs that include IP addresses. They rotate by size, so each log is deleted as newer ones replace it, usually within a few days.
- Our rate limiter keeps a short-lived counter per address (or per signed-in account). The counter expires within the rate-limit window, which is an hour by default.

The issues and repositories you see are public GitHub data.

## If you sign in

You sign in with GitHub through an app that asks for no permissions.

From GitHub we keep:

- your numeric GitHub id;
- your GitHub username;
- the link to your avatar.

We do not keep your GitHub token. We use it once to read those three items and then discard it.

From you we keep:

- the profile and preferences you set, such as your bio, experience level, target level, interests and preferred languages;
- the issues you save;
- the themes you create, and which one is applied to your account;
- notifications about saved issues that close, which the system records for you;
- reports you make: what you reported (an issue, a repository or a shared theme), the reason you chose and any note you add;
- issues you claim ("I'm working on this") and your learning path;
- whether your profile is public.

While you have an account, we also read your public pull requests from GitHub, using SkillIssues' own GitHub credential (never yours), and keep the ones to repositories in our catalog: each one's title, link, state, dates, size, whether its checks passed, whether it was approved or had changes requested, and which catalog issues it closes. From them we keep the XP they earn, and your level, badges, streak, quests and learning path, which are worked out from it.

To keep you signed in we keep:

- a session, stored as a hash. Your browser holds a secure, HTTP-only cookie;
- a short-lived cookie while you sign in, which ties the sign-in to the browser that began it.

A session lasts for 7 days by default, or until you sign out or sign out everywhere.

## The desktop app

The desktop app uses a GitHub token that stays on your computer. It never reaches our servers. The desktop app sends no usage data or crash reports to us, and it doesn't check for updates on its own.

## How we use it, and why

| Purpose | Basis |
| --- | --- |
| Sign you in and keep your session | Your consent, and providing the service you asked for |
| Show the catalog ordered by your languages; keep your saved issues, preferences and themes; check that a saved issue is still open | Providing the service you asked for |
| Read your public pull requests to award XP, levels, badges, quests and learning paths, and keep your claims | Providing the service you asked for |
| Show your public profile and list you on the leaderboard, only if you turn it on | Your consent |
| Rate limiting, logs and abuse prevention | Legitimate interests (keeping the service safe and available) |
| Responding to your requests and reports | Legitimate interests, and legal obligations |
| Review reports of issues, repositories and themes, and leave reported content out of lists until it's reviewed. Only the people who run SkillIssues see reports; they're never shown to the person or project you reported | Legitimate interests (keeping the catalog safe and useful) |

We don't sell your data, show ads, build advertising profiles or share your data with third parties for their own use.

## Who else handles your data

- **GitHub** provides sign-in and the public issue data.
- **Vercel** delivers the site's pages and sees visitor IP addresses. It operates globally.
- **Cloudflare** sits in front of the API and sees visitor IP addresses. It operates globally.
- **OVH** hosts the server and database, in Sydney, Australia.

These providers process data only to run the service for us. We may also disclose data if the law requires it.

## Your public profile

Your profile is private unless you turn on "Show my profile to anyone" in Settings. Then anyone can open a page with your username, name, avatar, when you joined, your level, XP, badges, languages and merged pull requests to catalog repositories, and you can be listed on the leaderboard when it opens. Turning it off removes the page and takes you off the leaderboard straight away. A claim shows other people that an issue is claimed, never who claimed it.

## Themes you share

A theme you mark public can be opened by anyone who has its link. It shows the theme file and the username you signed in with. A private theme is visible only to you.

## SkillIssues Live

The public live visualizer shows system activity. Anything tied to a signed-in person is replaced with random symbols before it leaves our servers, and no event contains an address, token or user id. See [SkillIssues Live](/docs/skillissues-live).

## Where data is kept and international transfers

Data is stored in a database on a server in Sydney, Australia. Because Vercel, Cloudflare and GitHub operate globally, and you may be anywhere in the world, your data can be processed in other countries. Production secrets and backups are encrypted, and backups are encrypted with a key that isn't kept on the server. See [Security](/security).

## How long we keep it

| Data | How long |
| --- | --- |
| Account, profile, preferences, saved issues, themes, notifications, reports, the pull requests we track, XP, claims, learning paths | Until you delete your account |
| Sessions | 7 days by default, or until you sign out |
| Access logs (include IP addresses) | Until newer logs replace them (they rotate by size), usually within a few days |
| Service logs | 14 days, with every IP address removed before they are stored |
| Rate-limit counters | Within the rate-limit window (an hour by default) |
| Encrypted backups | 3 days, then deleted. Backups are encrypted and kept on the server. Your data can remain in a backup for that time after you delete it from the live database |

## Your rights

Depending on where you live, you can:

- ask what data we hold about you and get a copy;
- correct it, or ask us to delete it;
- withdraw consent (for example, by deleting your account);
- object to or ask us to limit some processing;
- nominate someone to exercise your rights if you can't (a right under India's DPDP Act);
- make a complaint to us at privacy@dizziedbliss.lol, and, if you are not satisfied, to your local data protection authority (in India, the Data Protection Board).

We aim to reply within 30 days.

## Deleting your data

Sign in, open [the app](/app) and choose Delete account: it happens straight away. If you can't sign in, email privacy@dizziedbliss.lol with your GitHub username and we will delete it; we may ask you to confirm the request from that GitHub account first, for example with a comment on GitHub. Please don't post a deletion request in a public issue, since that publishes your username and request. Deleting your account removes your profile, preferences, saved issues, themes, sessions, notifications, reports, tracked pull requests, XP, claims and learning paths. You can also remove the sign-in app from your authorised applications in GitHub's settings.

## Security and breaches

We protect your data as described on the [Security](/security) page. If a breach affects your personal data, we will tell the people affected and the authorities as the law requires.

## Children

SkillIssues is for people aged 18 and over. We don't knowingly collect data from anyone under 18. If you believe a child has an account, email privacy@dizziedbliss.lol and we will delete it.

## Changes

We may update this policy. For material changes we will put a notice on the site before they take effect. The date at the top of the page shows when it last changed.

## Contact

Privacy questions, requests and complaints: privacy@dizziedbliss.lol. For a security problem, see [Security](/security).
