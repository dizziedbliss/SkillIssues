# SkillIssues

<div align="center">

<img src="frontend/public/logo.png" alt="SkillIssues Logo" width="120" style="border-radius: 24px; box-shadow: 0 8px 24px rgba(36, 20, 66, 0.15);" />

### *You Have the Skills. We Have the Issues.*

[![License: MIT](https://img.shields.io/badge/License-MIT-purple.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg?logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6.svg?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![Electron](https://img.shields.io/badge/Electron-30-47848F.svg?logo=electron&logoColor=white)](https://www.electronjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791.svg?logo=postgresql&logoColor=white)](https://postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D.svg?logo=redis&logoColor=white)](https://redis.io)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED.svg?logo=docker&logoColor=white)](https://docker.com)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5.svg?logo=kubernetes&logoColor=white)](https://kubernetes.io)

**SkillIssues** is an open-source developer-growth platform engineered to turn authentic GitHub issues into a gamified, skill-building journey. By categorizing open-source issues, scoring their difficulty, and matching them against developer preferences, SkillIssues streamlines the entire contribution lifecycle—from discovery, sandboxed local repository forking, and automated branch setup, to pull request verification and verifiable XP progression.

[Architecture Guide](ARCHITECTURE.md) • [Deployment Guide](DEPLOYMENT.md) • [Contributing Guide](CONTRIBUTING.md)

</div>

---

## 📑 Table of Contents

- [🌟 Why SkillIssues?](#-why-skillissues)
- [🔄 The End-to-End Contribution Lifecycle](#-the-end-to-end-contribution-lifecycle)
- [🏗️ Architectural Overview](#️-architectural-overview)
- [✨ Core Implemented Features](#-core-implemented-features)
  - [1. Pure Go High-Performance Microservices](#1-pure-go-high-performance-microservices)
  - [2. Multi-Category Interleaved Search & Ranking](#2-multi-category-interleaved-search--ranking)
  - [3. Dual Authentication & Unified Account Linking](#3-dual-authentication--unified-account-linking)
  - [4. Desktop Companion App (Electron)](#4-desktop-companion-app-electron)
  - [5. Interactive 3D Web Dashboard & Landing Page](#5-interactive-3d-web-dashboard--landing-page)
  - [6. Gamification & Progression Engine](#6-gamification--progression-engine)
  - [7. Ingestion Engine & Kaggle ETL Pipeline](#7-ingestion-engine--kaggle-etl-pipeline)
  - [8. Rate Limiting, Caching & Observability](#8-rate-limiting-caching--observability)
- [📁 Repository Structure](#-repository-structure)
- [⚡ Getting Started](#-getting-started)
  - [Prerequisites](#prerequisites)
  - [1. Clone & Configure Environment](#1-clone--configure-environment)
  - [2. Launch Docker Services](#2-launch-docker-services)
  - [3. Launch Desktop Client](#3-launch-desktop-client)
- [⚙️ Environment Configuration](#️-environment-configuration)
- [📡 API Reference](#-api-reference)
- [📦 Kubernetes & Production Readiness](#-kubernetes--production-readiness)
- [🧪 Testing & Quality Assurance](#-testing--quality-assurance)
- [📄 License](#-license)

---

## 🌟 Why SkillIssues?

- **Real Codebases, Zero Toy Problems**: Skip artificial coding challenges. Contribute to top-tier open-source projects (FastAPI, React, Vite, Kubernetes, CPython, Go, etc.).
- **Deterministic Difficulty Scoring**: AI/heuristic classifier scores issues into `BEGINNER` (75 XP), `INTERMEDIATE` (140 XP), or `ADVANCED` (250 XP) using comment density, issue description depth, required skills, and repository health.
- **Frictionless Workspace Setup**: Desktop app automates GitHub upstream forking, local cloning into custom user-selected paths, dynamic default branch tracking, and VS Code launching with 1 click.
- **Server-Side PR Verification**: Verifies PR author attribution and merge status against the GitHub REST/GraphQL API to award verified XP and badges.
- **Unified Identity**: Seamlessly link GitHub Personal Access Tokens (PAT) and Browser OAuth sessions into a single developer account.
- **Ultra-Fast Pure Go Backend**: Fully rewritten in Go 1.23 with sub-millisecond response times, connection pooling (`pgxpool`), Redis caching, and Prometheus observability.

---

## 🔄 The End-to-End Contribution Lifecycle

```text
  ┌────────────────────────────────────────────────────────┐
  │                 1. Issue Mining & Sync                 │
  │   GitHub GraphQL API / Kaggle ETL ──> syncd coordinator│
  │   Calculates difficulty score, labels, and tech stack  │
  └───────────────────────────┬────────────────────────────┘
                              │
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │               2. Discovery & Search                    │
  │   Interleaved multi-category search: Repos, Languages, │
  │   Issues, Authors; Filter by Difficulty & Preferences  │
  └───────────────────────────┬────────────────────────────┘
                              │
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │               3. Workspace Initialization              │
  │   Desktop client forks repo to user's GitHub account   │
  │   Clones to chosen folder, sets upstream, creates      │
  │   feature branch: skillissues/issue-<id>               │
  └───────────────────────────┬────────────────────────────┘
                              │
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │                  4. Code & Submit                      │
  │   1-click "Open in VS Code" -> Solve bug -> Push to    │
  │   origin -> Open Pull Request on upstream repository   │
  └───────────────────────────┬────────────────────────────┘
                              │
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │             5. Automated PR Verification               │
  │   Go backend inspects GitHub PR merge status & author  │
  │   attribution; Awards XP, updates level & unlocks badge│
  └────────────────────────────────────────────────────────┘
```

---

## 🏗️ Architectural Overview

SkillIssues utilizes a high-throughput microservices architecture orchestrated via Docker Compose and Kubernetes:

```
                      ┌─────────────────────────────────────────┐
                      │             Clients & UI                │
                      │  React 18 + Vite Web App (:5173)        │
                      │  Electron Desktop App (Frameless Native)│
                      └────────────────────┬────────────────────┘
                                           │
                           HTTP / REST (Port 8000)
                                           │
                                           ▼
                      ┌─────────────────────────────────────────┐
                      │          Go API Gateway (:8000)         │
                      │  • Chi Router & Middleware Stack        │
                      │  • Sliding-Window Rate Limiter          │
                      │  • Multi-Category Search Engine         │
                      │  • Auth, Progression & PR Verification  │
                      └────────────┬──────────────┬─────────────┘
                                   │              │
           ┌───────────────────────┘              └────────────────────────┐
           ▼                                                               ▼
┌─────────────────────┐                                         ┌─────────────────────┐
│  PostgreSQL 16 DB   │                                         │    Redis 7 Cache    │
│  (:5432)            │                                         │    (:6379)          │
│  Schema Migrations  │                                         │  • Session Store    │
│  001 -> 006         │                                         │  • Rate Limits      │
└──────────▲──────────┘                                         │  • Hot Queries      │
           │                                                    └──────────▲──────────┘
           │                                                               │
           ├───────────────────────────────┐                               │
           ▼                               ▼                               ▼
┌─────────────────────┐         ┌─────────────────────┐         ┌─────────────────────┐
│    Go syncd Daemon  │         │   Go Worker Daemon  │         │  Kaggle ETL Offline │
│    (:8002)          │◄───────►│   (Background Sync) │         │  CLI Pipeline       │
│  • GitHub GraphQL   │         │  • Priority Catalog │         │  • Bulk Ingestion   │
│  • Rate-Aware Sync  │         │    Refresher        │         │  • Sanitization     │
└─────────────────────┘         └─────────────────────┘         └─────────────────────┘
```

---

## ✨ Core Implemented Features

### 1. Pure Go High-Performance Microservices
- **`go-api` (Port 8000)**: Serves 28 REST endpoints for authentication, profile management, issue exploration, recommendations, saved bookmarks, contributions, admin tasks, and `/metrics`.
- **`syncd` (Port 8002)**: Concurrent GitHub GraphQL synchronization daemon. Manages worker pools, respects GitHub secondary rate limits, qualifies valid issues, extracts required skills, and computes difficulty metrics.
- **`worker`**: Background polling worker ensuring high-priority active challenges and upstream repositories remain fresh.
- **Distroless Runtime**: Production images run on Google Distroless Debian 12 non-root containers with custom static `/healthcheck` binaries for zero-vulnerability container footprints.

### 2. Multi-Category Interleaved Search & Ranking
- **GitHub-Style Categorized Results**: Searches partition output across 4 balanced categories:
  - 📁 **Repositories**: Prioritizes official and high-star exact/prefix matches (e.g. `golang/go`, `python/cpython`).
  - 💻 **Languages & Frameworks**: Matches specific language ecosystems.
  - 📋 **Issues**: Searches issue titles and descriptions.
  - 👤 **Authors & Organizations**: Matches issue creators and upstream orgs.
- **Window CTE Partitioning**: Employs PostgreSQL `ROW_NUMBER() OVER (PARTITION BY match_category)` to guarantee balanced representation rather than a single category dominating results.
- **Interactive UI Accordion**: Top 3 items per category displayed under `All`, expandable via `"Show more in {Category}"` with live category counts.

### 3. Dual Authentication & Unified Account Linking
- **Two Auth Modalities**:
  - **Browser GitHub OAuth**: Initiates OAuth in external browser, handles redirect callback, and completes handshake.
  - **Personal Access Token (PAT)**: Direct input with `read:user` and `repo` scopes.
- **Unified Identity (No Duplicate Accounts)**:
  - Database migration `006_link_users_by_username_and_github_id.up.sql` enforces case-insensitive `lower(username)` indexing.
  - `UpsertUser` matches existing accounts by either `github_id` OR `lower(username)`, preserving user XP, levels, and badges regardless of which login method is used.
- **Themed OAuth Flow**: Custom-designed callback pages matching the application's whitish-lavender theme (`#f7f6fb` / `#241442`).

### 4. Desktop Companion App (Electron)
- **Frameless / Borderless Window**: Custom sleek titlebar with minimize, maximize, and close controls styled with the platform's signature theme.
- **Flexible Workspace Directory**: Remembers the developer's preferred parent directory for cloned repositories and allows changing it anytime.
- **Sandboxed Git Engine**:
  - Automatically forks repositories to user's GitHub profile.
  - Non-blocking clone progress bar designed to withstand massive repositories without buffer overflow or IPC freezing.
  - Dynamically discovers upstream default branch (`main`, `master`, `develop`, etc.) instead of hardcoding assumptions.
  - Automatically configures `origin` and `upstream` remotes and creates the feature branch: `skillissues/issue-<id>`.
  - 1-click **Open in VS Code** (`code .`).
- **Workspace Drawer**: Redesigned status drawer with expandable build/clone logs and clear execution steps.
- **Multi-Submission Support**: Allows re-attempting or submitting multiple distinct pull requests for the same challenge.

### 5. Interactive 3D Web Dashboard & Landing Page
- **Promotional 3D Pipeline Flowchart (`frontend/src/LandingView.tsx`)**:
  - CSS 3D perspective viewport (`perspective: 1000px`) with interactive mouse tilt and parallax cursor tracking.
  - 4 pipeline stages: **Issue Mining** ➔ **Personalized Match** ➔ **Fork & Code** ➔ **Verify & Level Up**.
  - Interactive step inspector card allowing users to preview live mock code, matching tags, terminal clone output, and XP award cards.
- **Intelligent Routing**:
  - Guests automatically land on the **Overview** (landing page).
  - Authenticated developers default to **Challenges** (issue feed).
  - Top navigation bar allows seamless switching between **Overview** and **Challenges**.
- **Issue Modal UX**:
  - Floating external Back button (`← Back to issues [Esc]`).
  - Backdrop dismissal and `Esc` key listener.
  - Scroll lock on background page with clean scroll-position reset between issues.
- **Profile Customizer**: Interactive technology search and chip picker matching desktop capabilities.

### 6. Gamification & Progression Engine
- **Difficulty & XP**:
  - `BEGINNER`: **75 XP**
  - `INTERMEDIATE`: **140 XP**
  - `ADVANCED`: **250 XP**
- **5-Tier Progression Ladder**:
  - Level 1: `Novice Contributor` (0 XP)
  - Level 2: `Junior Contributor` (250 XP)
  - Level 3: `Open Source Contributor` (750 XP)
  - Level 4: `Senior Contributor` (1,750 XP)
  - Level 5: `Staff Contributor` (3,500+ XP)
- **Badges**: Automated award system granting badges such as `First Blood` (first verified PR), `Bug Hunter`, `Polyglot`, `Maintainer's Ally`, and difficulty tier medals.

### 7. Ingestion Engine & Kaggle ETL Pipeline
- **Continuous Ingestion (`syncd`)**: Polls top repositories with GraphQL queries, stripping bot noise, categorizing required skills, and computing difficulty vectors.
- **Offline Kaggle ETL (`go/cmd/kaggle_etl`)**: High-throughput CLI pipeline to ingest offline Kaggle dataset archives directly into PostgreSQL in batches with deduplication.

### 8. Rate Limiting, Caching & Observability
- **Redis Sliding-Window Limiter**:
  - Unauthenticated guests: 180 req/hour (rate-limited gracefully, protecting the catalog).
  - Authenticated developers: 1,000 req/hour.
  - Health checks (`/health`) and Prometheus metrics (`/metrics`) exempt.
- **Observability**: Prometheus endpoint exposing request counters, latency percentiles, sync cycle gauges, and cache hit rates.

---

## 📁 Repository Structure

```text
SkillIssues/
├── .github/                     # CI/CD workflows (lint, test, build, release)
├── compose.yaml                 # Core development Docker Compose definition
├── docker-compose.prod.yml      # Production stack with Nginx SSL reverse proxy
├── database/                    # PostgreSQL schema definition
│   └── schema.sql               # Authoritative base schema
├── go/                          # Pure Go microservices & business logic
│   ├── cmd/
│   │   ├── api/                 # Primary API Gateway (Port 8000)
│   │   ├── syncd/               # GitHub GraphQL Sync Coordinator (Port 8002)
│   │   ├── worker/              # Priority catalog refresh daemon
│   │   ├── kaggle_etl/          # Standalone offline ETL CLI
│   │   └── healthcheck/         # Static Distroless health probe
│   ├── internal/
│   │   ├── auth/                # OAuth, PAT, Session & Account Linking
│   │   ├── cache/               # Redis caching layer
│   │   ├── config/              # Environment config loader
│   │   ├── contribution/        # PR verification engine
│   │   ├── db/                  # pgxpool PostgreSQL adapter
│   │   ├── github/              # GitHub REST/GraphQL client
│   │   ├── issue/               # Multi-category partitioned search
│   │   ├── metrics/             # Prometheus metrics collectors
│   │   ├── middleware/          # Rate limiting, CORS, request logging
│   │   ├── profile/             # Developer profiles & preferences
│   │   ├── progression/         # XP, level, and badge calculation
│   │   ├── recommendation/      # Deterministic issue matcher
│   │   └── sync/                # Sync algorithms & worker pools
│   ├── migrations/              # Database migration files (001 - 006)
│   └── integration_test.go      # Go integration & API tests
├── electron/                    # Desktop companion application
│   ├── main.js                  # Frameless window, Git engine, IPC handlers
│   ├── preload.js               # Context-isolated secure bridge
│   ├── renderer.html            # Native desktop markup
│   ├── renderer.js              # UI controller, categorized search, pagination
│   └── renderer.css             # Himalayas/lavender frameless styling
├── frontend/                    # Web application
│   ├── src/
│   │   ├── LandingView.tsx      # Promotional 3D pipeline flowchart & hero
│   │   ├── main.tsx             # App root, catalog feed, modal & routing
│   │   └── styles.css           # Design system (lavender/deep purple theme)
│   ├── vite.config.ts           # Vite bundler configuration
│   └── Dockerfile               # Node 22 Alpine production container
├── k8s/                         # Kubernetes deployment manifests
│   ├── api/                     # Go API Gateway deployment & service
│   ├── syncd/                   # Ingestion daemon deployment & service
│   ├── worker/                  # Catalog worker deployment
│   ├── frontend/                # Web UI deployment & service
│   ├── ingress.yaml             # Ingress rules with TLS routing
│   └── configmap.yaml           # Cluster configuration
└── tests/                       # Test suites
```

---

## ⚡ Getting Started

### Prerequisites
- [Docker Desktop](https://www.docker.com/) (version 24+) with Docker Compose
- [Node.js](https://nodejs.org/) v20+ & npm (required only if running Electron locally)
- A GitHub Account & [Personal Access Token](https://github.com/settings/tokens) with `read:user` and `repo` scopes (or GitHub OAuth App credentials)

### 1. Clone & Configure Environment
```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
```
Edit `.env` to supply your `GITHUB_TOKEN` and optional GitHub OAuth App keys (`GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`).

### 2. Launch Docker Services
Launch the entire containerized stack:
```bash
docker compose up -d --build
```

Verify service health:
```bash
docker compose ps
```

Access the interfaces:
- 🌐 **Web Application**: [http://localhost:5173](http://localhost:5173)
- 🔌 **API Gateway**: [http://localhost:8000](http://localhost:8000)
- 📊 **Prometheus Metrics**: [http://localhost:8000/metrics](http://localhost:8000/metrics)
- 🔄 **Sync Coordinator**: [http://localhost:8002](http://localhost:8002)

### 3. Launch Desktop Client
In a separate terminal:
```bash
cd electron
npm install
npm start
```

---

## ⚙️ Environment Configuration

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `PORT` | `8000` | HTTP port for the primary Go API Gateway. |
| `DATABASE_URL` | `postgresql://skillissues:skillissues@postgres:5432/skillissues` | PostgreSQL connection string. |
| `REDIS_URL` | `redis://redis:6379/0` | Redis instance for session state, caching, and rate limiting. |
| `FRONTEND_URL` | `http://localhost:5173` | Allowed CORS origin for the web dashboard. |
| `GITHUB_TOKEN` | `""` | GitHub token for catalog ingestion and PR verification. |
| `GITHUB_CLIENT_ID` | `""` | GitHub OAuth Application Client ID. |
| `GITHUB_CLIENT_SECRET` | `""` | GitHub OAuth Application Client Secret. |
| `GITHUB_REDIRECT_URI` | `http://localhost:8000/auth/github/callback` | OAuth callback target URL. |
| `ADMIN_SYNC_TOKEN` | `dev_local_admin_sync_token` | Bearer token for triggering manual `/admin/*` operations. |
| `UNAUTH_RATE_LIMIT`| `180` | Requests allowed per hour for unauthenticated clients. |
| `AUTH_RATE_LIMIT`  | `1000` | Requests allowed per hour for authenticated developers. |
| `SYNC_INTERVAL_SECONDS` | `900` | Sync coordinator background polling frequency (in seconds). |

---

## 📡 API Reference

### Health & Observability
- `GET /health` — Service readiness probe.
- `GET /metrics` — Prometheus metrics export.

### Authentication
- `GET /auth/github` — Initiates GitHub OAuth authorization flow.
- `GET /auth/github/callback` — Handles OAuth code exchange and session issuance.
- `POST /auth/github/token` — Authenticates directly via GitHub Personal Access Token (PAT).
- `POST /auth/logout` — Revokes active session and invalidates Redis cache.

### Developer Profile & Progression
- `GET /me` — Current user profile, target level, total XP, and unlocked badges.
- `PUT /profile` — Updates bio, preferred languages, and experience level.

### Issue Discovery & Search
- `GET /issues` — Paginated and categorized search (`search`, `category`, `language`, `difficulty`, `page`, `limit`).
- `GET /issues/{id}` — Detailed issue metadata, required skills, and upstream context.
- `GET /recommendations` — Scored challenge recommendations tailored to the developer's profile.

### Workspaces & Contributions
- `GET /saved` / `POST /saved` / `DELETE /saved/{id}` — Manage saved challenges.
- `GET /working` — Active local workspace contributions.
- `POST /contributions` — Records fork creation and workspace initialization.
- `GET /contributions` — Historical submission history.
- `POST /contributions/{id}/submit` — Submits PR URL and commit reference.
- `POST /contributions/{id}/verify` — Server-side verification of PR merge status & XP allocation.

### Administrative Controls
- `POST /admin/sync` — Forces priority sync cycle (requires `ADMIN_SYNC_TOKEN`).
- `POST /admin/repos` — Registers new upstream repository for catalog ingestion.

---

## 📦 Kubernetes & Production Readiness

Production Kubernetes manifests are located in `k8s/`:
- **`k8s/namespace.yaml`**: Namespace definition for cluster isolation.
- **`k8s/configmap.yaml` & `secrets.yaml`**: Cluster environment and credential bindings.
- **`k8s/api/`**: Horizontally autoscaled Go API deployment with readiness/liveness probes.
- **`k8s/syncd/` & `worker/`**: Ingestion worker pods with rate limit backoff.
- **`k8s/ingress.yaml`**: Ingress rules with TLS termination.

Deploy via `kubectl`:
```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/
```

---

## 🧪 Testing & Quality Assurance

```bash
# Verify Go syntax & run integration tests
cd go
go test -v ./...

# Verify Electron syntax
cd electron
npm run check

# Verify React/TypeScript web bundle
cd frontend
npm run build
```

---

## 📄 License

Distributed under the [MIT License](LICENSE).
