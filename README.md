# SkillIssues

> **You Have the Skills. We Have the Issues.**

SkillIssues is a developer-growth platform designed to bridge the gap between learning syntax and mastering real-world software engineering. By matching a developer's experience level, interests, and preferred programming languages against a curated catalog of open-source GitHub issues, SkillIssues guides developers through real contributions, workspace cloning, pull request submission, and server-verified level progression.

---

## 🚀 Key Features

- **Personalized Challenge Matching**: Issues classified by difficulty (`BEGINNER`, `INTERMEDIATE`, `ADVANCED`), language, required skills, and interest tags.
- **Authentic Open-Source Catalog**: Pre-populated catalog of over **1,600+ real GitHub issues** across top repositories (`FastAPI`, `HTTPX`, `Vite`, `React`, `Flask`, `Next.js`, `Go`, `Kubernetes`, etc.) complete with authentic issue author handles.
- **Electron Desktop Client**:
  - Top runner progress bar for background Git operations (forking, cloning, committing, pushing).
  - 1-click **VS Code** workspace opening and local folder navigation.
  - Workspace drawer with branch creation (`skillissues/issue-#<number>`), commit & push dialog, and live Git status.
  - PAT-based direct login without complex OAuth setups.
- **Interactive Web Dashboard (React + Vite)**:
  - Clean markdown description rendering with syntax highlighting and GitHub Flavored Markdown support.
  - Filter by preferences, search query override, and saved challenges tab.
  - Profile progression sidebar with Level status, XP bar, completed count, and achievement badges.
- **Server-Side GitHub PR Verification**: Real-time verification of pull request merge status with level progression, XP awards, and unlockable badges (`First Blood 🩸`, `Bug Wrangler 🩹`, `Polyglot 🧪`, `World Traveller 🗺️`, `Touch Grass 🌱`).

---

## 🔄 The Core Feedback Loop

```
  ┌──────────────────────┐
  │  Developer Profile   │ (Skills, Languages, Target Level)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Personalized Match   │ (Scored by skill fit & difficulty)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Workspace Auto-Setup │ (Fork & Clone to local directory)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Solve Real Codebase  │ (Write solution in VS Code / IDE)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Commit & Submit PR   │ (Branch creation & GitHub Push)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Server Verification  │ (Verify merge & calculate XP / Badges)
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ Level Up & Progression│ (Harder challenges unlocked)
  └──────────────────────┘
```

---

## 🛠️ Architecture Overview

SkillIssues is built with a microservice control plane powered by Docker Compose:

| Service | Technology | Port / Role | Description |
| :--- | :--- | :--- | :--- |
| **`frontend`** | React 18, Vite, TypeScript | `:5173` | Web dashboard for issue discovery, saved challenges, and profile overview. |
| **`electron`** | Electron, Node.js, HTML/CSS/JS | Native App | Desktop client with integrated Git tools, VS Code launcher, and workspace manager. |
| **`api`** | Python 3.12, FastAPI, Uvicorn | `:8000` | Main control plane for auth, issue search, profile management, and GitHub verification. |
| **`ingestion`** | Python 3.12, Pandas, ijson | `:8000` (Internal) | Streamed catalog ingestion & qualification for GitHub repositories and open issues. |
| **`recommendation`** | Python 3.12, FastAPI | Internal | Service boundary for issue scoring based on skill overlap, language fit, and difficulty. |
| **`contribution`** | Python 3.12, FastAPI | Internal | Validates PR repository ownership and author attribution. |
| **`worker`** | Python 3.12 | Background | Priority refresh worker for background catalog sync without heavy queues. |
| **`postgres`** | PostgreSQL 16 Alpine | `:5432` | Relational storage for users, profiles, repositories, issues, saved challenges, and PR contributions. |

---

## ⚡ Quickstart Guide

### Prerequisites

- **Docker Desktop** (with Compose enabled)
- **Node.js** v20+ (for Electron desktop app)
- **GitHub Personal Access Token (PAT)** with `repo` scope (for forking & PR verification)

### 1. Run the Control Plane & Web App

Clone the repository and start the microservice stack:

```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
docker compose up -d --build
```

- Web Dashboard: [http://localhost:5173](http://localhost:5173)
- API OpenAPI Docs: [http://localhost:8000/docs](http://localhost:8000/docs)

### 2. Launch the Electron Desktop Client

In a separate terminal window:

```bash
cd electron
npm install
npm start
```

### 3. Connect Your GitHub Account

1. Open either the Electron client or Web Dashboard.
2. Click **Connect GitHub** in the top navigation bar.
3. Enter your GitHub Personal Access Token (`ghp_...` or `github_pat_...`) with `repo` scope.
4. You are ready to fork repos, track workspaces, and verify PR contributions!

---

## 📖 Key API Endpoints

- `GET /health` - Service health status check.
- `POST /auth/github/token` - Authenticate directly via GitHub PAT token.
- `GET /me` - Retrieve user profile, level, XP, level progression info, and achievement badges.
- `PUT /profile` - Update developer bio, target level, interests, and preferred languages.
- `GET /issues` - Search and list qualified open-source issues (`?search=<query>&limit=20`).
- `GET /issues/{id}` - Retrieve detailed issue payload.
- `GET /recommendations` - Get personalized issue recommendations scored by developer profile.
- `GET /saved` / `POST /saved` / `DELETE /saved/{id}` - Manage saved challenge bookmark list.
- `GET /working` - List active local repository contributions.
- `POST /contributions` - Track new contribution (fork & clone initialization).
- `POST /contributions/{id}/submit` - Create branch and submit PR to GitHub.
- `POST /contributions/{id}/verify` - Server-side verification of PR merge state + XP allocation.

---

## 🧪 Development & Testing

Run syntax checks and format code:

```bash
# Node.js syntax verification for Electron
node -c electron/main.js && node -c electron/preload.js && node -c electron/renderer.js

# Code formatting with Prettier
npx prettier --write "electron/**/*.{js,css,html}" "frontend/src/**/*.{ts,tsx,css}"

# Rebuild containers
docker compose up -d --build api frontend
```

---

## 📄 License

Distributed under the [MIT License](LICENSE).
