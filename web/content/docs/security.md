---
title: Security
slug: /security
section: Legal & Trust
summary: How SkillIssues protects your account, your code and its own servers, and how to report a vulnerability.
draft: false
updated: 2026-10-20
---

## Your account

You sign in with GitHub through an app that asks for no permissions. We read your numeric GitHub id, username and avatar, then discard the token. We never store it, not encrypted and not for a minute.

Your account is tied to your GitHub id, never to your username. GitHub lets someone else take a username after you change yours, so a username can't be used to reach your account. If another account holds the username, the old row's username is cleared, not merged.

A session lasts until you sign out, it expires, or you sign out everywhere. It lives in a secure, HTTP-only cookie. Sessions are kept as hashes, so a copy of the database doesn't contain a usable session token. Sign-in is bound to the browser that began it, and requests from other websites that would change your data are refused.

Because there is no token to revoke, removing the app from your GitHub settings doesn't end a SkillIssues session. Sign out, or sign out everywhere.

## Your code

Anything that writes to GitHub (forking, pushing, opening pull requests) happens in the [desktop app](/download), with a token that stays on your computer. It never reaches our servers. SkillIssues servers can't act on your GitHub account.

Cloned code is never run by the desktop app.

## Issue content

Issue descriptions are written by strangers. Everything is rendered through a sanitising Markdown renderer, never as raw HTML.

## Themes

The app can be restyled with [themes](/docs/themes), including ones other people share. A theme's tokens are checked against a fixed schema and its stylesheet against a strict allowlist. A theme can't load anything from elsewhere, and the browser's security policy blocks it from trying. The landing page is never themed.

## Our servers

- **Signed releases.** Every release is built by our public CI, signed, and verified on the server before it runs. A pushed or re-tagged image does not deploy.
- **Encrypted secrets.** Production secrets are stored encrypted in the repository. Only the maintainer and the production server can decrypt them. CI never holds the key.
- **Encrypted backups.** Database backups are encrypted with a key that isn't on the server, so the server can write backups but can't read them.
- **Least privilege.** The API and the crawler connect to the database with accounts that can read and write data but can't change the schema or run commands. The live visualizer's relay can read one settings table and nothing else.
- **Internal tools stay internal.** Metrics, health details and admin routes are not reachable from the internet.
- **Rate limits.** Requests are limited per signed-in user, or per client address when signed out, so one visitor can't use up the service.
- **Cloudflare.** The site sits behind Cloudflare. The web server trusts the visitor address Cloudflare provides only when the request really came from Cloudflare.

## Reporting a vulnerability

Please don't open a public issue or pull request for a vulnerability.

Contact the maintainer, [@dizziedbliss](https://github.com/dizziedbliss), privately on GitHub, and include:

- what is affected (an endpoint, a component or a file) and what an attacker could gain;
- steps to reproduce, or a proof of concept;
- whether you've seen it being exploited.

Only the latest deployed version is supported. Fixes ship through the main branch.

### What matters most

- Anything that exposes a session, another person's data, or the server's own credentials.
- Anything that makes the server hold or forward a user's GitHub token, or take over an account.
- Anything that lets an image that wasn't built and signed by our CI run in production.
- Reaching internal-only endpoints from the internet.
- Script injection through rendered issue descriptions.

See also the [security policy](https://github.com/dizziedbliss/SkillIssues/blob/main/SECURITY.md) in the repository.
