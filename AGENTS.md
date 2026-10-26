# AGENTS.md

Instructions for AI coding agents (Codex, Cursor, Copilot, Gemini, Claude and others) working in this
repository. This is the short version. [CLAUDE.md](CLAUDE.md) is the complete reference: read it before
changing the ingestion pipeline, the database, authentication, deployment or the theme system. When the
two disagree, CLAUDE.md wins. Tell the maintainer, so this file can be fixed.

Contributors using AI tools also follow *Using AI tools* in [CONTRIBUTING.md](CONTRIBUTING.md).

## What this is

SkillIssues helps people find open GitHub issues they can solve and follows them to a merged pull
request. **It is production code.** It runs against the real GitHub API with a registered GitHub App,
crawls real repositories, and holds real people's GitHub identities. Every merge to `main` deploys.

| Path | What |
|---|---|
| `backend/` | Go: `cmd/api` (HTTP API, :8000), `cmd/ingestd` (GitHub crawler, :8002), `cmd/flowd` (live visualizer relay), `cmd/migrate`, `cmd/repoimport` |
| `backend/internal/migrate/migrations/` | Goose migrations, the only schema source |
| `web/` | React 19 + Vite site and app (deployed to Vercel), CSS Modules |
| `desktop/` | Electron app, TypeScript strict |
| `packages/` | `client` (typed API), `theme` (user themes), `markdown` (sanitised rendering) |
| `monitoring/` | Prometheus, Grafana, Loki, Alloy |

## Commands

```bash
# Full stack
cp .env.example .env && docker compose up -d --build

# Backend
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...

# Web and desktop: one npm workspace, install once at the repo root, with ci (dependencies are pinned)
npm ci
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build && npm test
cd packages/theme && npm test
```

Run the checks for every part you touch before saying the work is done. Tests must pass with Redis
stopped.

## Rules you must not break

- **Never swallow an error.** No discarded `err`, no `catch(() => [])`, no handler that reports success
  after a failed write. A failure nobody can see is a bug.
- **Never hardcode an operational value** (limit, interval, page size, worker count, threshold). Add a
  key to `settings.Registry` and a seeded row in a new migration. See `docs/settings.md`.
- **Migrations are append-only.** Never edit one that has been applied anywhere. Add a new one, idempotent,
  with Up and Down. A new table is granted to the database roles that use it.
- **Never log a credential**, not even a prefix or its length.
- **The server holds no user GitHub token, ever.** Sign-in reads the identity and discards the token. Fork,
  push and pull requests happen in the desktop app with a token that stays on the user's machine.
- **`github_id` is the only identity**; a username is display data.
- **Render issue bodies only through `packages/markdown`**: they are attacker-controlled.
- **Thread `ctx`** into every database, Redis and HTTP call; use the shared `pgxpool`.
- **Unlabelled issues are `UNCLASSIFIED`, never `BEGINNER`.** Unknown is not a negative.
- **No ranking by popularity in the crawl.** Stars never enter the For you score.
- **Don't widen a security boundary** (CSP in `vercel.json` or the desktop, CORS, the desktop's
  `security.ts`, rate limits, database grants) without saying why in the pull request.

## Interface design

The maintainer's direction for every screen. Full version: *Interface design* in CLAUDE.md.

- **Hairlines, not boxes.** Sections are split by space and 1px rules; boxes only for things that float
  (menus, dialogs, the search palette).
- **The accent is rare:** the main button, where you are, what's earned, and section headings. Page titles
  and body text stay in the text colour. Rules and chips are neutral (`--rule`, `--chip`).
- **Light mode is soft:** warm, slightly dim paper and softened ink. Never pure white or black.
- **Fewer words.** Don't add paragraphs explaining controls; let the control explain itself.
- **Reuse the shared components**: `explore/SearchBar.tsx`, `profile/Header.tsx`. Don't restyle a copy.
- **Whole rows and cards are click targets**; no extra "View" buttons.
- **Every string is an i18n key** in `web/src/i18n/en.ts`; never hardcode visible text.
- **Keep `data-ui` hooks stable**: user themes select them (`web/content/docs/themes.md`).
- Check dark and light, and a 390px-wide phone, with real data.

## Git and pull requests

- Branch from `main`: `feat/…`, `fix/…`, `docs/…`, `chore/…`. Conventional Commits:
  `feat(search): …`, `fix(desktop): …`.
- Never push to `main`; it changes only through reviewed pull requests.
- Commit only when the maintainer asks. When you do, push the branch in the same step.
- One concern per pull request. Say what changed, why, how you tested it, and that an AI tool was used.
- Versions follow [SemVer](https://semver.org/) and changelogs [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
  The website and API share one version (`CHANGELOG.md`); the desktop has its own (`desktop/release-notes.md`).
  A change people will notice adds a line under `## [Unreleased]` in the right one.

## Never do these

- Commit notes about operating production (host access, credentials, rotation or restore steps).
- Commit a plaintext secret. Secrets are SOPS-encrypted in `secrets/prod.env.sops`.
- Add `pull_request_target`, widen a workflow's `permissions`, or replace a pinned action SHA with a tag.
- Add a dependency without a reason, from a git URL or tarball, or without checking that the package
  really exists. AI tools invent package names, and attackers register them.
- Add obfuscated code, `eval`, encoded blobs, or behaviour that depends on a date, user or hidden
  variable. CI passing is not proof a change is safe; see *What gets a pull request closed* in CONTRIBUTING.md.
- Write production data, settings or infrastructure without the maintainer's explicit go-ahead each time.

## Where to read more

- [CLAUDE.md](CLAUDE.md): everything above in full, plus architecture, ingestion policy, deployment and
  key design decisions.
- [CONTRIBUTING.md](CONTRIBUTING.md): setup, checks, review rules, AI policy.
- [ARCHITECTURE.md](ARCHITECTURE.md), `docs/` (API, settings, database, ingestion design).
- [SECURITY.md](SECURITY.md): report vulnerabilities privately.
