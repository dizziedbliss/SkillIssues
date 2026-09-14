# Contributing to SkillIssues

Thank you for your interest in contributing to **SkillIssues**! We welcome contributions from developers of all skill levels—whether you are fixing a bug, adding new features, improving documentation, or creating new challenge catalog connectors.

---

## 🛠️ Getting Started

### 1. Prerequisites
Before contributing, ensure you have the following installed locally:
- **Git**
- **Docker Desktop** (with Docker Compose enabled)
- **Node.js** (v20+) & **npm**
- **Python 3.12+**

### 2. Fork & Clone
1. Fork the repository on GitHub: `https://github.com/dizziedbliss/SkillIssues`
2. Clone your fork locally:
   ```bash
   git clone https://github.com/<your-username>/SkillIssues.git
   cd SkillIssues
   ```

---

## 🏗️ Repository Layout

- `electron/` — Electron desktop app source code (`main.js`, `preload.js`, `renderer.js`, `renderer.html`, `renderer.css`).
- `frontend/` — React + TypeScript + Vite web dashboard (`src/main.tsx`, `src/styles.css`).
- `services/api/` — FastAPI control plane service (`app/main.py`).
- `services/ingestion/` — Data ingestion & qualification pipeline (`app.py`).
- `services/recommendation/` — Issue recommendation calculation service (`app.py`).
- `services/contribution/` — Contribution state & PR validator (`app.py`).
- `services/worker/` — Priority refresh worker service (`worker.py`).
- `database/` — PostgreSQL schema migrations (`schema.sql`) and seed data (`seed/seed.sql`).

For a detailed system design and microservice breakdown, refer to [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 💻 Local Development Workflow

### 1. Start Microservices
Start the core backend stack with Docker Compose:
```bash
docker compose up -d --build
```

Verify service health at:
- Web App: `http://localhost:5173`
- API Health: `http://localhost:8000/health`
- API Docs: `http://localhost:8000/docs`

### 2. Launch the Desktop Client
In a separate terminal window:
```bash
cd electron
npm install
npm start
```

### 3. Syntax Verification & Formatting
Before committing any changes, run syntax checks and Prettier formatting:
```bash
# Node syntax check for Electron scripts
node -c electron/main.js && node -c electron/preload.js && node -c electron/renderer.js

# Python syntax compilation
python -m py_compile services/api/app/main.py services/recommendation/app.py services/contribution/app.py services/worker/worker.py services/ingestion/app.py

# Format JS/TS/CSS/HTML code
npx prettier --write "electron/**/*.{js,css,html}" "frontend/src/**/*.{ts,tsx,css}"
```

---

## 🌿 Branching & Pull Requests

### Branch Naming Conventions
Use clear, descriptive branch names prefixed by feature, fix, or issue number:
- `feature/issue-recommendation-scoring`
- `fix/git-runner-progress`
- `docs/architecture-guide`
- `skillissues/issue-<number>` *(Note: use hyphens instead of `#` to ensure git CLI compatibility)*

### Submitting a Pull Request
1. Commit your changes with clear, descriptive commit messages following Conventional Commits:
   ```bash
   git commit -m "feat(electron): add runner progress indicator for git operations"
   ```
2. Push your feature branch to your GitHub fork:
   ```bash
   git push origin feature/your-feature-name
   ```
3. Open a Pull Request against the main branch on `SkillIssues`.
4. Ensure all automated syntax checks pass.

---

## 📋 Code & Design Guidelines

- **Preserve API Contracts**: Ensure all method signatures and JSON endpoint schemas remain backwards-compatible.
- **Error Handling**: Provide clear, user-friendly error messages in response payloads and avoid swallowing runtime exceptions silently.
- **Design System Consistency**:
  - Web and Desktop apps follow the soothing **Himalayas light theme** with clean whitish-lavender accents (`#f8f7fc`, `#7c5ce5`, `#6344d4`).
  - Keep feedback clear and non-intrusive.
  - Maintain spacious, readable typography (Noto Sans wordmarks, clean sans-serif text) with comfortable card padding and soft ambient shadows.
- **Security Boundaries**:
  - Never expose backend secrets or raw GitHub tokens to the Electron renderer window.
  - Confine all local Git operations to the designated `Documents/SkillIssues` workspace directory.
  - Validate branch names and paths strictly to avoid command injection.
