# Contributing to SkillIssues

Thanks for being here — typo fixes, bug reports, tests and features are all welcome, and nobody
expects you to understand the whole codebase on day one. Ask questions in issues or on your PR.

This is production code: it runs against the real GitHub API and attributes real contributions
to real accounts. The non-negotiable rules are in [CLAUDE.md](CLAUDE.md) (written for AI
assistants, but it's the most complete statement of the project's conventions — worth a read).

## What you need

- Git, Docker with Compose v2
- Go 1.26, Node.js 22, npm 11 — if you run services natively or work on the clients
- A GitHub token for `GITHUB_TOKEN` (no scopes needed; the crawler reads public data). For
  login, forking and PR flows locally you also need your own GitHub OAuth App — see
  `.env.example`.

## Where things live

```text
backend/cmd/          api, ingestd, migrate, repoimport, healthcheck
backend/internal/     auth, issue (search, bodies), ingest (crawler), progression (XP),
                      recommendation, settings, metrics, middleware, migrate/migrations, …
web/src/              React web app
desktop/src/          main/ (Electron main process), preload/, renderer/ (React)
packages/             api-client, ui, markdown — shared by web and desktop
monitoring/           Prometheus and Grafana config
docs/                 API reference, settings reference, ingestion design
```

[ARCHITECTURE.md](ARCHITECTURE.md) explains how the pieces fit.

## Running locally

### Everything in Docker

```bash
cp .env.example .env
docker compose up -d --build
```

Then load repositories once — see *Load repositories* in [README.md](README.md#2-load-repositories).

### Services natively, databases in Docker

```bash
docker compose up -d postgres redis

cd backend
export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"
go run ./cmd/migrate                       # schema first

export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="<your token>"
go run ./cmd/api                           # :8000
go run ./cmd/ingestd                       # :8002, in another shell (optional)
```

### Web and desktop

`web`, `desktop` and `packages/*` are one npm workspace with one lockfile. Install **once, at
the repo root**, with `npm ci` (dependencies are pinned — don't `npm install`):

```bash
npm ci
cd web && npm run dev            # http://localhost:5173
cd desktop && npm run dev        # Electron with hot reload; API at http://localhost:8000
```

## Checks

These are what CI runs; please run the ones for the parts you touched.

```bash
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build
```

`-short` skips tests that need live GitHub. Tests must pass with Redis stopped.

## Branches and commits

- Branch from `main`: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`. (Use
  `chore/` for refactors — an existing branch named `refactor` blocks `refactor/<topic>`.)
- [Conventional Commits](https://www.conventionalcommits.org/): `feat(search): …`,
  `fix(desktop): …`, `perf(ingestd): …`, `docs: …`.
- One concern per PR. Say what changed, why, and how you tested it. `main` only changes through
  reviewed PRs, and every merge to `main` deploys to production.

## Rules that reviews will hold you to

**Go**
- Never discard an error. Log it, return it, or count it — a failure nobody can see is a bug.
- Thread `ctx` into every database, Redis and HTTP call; use the shared `pgxpool`.
- No operational literals (limits, intervals, sizes, worker counts). Add a live setting
  instead — [docs/settings.md](docs/settings.md#adding-a-setting) shows how.
- Never log a token, or any part or length of one.
- Writes to GitHub (fork, push, PR) use the user's own token, never the server's.

**Database**
- Migrations live in `backend/internal/migrate/migrations/` and are append-only: never edit one
  that has been applied anywhere. Add a new numbered file with `-- +goose Up` and
  `-- +goose Down`, idempotent (`IF NOT EXISTS`).
- Issue identity is `(repository_id, number)`. Unlabelled issues are `UNCLASSIFIED`, never
  `BEGINNER`.

**Web and desktop**
- No `any`; declare types for API payloads (they belong in `packages/api-client`).
- Render issue bodies only through `@skillissues/markdown` — they're attacker-controlled.
- Shared UI goes in `packages/ui`; platform differences come in as props, not forked copies.
- Desktop: keep `contextIsolation`, `sandbox` and the narrow preload bridge; run git
  asynchronously so the UI never blocks.

## Code of Conduct

Please read the [Code of Conduct](CODE_OF_CONDUCT.md). Be kind and assume good intent.
