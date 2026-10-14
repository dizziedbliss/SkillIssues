# Security

SkillIssues runs in production against real GitHub accounts: it stores users' GitHub
authorizations (encrypted at rest) and acts on their behalf when they fork, push and open pull
requests. Security reports are taken seriously.

## Reporting a vulnerability

**Do not open a public issue or pull request for a vulnerability.** Contact the maintainer,
[@dizziedbliss](https://github.com/dizziedbliss), privately on GitHub, with:

- what is affected (endpoint, component or file) and what an attacker gains;
- steps to reproduce, or a proof of concept;
- whether you have seen it exploited.

The repository is private today, so GitHub's private vulnerability reporting is not available
for it. If the repository becomes public, that feature will be enabled and becomes the preferred
channel.

## What matters most

- Anything that exposes a user's GitHub token, a session, or another user's data.
- Anything that lets a write to GitHub (fork, push, pull request) happen under the wrong
  identity — in particular, under the server's credential instead of the user's.
- Ways to earn XP without a merged pull request you authored.
- Reaching internal-only endpoints (`/metrics`, `/health/detail`, ingestd's `/sync/*`, Grafana,
  Prometheus) from the internet.
- Script injection through rendered issue bodies (they are attacker-controlled and must go
  through `@skillissues/markdown`).

Only the latest deployed version is supported; fixes ship through `main`.
