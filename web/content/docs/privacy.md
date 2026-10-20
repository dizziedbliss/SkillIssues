---
title: Privacy policy
slug: /docs/privacy
section: Legal & Trust
summary: What SkillIssues collects, why, and what it doesn't collect.
draft: true
updated: 2026-10-20
---

> **Draft.** This policy describes what the software does today. It is a draft waiting for review by the owner and has not been checked by a lawyer. It may change before it is final.

## If you only browse

You don't need an account to browse. We don't ask for your name or email, we don't set cookies for browsing, and we don't use third-party analytics or advertising.

To deliver pages and to stop one address using up the service, our infrastructure sees your IP address:

- **Cloudflare**, our network provider, sits in front of the site and handles your request first.
- **Our web server** keeps access logs that include IP addresses. The logs are rotated out as they fill.
- **Our rate limiter** keeps a short-lived counter per address (or per signed-in account). The counter expires within the rate-limit window, which is an hour by default.

The issues and repositories you see are public GitHub data.

## If you sign in

You sign in with GitHub through an app that asks for no permissions.

**From GitHub we keep:**

- your numeric GitHub id;
- your GitHub username;
- the link to your avatar.

**We do not keep your GitHub token.** We use it once to read the three items above and then discard it.

**From you we keep:**

- the profile and preferences you set, such as your bio, experience level, target level, interests and preferred languages;
- the issues you save;
- the themes you create, and which one is applied to your account;
- notifications about saved issues that close, which the system records for you.

**To keep you signed in we keep:**

- a session, stored as a hash. Your browser holds a secure, HTTP-only cookie;
- a short-lived cookie while you sign in, which ties the sign-in to the browser that began it.

A session lasts for 7 days by default, or until you sign out or sign out everywhere.

## How we use it

- To show you the catalog, ordered by your preferred languages.
- To keep your saved issues, preferences and themes.
- To check that a saved issue is still open.
- To keep the service safe, such as rate limiting.

We don't sell your data, show ads, build advertising profiles or share your data with third parties for their own use.

## Themes you share

A theme you mark public can be opened by anyone who has its link. It shows the theme file and the username you signed in with. A private theme is visible only to you.

## SkillIssues Live

The public live visualizer shows system activity. Anything tied to a signed-in person is replaced with random symbols before it leaves our servers, and no event contains an address, token or user id. See [SkillIssues Live](/docs/skillissues-live).

## Where data is kept

Data is stored in a database on our server. Production secrets and backups are encrypted, and backups are encrypted with a key that isn't kept on the server. See [Security](/docs/security).

## Deleting your data

You can delete your account. This removes your profile, preferences, saved issues, themes, sessions and notifications. To ask, [open an issue](https://github.com/dizziedbliss/SkillIssues/issues/new) and say you want your account deleted. Don't include private details. Deleting your account also gives you a link to the page on GitHub where you can remove the sign-in app from your authorised apps.

Backups are kept for a limited time, so your data can remain in an encrypted backup for a while after it's deleted from the live database.

## Children

SkillIssues is a tool for developers. We don't knowingly collect data from children.

## Changes

We may update this policy. The date at the top of the page shows when it last changed.

## Contact

Questions about privacy: [open an issue](https://github.com/dizziedbliss/SkillIssues/issues/new). For a security problem, see [Security](/docs/security).
