# Contributing to SkillIssues

> **First off: thank you for being here.** Whether you found a typo in the docs, a nasty race condition in the sync worker, or you just want to make the desktop app feel 10% less cursed — we are thrilled to have your help.

SkillIssues is built for developers of all stripes: folks taking their very first steps into open-source, full-stack builders, and grizzled veterans who remember compiling kernels while drinking black coffee. Nobody loads a 20,000-line codebase into their head on day one. Ask questions, have fun, and let us build something awesome together.

---

## 📑 Table of Contents

- [The Short Version](#the-short-version)
- [🛠️ What You Will Need](#️-what-you-will-need)
- [📁 What Lives Where](#-what-lives-where)
- [💻 Running Locally](#-running-locally)
  - [Option 1: The "I Just Want It to Run" Docker Way (Recommended)](#option-1-the-i-just-want-it-to-run-docker-way-recommended)
  - [Option 2: The Fast-Reload Hybrid Way (Go / React Developers)](#option-2-the-fast-reload-hybrid-way-go--react-developers)
  - [Firing Up the Desktop App (Electron)](#firing-up-the-desktop-app-electron)
- [🧪 Running Checks & Tests](#-running-checks--tests)
- [🌿 Branching & Commits](#-branching--commits)
- [🚀 Submitting Your Pull Request](#-submitting-your-pull-request)
- [📐 Code Guidelines (Without the Bureaucracy)](#-code-guidelines-without-the-bureaucracy)
  - [Go Backend](#go-backend)
  - [React & TypeScript](#react--typescript)
  - [Electron Desktop](#electron-desktop)
  - [Database & Migrations](#database--migrations)
- [📜 Code of Conduct](#-code-of-conduct)

---

## The Short Version

1. Fork the repo and create a feature branch off `main` (or `refactor`).
2. Run tests and make sure your code builds cleanly.
3. Open a PR with a clear description of what you did and why.
4. Don't worry about being perfect — we review with empathy and high fives.

---

## 🛠️ What You Will Need

Before you start wrestling with code, make sure you have:

- [Git](https://git-scm.com/) (v2.30+)
- [Go](https://go.dev/) (v1.23+)
- [Node.js](https://nodejs.org/) (v20+ LTS) & `npm`
- [Docker Desktop](https://www.docker.com/) (v24+ with Docker Compose)
- A GitHub account and a [Personal Access Token (PAT)](https://github.com/settings/tokens) with `read:user` and `repo` scopes (needed if you want to test authenticated flows or PR verification).

---

## 📁 What Lives Where

```text
SkillIssues/
├── backend/          # Go services (API gateway, ingestion engine, migrations)
│   ├── cmd/          # Entrypoints: /api, /ingestd, /migrate, /healthcheck
│   └── internal/     # Core domain packages: auth, db, issue search, progression, etc.
├── web/              # React 18 + Vite + TypeScript dashboard
│   └── src/          # Components, styles, 3D landing hero, and catalog views
├── desktop/          # Electron companion app (native frameless window + Git engine)
├── database/         # PostgreSQL schema and numbered migrations (001 -> 006)
├── data/             # Datasets & offline metadata archives
├── k8s/              # Production Kubernetes manifests (api, ingestd, web)
├── compose.yaml      # Docker Compose stack for local development
├── ARCHITECTURE.md   # System deep-dive and component diagrams
└── CONTRIBUTING.md   # You are here!
```

---

## 💻 Running Locally

### Option 1: The "I Just Want It to Run" Docker Way (Recommended)

If you don't feel like setting up local Go compilers or configuring PostgreSQL services on your host machine:

```bash
# 1. Clone your fork
git clone https://github.com/<your-username>/SkillIssues.git
cd SkillIssues

# 2. Grab the environment template
cp .env.example .env

# 3. Spin up the whole universe
docker compose up -d --build
```

Give it a moment, then check:
```bash
docker compose ps
```

Everything will be live at:
- 🌐 **Web Dashboard**: [http://localhost:5173](http://localhost:5173)
- 🔌 **API Gateway**: [http://localhost:8000](http://localhost:8000)
- 📊 **Prometheus Metrics**: [http://localhost:8000/metrics](http://localhost:8000/metrics)
- 🔄 **Sync Coordinator**: [http://localhost:8002](http://localhost:8002)

To tail service logs in real time:
```bash
docker compose logs -f go-api ingestd
```

---

### Option 2: The Fast-Reload Hybrid Way (Go / React Developers)

If you are actively hacking on backend Go logic or tweaking frontend React UI and want instantaneous hot-reloads:

#### 1. Start the databases in Docker:
```bash
docker compose up -d postgres redis
```

#### 2. Run the Go API Gateway natively:
```bash
cd backend
export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"
export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="ghp_your_token_here"
export PORT=8000
go run ./cmd/api
```

#### 3. Run the Web frontend natively:
```bash
cd web
npm install
npm run dev
```

---

### Firing Up the Desktop App (Electron)

The desktop app talks to your running API Gateway on `http://localhost:8000`:

```bash
cd desktop
npm install
npm start
```

---

## 🧪 Running Checks & Tests

Before pushing changes, run these quick sanity checks so CI doesn't yell at you:

### Backend (Go)
```bash
cd backend
go test -v ./...
go fmt ./...
go vet ./...
```

### Web App (React / TypeScript)
```bash
cd web
npm run build
```

### Desktop App (Electron)
```bash
cd desktop
npm run check
```

---

## 🌿 Branching & Commits

### Branch Names
Keep them simple and descriptive:
- `feat/add-author-filter`
- `fix/desktop-clone-buffer`
- `docs/clarify-env-vars`
- `refactor/clean-search-query`

### Commit Messages
We like [Conventional Commits](https://www.conventionalcommits.org/) because they make the release history easy to read:
- `feat(search): add category accordion counts`
- `fix(electron): handle dynamic default branch checkout`
- `docs: update quickstart instructions`
- `perf(ingestd): optimize worker queue drainage`

---

## 🚀 Submitting Your Pull Request

1. **Keep it focused**: One PR per logical concern. It is ten times easier to review two 100-line PRs than one 2,000-line mystery box.
2. **Push to your fork**:
   ```bash
   git push origin feat/your-feature-name
   ```
3. **Open the PR**: Pick our `main` (or active development) branch.
4. **Tell us what you built**:
   - What changed?
   - Why did you choose this approach?
   - How did you test it? (Terminal output or screenshots are awesome).
5. **Review time**: We review with respect and constructive suggestions. If we ask questions, it's not a critique of you as a human — we're just making sure the code works reliably in production.

---

## 📐 Code Guidelines (Without the Bureaucracy)

### Go Backend
- **Keep it idiomatic**: If Go has a natural way to do something, prefer it.
- **Handle your errors**: Check `if err != nil` and return descriptive context (`fmt.Errorf("reading issue %d: %w", id, err)`).
- **Pass contexts**: Always pass `ctx context.Context` into database queries, Redis calls, and outgoing HTTP calls.
- **Pooled connections**: Use the shared `*pgxpool.Pool` — don't invent your own connections.

### React & TypeScript
- **TypeScript matters**: Avoid `any` when possible. Declare clean interfaces for API payloads.
- **Stay on theme**: We use a signature whitish-lavender and deep violet palette (`#f7f6fb`, `#1e1338`, `#241442`, `#7c3aed`). Let's keep it visually cohesive!
- **Accessibility & feel**: Modals should close on `Esc` or outside clicks, and inputs should have helpful placeholders.

### Electron Desktop
- **Safety first**: Keep `contextIsolation: true` in preload. Never expose dangerous Node modules directly to the browser window.
- **Never block the UI**: Stream Git operations asynchronously so the window never stutters or freezes during large repository clones.

### Database & Migrations
- **Where they live**: `backend/internal/migrate/migrations/`, embedded into the binaries with
  `go:embed`. This is the single source of truth for the schema — there is no separate
  `schema.sql`.
- **Never rewrite history**: Once a migration has been applied anywhere, treat it as frozen.
  Add a new one instead.
- **Use goose annotations**: Each file carries `-- +goose Up` and `-- +goose Down` sections.
- **Be idempotent**: Use `IF NOT EXISTS` where appropriate.
- **Applying them**: `go run ./cmd/migrate`, or the `migrate` service that Docker Compose runs
  to completion before `go-api` and `ingestd` start.

---

## 📜 Code of Conduct

We are committed to making SkillIssues a welcoming, safe, and friendly space for everyone. Please read our [Code of Conduct](CODE_OF_CONDUCT.md).

Be kind, assume good intent, and let's build something helpful!
