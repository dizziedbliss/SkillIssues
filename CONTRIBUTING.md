# Contributing to SkillIssues

Thank you for your interest in contributing to **SkillIssues**! We welcome contributions from engineers and creators of all skill levels—whether you are implementing new features, optimizing database queries, improving UI/UX, refining algorithms, or writing documentation.

This guide details everything you need to know to set up your local development environment, understand our architecture, write clean and tested code, and submit a pull request.

---

## 📑 Table of Contents

- [🛠️ Developer Prerequisites](#️-developer-prerequisites)
- [📁 Repository Structure](#-repository-structure)
- [💻 Local Development Guide](#-local-development-guide)
  - [Option A: Full Stack via Docker Compose (Recommended)](#option-a-full-stack-via-docker-compose-recommended)
  - [Option B: Hybrid / Native Local Development](#option-b-hybrid--native-local-development)
  - [Running the Electron Desktop App](#running-the-electron-desktop-app)
- [🧪 Testing & Quality Assurance](#-testing--quality-assurance)
  - [Go Backend Tests](#go-backend-tests)
  - [Frontend Web Verification](#frontend-web-verification)
  - [Electron Desktop Verification](#electron-desktop-verification)
- [🌿 Branching & Commit Conventions](#-branching--commit-conventions)
- [🚀 Submitting a Pull Request](#-submitting-a-pull-request)
- [📐 Code & Architectural Guidelines](#-code--architectural-guidelines)
  - [Go Backend Standards](#go-backend-standards)
  - [React & TypeScript Standards](#react--typescript-standards)
  - [Electron IPC & Security Standards](#electron-ipc--security-standards)
  - [Database Migration Rules](#database-migration-rules)
- [📄 Code of Conduct](#-code-of-conduct)

---

## 🛠️ Developer Prerequisites

Ensure you have the following installed locally:

- **Git** (v2.30+)
- **Go** (v1.23+)
- **Node.js** (v20+ LTS) & **npm**
- **Docker Desktop** (v24+ with Docker Compose enabled)
- A GitHub Account & [Personal Access Token (PAT)](https://github.com/settings/tokens) with `read:user` and `repo` scopes

---

## 📁 Repository Structure

```text
SkillIssues/
├── backend/                     # Pure Go microservices & business logic
│   ├── cmd/
│   │   ├── api/                 # Primary API Gateway (:8000)
│   │   ├── syncd/               # Sync Coordinator & GitHub GraphQL engine (:8002)
│   │   ├── worker/              # Priority catalog refresh daemon
│   │   ├── kaggle_etl/          # Standalone offline dataset ingestion CLI
│   │   └── healthcheck/         # Static Distroless health probe
│   ├── internal/                # Shared internal domain packages
│   │   ├── auth/                # OAuth, PAT, Session & Account Linking
│   │   ├── cache/               # Redis caching layer
│   │   ├── contribution/        # PR verification engine
│   │   ├── db/                  # pgxpool PostgreSQL adapter
│   │   ├── github/              # GitHub REST/GraphQL client
│   │   ├── issue/               # Multi-category partitioned search
│   │   ├── metrics/             # Prometheus telemetry
│   │   ├── middleware/          # Rate limiting, CORS, logging
│   │   ├── profile/             # User profile & preferences
│   │   ├── progression/         # XP, level, and badge calculation
│   │   ├── recommendation/      # Deterministic issue matcher
│   │   └── sync/                # Sync algorithms & worker pools
│   └── integration_test.go      # Go API & integration tests
├── desktop/                     # Desktop companion application (Electron)
│   ├── main.js                  # Frameless window, Git engine, IPC handlers
│   ├── preload.js               # Context-isolated secure bridge
│   ├── renderer.html            # Native desktop markup
│   ├── renderer.js              # UI controller, categorized search, pagination
│   └── renderer.css             # Himalayas/lavender frameless styling
├── web/                         # Web application (React 18 + Vite + TypeScript)
│   ├── src/
│   │   ├── LandingView.tsx      # Promotional 3D pipeline flowchart & hero
│   │   ├── main.tsx             # App root, catalog feed, modal & routing
│   │   └── styles.css           # Design system (lavender/deep purple theme)
│   ├── vite.config.ts           # Vite bundler configuration
│   └── Dockerfile               # Production container definition
├── database/                    # Database baseline schemas & migrations
│   ├── schema.sql               # Authoritative PostgreSQL schema
│   └── migrations/              # Incremental database migrations (001 - 006)
├── data/                        # Datasets & offline catalog archives
├── k8s/                         # Kubernetes deployment manifests
├── compose.yaml                 # Core development Docker Compose stack
├── docker-compose.prod.yml      # Production stack with Nginx proxy
├── ARCHITECTURE.md              # Comprehensive technical design document
├── DEPLOYMENT.md                # Production deployment guide
└── README.md                    # Project overview & quickstart
```

---

## 💻 Local Development Guide

### Option A: Full Stack via Docker Compose (Recommended)

The fastest way to spin up the entire system is with Docker Compose:

```bash
# 1. Clone your fork
git clone https://github.com/<your-username>/SkillIssues.git
cd SkillIssues

# 2. Configure environment
cp .env.example .env

# 3. Start all services
docker compose up -d --build
```

**Service URLs:**
- 🌐 **Web Dashboard**: [http://localhost:5173](http://localhost:5173)
- 🔌 **API Gateway**: [http://localhost:8000](http://localhost:8000)
- 📊 **Prometheus Metrics**: [http://localhost:8000/metrics](http://localhost:8000/metrics)
- 🔄 **Sync Coordinator**: [http://localhost:8002](http://localhost:8002)

To view real-time logs:
```bash
docker compose logs -f go-api syncd
```

---

### Option B: Hybrid / Native Local Development

If you are developing Go backend logic or React frontend components and prefer rapid hot-reloading:

#### 1. Start Infrastructure Dependencies
Start only PostgreSQL and Redis in Docker:
```bash
docker compose up -d postgres redis
```

#### 2. Run Go Services Locally
In one terminal, launch the Go API Gateway:
```bash
cd backend
export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"
export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="ghp_your_token_here"
export PORT=8000
go run ./cmd/api
```

In another terminal, optionally run `syncd`:
```bash
cd backend
export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"
export REDIS_URL="redis://localhost:6379/0"
export GITHUB_TOKEN="ghp_your_token_here"
export PORT=8002
go run ./cmd/syncd
```

#### 3. Run Web App Locally
In a third terminal:
```bash
cd web
npm install
npm run dev
```

---

### Running the Desktop App

The Electron desktop client communicates with the running API Gateway (`http://localhost:8000`):

```bash
cd desktop
npm install
npm start
```

---

## 🧪 Testing & Quality Assurance

Before submitting any pull request, make sure all test suites pass locally.

### Go Backend Tests
Run the Go unit, integration, and contract tests:
```bash
cd backend
go test -v ./...
```

To format Go code according to Go standards:
```bash
cd backend
go fmt ./...
go vet ./...
```

### Frontend Web Verification
Ensure TypeScript compilation and Vite production bundle pass with zero errors:
```bash
cd web
npm run build
```

### Desktop App Verification
Verify Electron JavaScript syntax:
```bash
cd desktop
npm run check
```

---

## 🌿 Branching & Commit Conventions

### Branch Naming Conventions
Always create descriptive feature or bugfix branches off `main` or the active feature branch:
- `feat/add-new-search-filter`
- `fix/electron-clone-buffer-overflow`
- `perf/sync-worker-batching`
- `docs/update-architecture-flow`

### Commit Message Guidelines (Conventional Commits)
We enforce clean, standardized commit messages:
```text
<type>(<scope>): <subject>

[optional body]
```

**Common types:**
- `feat`: A new user-facing feature or capability.
- `fix`: A bug fix.
- `docs`: Documentation changes only.
- `refactor`: Code change that neither fixes a bug nor adds a feature.
- `perf`: A code change that improves performance.
- `test`: Adding missing tests or correcting existing tests.
- `chore`: Changes to build process, dependency updates, or tooling.

*Example:*
```bash
git commit -m "feat(issue): add window CTE partitioned multi-category search"
```

---

## 🚀 Submitting a Pull Request

1. **Keep Pull Requests Focused**: Limit each PR to a single feature, bug fix, or refactoring goal. Avoid combining unrelated changes.
2. **Push to Your Fork**:
   ```bash
   git push origin feat/your-feature-name
   ```
3. **Open a PR**: Open a pull request against the `main` (or `refactor`) branch on `SkillIssues`.
4. **Describe Your Changes**:
   - What problem does this PR solve?
   - What changes were made?
   - How was this verified? (Include automated test outputs, screenshots, or logs).
5. **Review & Iterate**: Address review feedback promptly and keep discussions constructive.

---

## 📐 Code & Architectural Guidelines

### Go Backend Standards
- **Idiomatic Go**: Follow standard Go naming conventions and error handling patterns (`if err != nil`).
- **Context Propagation**: Always pass `context.Context` through to database calls, HTTP requests, and Redis commands.
- **Connection Pooling**: Use the shared `*pgxpool.Pool` instance; never open direct unpooled database connections.
- **Graceful Error Responses**: Always return structured JSON errors (`{"error": "description"}`) with appropriate HTTP status codes.

### React & TypeScript Standards
- **Strict Typing**: Avoid `any`. Define explicit TypeScript types or interfaces for API response shapes and component props.
- **Design System Cohesion**: Strictly adhere to the platform's whitish-lavender / purple palette (`#f7f6fb`, `#1e1338`, `#241442`, `#7c3aed`). Do not introduce arbitrary uncoordinated colors.
- **Responsive & Accessible**: Ensure layouts render cleanly across standard desktop resolutions and tablet/mobile viewports.

### Electron IPC & Security Standards
- **Context Isolation**: Maintain strict separation between main and renderer processes using `preload.js` and `contextBridge.exposeInMainWorld`.
- **Input Sanitization**: Validate all file paths, repository URLs, and branch names in the main process before executing Git commands.
- **Non-blocking Operations**: Never block the Electron UI thread with synchronous shell calls (`execSync`); always use streamed async child processes.

### Database Migration Rules
- **Never Modify Existing Migrations**: Once a migration has been committed, never edit it.
- **Create Incremental Migrations**: Add new numbered migrations in `database/migrations/` with paired `<number>_<name>.up.sql` and `<number>_<name>.down.sql` files.
- **Safe DDL**: Ensure migrations use `IF NOT EXISTS` or idempotent constraints where applicable.

---

## 📄 Code of Conduct

SkillIssues is an inclusive, welcoming community. We are dedicated to providing a harassment-free experience for everyone, regardless of background, gender identity, race, or experience level. Be respectful, constructive, and supportive of fellow contributors.
