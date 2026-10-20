# Security

SkillIssues runs in production against real GitHub accounts. **The server holds no user GitHub
token**: sign-in uses a no-scope OAuth App that only reveals who the user is, and the token is
discarded after the numeric GitHub id is read. Anything that writes to GitHub (fork, push, pull
request) is done by the desktop client with a token that stays on the user's machine. Security
reports are taken seriously.

## Reporting a vulnerability

**Do not open a public issue or pull request for a vulnerability.** Contact the maintainer,
[@dizziedbliss](https://github.com/dizziedbliss), privately on GitHub, with:

- what is affected (endpoint, component or file) and what an attacker gains;
- steps to reproduce, or a proof of concept;
- whether you have seen it exploited.

The repository is public. Until GitHub's private vulnerability reporting is enabled for it,
contact the maintainer privately as above; once it is, that becomes the preferred channel.

## What matters most

- Anything that exposes a session, another user's data, or the server's own credentials (the
  GitHub App key, the admin token, database passwords).
- Anything that makes the server hold or forward a user's GitHub token, or take over an account
  (accounts are matched on GitHub's numeric id, never on a username).
- Anything that lets an image other than one built and signed by this repository's `deploy.yml`
  on `main` run in production.
- Reaching internal-only endpoints (`/metrics`, `/health/detail`, ingestd's `/sync/*`, Grafana,
  Prometheus) from the internet.
- Script injection through rendered issue bodies (they are attacker-controlled and must go
  through `@skillissues/markdown`).

Only the latest deployed version is supported; fixes ship through `main`.
