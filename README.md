# SkillIssues

<div align="center">

![SkillIssues Banner](frontend/public/logo.svg)

### *You Have the Skills. We Have the Issues.*

[![License: MIT](https://img.shields.io/badge/License-MIT-purple.svg)](LICENSE)
[![Python](https://img.shields.io/badge/Python-3.12-blue.svg?logo=python&logoColor=white)](https://python.org)
[![FastAPI](https://img.shields.io/badge/FastAPI-0.110+-009688.svg?logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg?logo=react&logoColor=black)](https://react.dev)
[![Electron](https://img.shields.io/badge/Electron-30-47848F.svg?logo=electron&logoColor=white)](https://www.electronjs.org)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED.svg?logo=docker&logoColor=white)](https://docker.com)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791.svg?logo=postgresql&logoColor=white)](https://postgresql.org)

**SkillIssues** is an open-source developer-growth platform designed to bridge the gap between learning syntax and mastering real-world software engineering. By matching a developer's experience level, interests, and preferred languages against a curated catalog of open-source GitHub issues, SkillIssues guides developers through real contributions, safe local workspace setup, pull request submissions, and server-verified level progression.

[Architecture Guide](ARCHITECTURE.md) • [Contributing Guide](CONTRIBUTING.md) • [Deployment Guide](DEPLOYMENT.md)

</div>

---

## 🌟 Why SkillIssues?

- **Real Codebases, Real Impact**: Stop grinding artificial toy problems. Work on authentic open-source issues across top repositories like FastAPI, HTTPX, Vite, React, Flask, Next.js, and Kubernetes.
- **Smart Matching Engine**: Deterministic recommendation scoring based on your demonstrated skills, target difficulty (`BEGINNER`, `INTERMEDIATE`, `ADVANCED`), and preferred tech stack.
- **Desktop Workflow Companion**: Safe local repository management in Electron—clone into sandboxed folders, create standardized feature branches, open in VS Code with 1 click, and push upstream.
- **Server-Side PR Verification**: Automated verification of GitHub PR merge states with gamified XP rewards, level progression, and unlockable achievement badges.
- **Zero-Friction GitHub Integration**: Connect directly via a standard GitHub Personal Access Token (PAT) with `repo` scope or through OAuth.

---

## 🔄 The Contribution Loop

```
  ┌────────────────────────┐
  │   Developer Profile    │ (Skills, Languages, Target Level)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │  Personalized Match    │ (Multi-factor scoring algorithm)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │ Workspace Auto-Setup   │ (Cloned to ~/Documents/SkillIssues)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │ Solve in VS Code / IDE │ (Write code, fix bug, run tests)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │ Push & Open Pull Req   │ (Standard branch: skillissues/issue-<id>)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │ Server-Side PR Verify  │ (Verify merge state, author attribution)
  └───────────┬────────────┘
              │
              ▼
  ┌────────────────────────┐
  │ Level Up & Earn Badges │ (XP awarded, harder tiers unlocked)
  └────────────────────────┘
```

---

## 🛠️ System Architecture

SkillIssues uses a clean microservices architecture orchestrated via Docker Compose:

| Service | Directory | Tech Stack | Port / Network | Description |
| :--- | :--- | :--- | :--- | :--- |
| **`frontend`** | `frontend/` | React 18, Vite, TypeScript | `:5173` | Modern web dashboard for issue discovery, saved challenges, and profile stats. |
| **`electron`** | `electron/` | Electron, Node.js, HTML/CSS | Native App | Secure desktop companion with sandboxed Git engine and VS Code launcher. |
| **`api`** | `services/api/` | Python 3.12, FastAPI, Uvicorn | `:8000` | Central control plane for auth, issue search, profiles, and verification. |
| **`recommendation`**| `services/recommendation/` | Python 3.12, FastAPI | Internal | Deterministic scoring engine matching developers with optimal issues. |
| **`contribution`** | `services/contribution/` | Python 3.12, FastAPI | Internal | Validates PR status, repository ownership, and author attribution on GitHub. |
| **`ingestion`** | `services/ingestion/` | Python 3.12, GraphQL, ijson | Internal | Ingests, sanitizes, and categorizes real-time issues from GitHub. |
| **`worker`** | `services/worker/` | Python 3.12, HTTPX | Background | Priority sync worker periodically refreshing catalog open issues. |
| **`postgres`** | `database/` | PostgreSQL 16 Alpine | `:5432` | Relational store for users, profiles, repositories, issues, and contributions. |

> 📖 For a deep architectural dive, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 📁 Repository Structure

```
SkillIssues/
├── .github/                 # GitHub Actions CI/CD workflows
├── database/                # PostgreSQL schema and seed migrations
│   ├── schema.sql           # Database tables, constraints, and indexes
│   └── seed/seed.sql        # Initial challenge catalog and sample data
├── electron/                # Native desktop companion application
│   ├── main.js              # Electron main process (secure Git engine, IPC handlers)
│   ├── preload.js           # Isolated IPC bridge (contextBridge)
│   ├── renderer.html        # Desktop interface markup
│   ├── renderer.js          # Desktop UI logic, state management, and Git controls
│   └── renderer.css         # Soothing Himalayas / lavender light styling
├── frontend/                # Web dashboard application
│   ├── src/
│   │   ├── main.tsx         # React app root (catalog discovery, profile, bookmarks)
│   │   └── styles.css       # Web design system styles
│   └── vite.config.ts       # Vite build configuration
├── services/                # Backend microservices
│   ├── api/                 # FastAPI API Gateway & Control Plane
│   ├── contribution/        # PR verification and author matching service
│   ├── ingestion/           # GitHub GraphQL issue ingestion & qualification pipeline
│   ├── recommendation/      # Personalized issue recommendation engine
│   └── worker/              # Background priority sync worker daemon
├── ARCHITECTURE.md          # Comprehensive technical design document
├── CONTRIBUTING.md          # Open-source contribution guidelines & code standards
├── DEPLOYMENT.md            # Production deployment guide (Docker, Nginx, Cloud)
├── compose.yaml             # Local development Docker Compose stack
├── docker-compose.prod.yml  # Production Docker Compose stack with Nginx proxy
└── LICENSE                  # MIT License
```

---

## ⚡ Quickstart Guide

### Prerequisites
- [Docker Desktop](https://www.docker.com/) (with Docker Compose enabled)
- [Node.js](https://nodejs.org/) v20+ and npm (for Electron desktop client)
- A GitHub account and [Personal Access Token (PAT)](https://github.com/settings/tokens) with `repo` scope

### 1. Clone the Repository
```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
```

### 2. Configure Environment
Copy the example environment file:
```bash
cp .env.example .env
```
*(Optional: Add your `GITHUB_TOKEN` to `.env` to enable live GitHub GraphQL issue sync.)*

### 3. Start the Backend Control Plane & Web App
```bash
docker compose up -d --build
```

- **Web Dashboard**: [http://localhost:5173](http://localhost:5173)
- **API Documentation (Swagger)**: [http://localhost:8000/docs](http://localhost:8000/docs)
- **Database**: `localhost:5432` (`skillissues` / `skillissues`)

### 4. Launch the Electron Desktop Client
In a separate terminal window:
```bash
cd electron
npm install
npm start
```

### 5. Connect Your GitHub Account
1. Open the Web Dashboard or Desktop Client.
2. Click **Connect GitHub** in the top navigation bar.
3. Paste your GitHub Personal Access Token (`ghp_...` or `github_pat_...`) with `repo` scope.
4. Browse personalized challenges, fork repositories, start local workspaces, and submit solutions!

---

## ⚙️ Environment Variables Reference

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `DATABASE_URL` | `postgresql://skillissues:skillissues@postgres:5432/skillissues` | PostgreSQL connection string. |
| `GITHUB_TOKEN` | *(Optional)* | GitHub PAT token for GraphQL catalog ingestion and PR verification. |
| `GITHUB_WEBHOOK_SECRET` | *(Optional)* | Secret for validating incoming GitHub webhook payloads. |
| `DEMO_MODE` | `true` (dev) / `false` (prod) | Allows demo authentication without external OAuth / PAT keys. |
| `RECOMMENDATION_URL` | `http://recommendation:8000` | Internal URL for the recommendation microservice. |
| `CONTRIBUTION_URL` | `http://contribution:8000` | Internal URL for the contribution verification service. |
| `ADMIN_SYNC_TOKEN` | `dev-sync-token` | Secret header token required for admin catalog sync requests. |
| `SYNC_INTERVAL_SECONDS` | `900` | Background issue refresh interval for the worker daemon. |
| `SKILLISSUES_API_URL` | `http://localhost:8000` | Target API gateway URL used by the Electron desktop client. |

---

## 📖 Key API Endpoints

- `GET /health` — Service health probe.
- `POST /auth/github/token` — Authenticate via GitHub PAT.
- `GET /me` — Fetch authenticated user profile, level, XP, and badges.
- `PUT /profile` — Update developer bio, target level, interests, and preferred languages.
- `GET /issues` — Search and filter open-source issues (`?search=<q>&language=<lang>&difficulty=<diff>`).
- `GET /recommendations` — Fetch personalized issue recommendations scored by profile fit.
- `GET /saved` / `POST /saved` / `DELETE /saved/{id}` — Manage saved challenge bookmarks.
- `GET /working` — List currently active local workspace contributions.
- `POST /contributions` — Track new contribution (fork & clone initialization).
- `POST /contributions/{id}/submit` — Submit branch and record GitHub PR.
- `POST /contributions/{id}/verify` — Server-side verification of PR merge status & XP allocation.

---

## 🧪 Testing & Code Quality

```bash
# Node.js syntax verification for Electron
node -c electron/main.js && node -c electron/preload.js && node -c electron/renderer.js

# Python syntax compilation across all services
python -m py_compile services/api/app/main.py services/recommendation/app.py services/contribution/app.py services/worker/worker.py services/ingestion/app.py

# Format JS/TS/CSS/HTML code with Prettier
npx prettier --write "electron/**/*.{js,css,html}" "frontend/src/**/*.{ts,tsx,css}"
```

---

## 🤝 Contributing

We welcome contributions from developers of all skill levels! Whether you want to add new issue classification heuristics, expand the recommendation engine, improve UI accessibility, or refine documentation:

1. Read our [Contributing Guide](CONTRIBUTING.md).
2. Review the [Architecture Guide](ARCHITECTURE.md).
3. Pick an open issue or open a pull request.

---

## 📄 License

Distributed under the [MIT License](LICENSE).
