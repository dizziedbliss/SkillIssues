# Contributing to SkillIssues

Thank you for your interest in contributing to **SkillIssues**! We welcome contributions from developers of all skill levels—whether you are fixing a bug, adding new features, improving documentation, or creating new challenge catalog connectors.

---

## 🛠️ Getting Started

### 1. Prerequisites
Before contributing, make sure you have the following installed locally:
- **Git**
- **Docker Desktop** (with Docker Compose)
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

- `electron/` - Electron desktop app source code (`main.js`, `preload.js`, `renderer.js`, `renderer.html`, `renderer.css`).
- `frontend/` - React + TypeScript + Vite web dashboard (`src/main.tsx`, `src/styles.css`).
- `services/api/` - FastAPI control plane service (`app/main.py`).
- `services/ingestion/` - Data ingestion & qualification pipeline (`app.py`).
- `services/recommendation/` - Issue recommendation calculation service (`app.py`).
- `services/contribution/` - Contribution state & PR validator (`app.py`).
- `services/worker/` - Priority refresh worker service (`worker.py`).
- `database/` - PostgreSQL schema migrations (`schema.sql`) and seed data (`seed/seed.sql`).

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

# Format JS/TS/CSS/HTML code
npx prettier --write "electron/**/*.{js,css,html}" "frontend/src/**/*.{ts,tsx,css}"
```

---

## 🌿 Branching & Pull Requests

### Branch Naming Conventions
Use clear, descriptive branch names prefixed by feature or issue number:
- `feature/issue-recommendation-scoring`
- `fix/git-runner-progress`
- `docs/contributing-guide`
- `skillissues/issue-#<number>`

### Submitting a Pull Request
1. Commit your changes with clear, descriptive commit messages:
   ```bash
   git commit -m "feat(electron): add runner progress indicator for git operations"
   ```
2. Push your feature branch to your GitHub fork:
   ```bash
   git push origin feature/your-feature-name
   ```
3. Open a Pull Request on the main `SkillIssues` repository.
4. Ensure all automated syntax checks pass.

---

## 📋 Code Guidelines

- **Preserve API Contracts**: Ensure all method signatures and JSON endpoint schemas remain backwards-compatible.
- **Error Handling**: Provide clear error messages in response payloads and avoid swallowing runtime exceptions silently.
- **Clean UI Feedback**: Keep button press animations, hover states, and toast notifications consistent with the AWS Console dark design system (`#0f141c`, `#161e2e`, `#ec7211`, `#539fe5`).

---

## 📄 License

By contributing to SkillIssues, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).
