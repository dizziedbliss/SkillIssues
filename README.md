# SkillIssues

> **You have the skills. We have the issues.**

SkillIssues collects open issues from real GitHub repositories, sorts them by difficulty,
language and skills, and helps you go from "what should I work on?" to a merged pull request —
with XP, levels and badges awarded only after GitHub confirms the merge.

[Architecture](ARCHITECTURE.md) · [Contributing](CONTRIBUTING.md) · [API](docs/api.md) · [Settings](docs/settings.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

---

## What it does

- **Collects issues** from about 288k eligible public repositories (at least 25 stars, pushed
  within two years) — open issues only, most recently updated first.
- **Classifies difficulty from labels.** `good first issue`, `difficulty/easy`, `size/xs` and
  similar become `BEGINNER`; medium and hard labels become `INTERMEDIATE` and `ADVANCED`. An
  issue with no difficulty label is `UNCLASSIFIED` — unknown, not easy.
- **Search and filter** by repository, title, language/technology and author. Search results
  are balanced across those categories so one can't drown out the rest.
- **Recommends issues** from your profile: preferred languages, skills, interests and target
  level.
- **Sets up a workspace** from the desktop app: forks with your own GitHub account, clones into
  your workspace folder, creates a branch, pushes, and opens VS Code.
- **Verifies the pull request** on GitHub — merged, and authored by you — before awarding XP.

### XP and levels

| Difficulty | Base XP |
| :--- | ---: |
| `BEGINNER` | 75 |
| `INTERMEDIATE` | 140 |
| `ADVANCED` | 250 |

Every merged PR adds **+25**, **+15** for your first or second merge in that repository, and
**+10** if the issue spans more than one skill or technology — so a beginner issue can be worth
up to 125. Levels get steadily more expensive: level 2 at 300 XP, 3 at 750, 4 at 1,350, 5 at
2,100 (each level costs 150 more than the last). Badges: *First Blood* (1 merge), *Bug Wrangler*
(3), *Touch Grass* (5), *Polyglot* (2 languages), *World Traveller* (2 repositories).

---

## Repository layout

```text
SkillIssues/
├── backend/        Go: cmd/{api,ingestd,flowd,migrate,repoimport,healthcheck}, internal/*
│                   schema = backend/internal/migrate/migrations (goose, embedded)
├── web/            React 19 + Vite + TypeScript web app
├── desktop/        Electron 44 app (React + TypeScript renderer)
├── packages/       shared npm workspaces: api-client, ui, markdown
├── monitoring/     Prometheus config, Grafana provisioning and dashboards
├── scripts/        deploy-remote.sh (run on the VPS), backup-db.sh
├── systemd/        daily backup timer for the VPS
├── benchmark/      Python v6 ingestion prototype and its benchmark data (historical)
├── docs/           API reference, live settings, ingestion design
├── compose.yaml        local stack (builds from source)
├── compose.prod.yaml   production stack (pulls images from GHCR)
├── Caddyfile           production reverse proxy / TLS
└── Dockerfile          one file, targets: api, ingestd, flowd, migrate, web, repoimport
```

---

## Getting started

### Prerequisites

- Docker with Compose v2
- Go 1.26, Node.js 22 and npm 11 (only for running services natively or working on the clients)
- A GitHub token for `GITHUB_TOKEN` — the crawler only reads public data, so a token with no
  scopes is enough

### 1. Start the stack

```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env          # set GITHUB_TOKEN; everything else has a dev default
docker compose up -d --build
docker compose ps             # migrate and metrics-role exit 0; everything else running
```

### 2. Load repositories

The catalog starts empty. Repositories come from the Kaggle dataset
*github-repository-metadata-with-5-stars* (`repo_metadata.json`, ~3 GB). Put it at
`data/repo_metadata.json`, then:

```bash
cd backend
DATABASE_URL=postgresql://skillissues:skillissues@localhost:5432/skillissues \
  go run ./cmd/repoimport --file ../data/repo_metadata.json
```

Only eligible repositories are stored. A full pass takes about 16 minutes and is safe to re-run.
`ingestd` then starts crawling their issues on its own.

### 3. Open it

| What | URL |
|---|---|
| Web app | http://localhost:5173 |
| API | http://localhost:8000 (`/health`, `/metrics`) |
| ingestd | http://localhost:8002/sync/status |
| flowd (live flow stream) | `curl -N 'http://localhost:8003/stream?format=ndjson'` |
| Grafana | http://localhost:3000 — `admin` / `admin` unless `GRAFANA_ADMIN_PASSWORD` is set |
| Prometheus | http://localhost:9090 |

```bash
docker compose logs -f go-api ingestd
```

### Desktop app

```bash
npm ci              # once, at the repo root — web, desktop and packages share one lockfile
cd desktop
npm run dev         # expects the API at http://localhost:8000
```

### Checks (what CI runs)

```bash
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build
```

More detail — native (non-Docker) development, branch and commit conventions — is in
[CONTRIBUTING.md](CONTRIBUTING.md).

---

## Contributing

Bug reports, fixes, docs, tests and accessibility improvements are all welcome. Read
[CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md), then open an
issue or a pull request.

When reporting a bug, include what you expected, what happened, steps to reproduce, logs with
secrets removed, and your OS and tool versions.

## License

[MIT](LICENSE).
