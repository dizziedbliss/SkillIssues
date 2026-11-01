# SkillIssues: guide for coding agents

For every AI coding agent (Claude Code, Codex, Cursor, Copilot, Gemini…). The rules live in the
`CLAUDE.md` of each folder; the `AGENTS.md` beside it only points here, so there is one text to keep true. Contributors using AI tools also
follow *Using AI tools* in [CONTRIBUTING.md](CONTRIBUTING.md).

**This is production code.** It runs against the real GitHub API with a registered GitHub App, crawls
real repositories, and holds real people's GitHub identities. Every merge to `main` deploys.

## Guides per area (read the one for where you work)

| Path | Guide |
|---|---|
| `backend/` | Go: API, crawler, flow relay, import, migrate → [backend/CLAUDE.md](backend/CLAUDE.md) |
| `backend/internal/ingest/` | the crawl and its rules → [CLAUDE.md](backend/internal/ingest/CLAUDE.md) |
| `backend/internal/issue/` | catalog queries, search, For you → [CLAUDE.md](backend/internal/issue/CLAUDE.md) |
| `backend/internal/migrate/` | migrations, grants, settings rows → [CLAUDE.md](backend/internal/migrate/CLAUDE.md) |
| `web/` | the site and app, interface design → [web/CLAUDE.md](web/CLAUDE.md) |
| `desktop/` | the Electron app and its releases → [desktop/CLAUDE.md](desktop/CLAUDE.md) |
| `installer/` | the Windows setup → [installer/CLAUDE.md](installer/CLAUDE.md) |
| `packages/` | `client`, `theme`, `markdown` → [packages/CLAUDE.md](packages/CLAUDE.md) |
| `.github/` | CI, deploy, releases → [.github/CLAUDE.md](.github/CLAUDE.md) |
| `monitoring/` | Prometheus, Grafana, Loki, Alloy → [monitoring/CLAUDE.md](monitoring/CLAUDE.md) |

Background, not rules: [ARCHITECTURE.md](ARCHITECTURE.md), `docs/` (API, settings, database, ingestion design).

## Workflow

```bash
cp .env.example .env && docker compose up -d --build   # full stack; `migrate` applies the schema, no manual SQL
npm ci                                                  # once, at the repo root (web, desktop, packages/*)

# Then the checks for each part you touched, in the order its guide gives. Before a PR, what CI runs:
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build && npm test
cd packages/theme && npm test
```

Run them before saying work is done. **Tests pass with Redis stopped.**

## Rules you must not break

- **Nothing swallows an error.** No discarded `err`, no `catch(() => [])`, no ignored `UPDATE` result, no
  handler reporting success after a failed write. A failure nobody can see is worse than a crash.
- **No operational value is hardcoded.** Thresholds, intervals, page sizes, worker counts and caps are
  live settings (`settings.Registry` + a seeded row in a migration; `docs/settings.md`).
- **Migrations are append-only**, and a new table is granted to the roles that use it.
- **Failures are observable.** `/metrics` and `/health/detail` reflect reality; a component that is down
  is never logged as connected.
- **Never log a credential**, not even a prefix or its length.
- **The server holds no user GitHub token, ever.** Fork, push and pull requests happen in the desktop app
  with a token that stays on the person's machine. **`github_id` is the only identity.**
- **Issue bodies are attacker-controlled:** render them only through `packages/markdown`.
- **No popularity ranking** in the crawl or the For you score. Unlabelled issues are `UNCLASSIFIED`,
  never `BEGINNER`: unknown is not a negative.
- **Don't widen a security boundary** (CSP, CORS, the desktop's `security.ts`, rate limits, database
  grants, workflow permissions) without saying why in the pull request.

## Deployment

One VPS (`compose.prod.yaml`, Caddy) for everything but the site, which is on Vercel. The site calls the
API host cross-origin; the two hosts are one site, so the Lax session cookie travels, and go-api's CORS
and CSRF trust `FRONTEND_URL`.

- **Operational notes never enter the repository:** host access, credentials, rotation and restore steps
  are not committed, linked or described in tracked files.
- **Never commit a plaintext secret.** Secrets are SOPS-encrypted in `secrets/prod.env.sops`.
- `compose.prod.yaml` only pulls signed GHCR images; nothing builds on the host. One root `Dockerfile`
  with targets `api`, `ingestd`, `flowd`, `migrate`, `repoimport`; `web/Dockerfile` is the dev-only Vite
  image. Don't add a third.
- **Caddy is the only public port.** It returns 404 for `/metrics`, `/health/detail` and `/admin/*`, takes
  the client address from `CF-Connecting-IP` only from Cloudflare's ranges (`trusted_proxies`, keep them
  current from cloudflare.com/ips), and sends go-api exactly that in `X-Forwarded-For`. Cloudflare's
  "Always Use HTTPS" stays off: it breaks the ACME HTTP-01 challenge.
- The services connect as `skillissues_api`, `skillissues_ingest`, `skillissues_flow`, never the owner.
- **Backups are encrypted on the host** (`scripts/backup-db.sh`: `pg_dump | gzip | age`); the private key
  is off the server. `BACKUP_DESTINATION` is required with no default (`r2`, or `local`, which has no
  off-site copy). It fails loudly on any missing setting or failed step and never keeps a plaintext copy.
- **Never write production data, settings or infrastructure** without the maintainer's go-ahead, each
  time.

## Git and pull requests

- Branch from `main`: `feat/…`, `fix/…`, `docs/…`, `refactor/…`, `chore/…`. Conventional Commits
  (`feat(search): …`, `fix(desktop): …`). Never push to `main`.
- Commit only when the maintainer asks; when you do, push the branch in the same step.
- One concern per PR: what changed, why, how you tested it, and that an AI tool was used.
- SemVer and [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The website and API share
  `CHANGELOG.md`; the desktop has `desktop/release-notes.md`. A change people will notice adds a line
  under `## [Unreleased]` in the right one, in the same PR.

## Never do these

- Add `pull_request_target`, widen a workflow's `permissions`, or replace a pinned action SHA with a tag.
- Add a dependency without a reason, from a git URL or tarball, or without checking the package really
  exists. AI tools invent package names, and attackers register them.
- Add obfuscated code, `eval`, encoded blobs, or behaviour that depends on a date, user or hidden
  variable. CI passing is not proof a change is safe (*What gets a pull request closed* in CONTRIBUTING.md).

## Gotchas

- `npm ci` runs once at the repo root. `npm install`, or an install inside `web/` or `desktop/`, breaks
  the single lockfile and the `@skillissues/*` links.
- Container healthchecks use `127.0.0.1`, not `localhost`: some images resolve `localhost` to IPv6 first,
  and the check failed as if the service were down.

## Keeping these guides right

- **They describe what is true now.** No planned features. Where a designed piece is deliberately not
  built, its guide says so under *Not built*.
- **Short beats complete.** For each line ask: would removing it cause a mistake? If not, cut it. No
  codebase tours, no style a formatter enforces, nothing that changes weekly.
- **Every mistake becomes a line.** When an agent gets something wrong, or a review catches it, add it
  to *Gotchas* in the guide nearest the code, in the same PR.
- **Edit `CLAUDE.md`, never `AGENTS.md`.** Each `AGENTS.md` only points to the `CLAUDE.md` beside it. A
  new folder guide gets both files and a row in the table above.
