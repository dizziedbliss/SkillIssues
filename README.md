# SkillIssues

> **You have the skills. We have the issues.**

SkillIssues curates real open-source GitHub issues and helps you sort through the chaos.

[Architecture](ARCHITECTURE.md) · [Deployment](DEPLOYMENT.md) · [Contributing](CONTRIBUTING.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

---

## What is this?

GitHub has more issues than anyone has time to read. SkillIssues pulls that work into one place and helps you find the issues that fit what you already know — or what you are curious enough to wrestle with next.

It is not another tutorial platform. It is not trying to teach you software development from scratch. It is a curated issue browser with useful sorting, matching, workspace setup, and contribution tracking.

In other words:

```text
GitHub's issue chaos
        ↓
curated, categorized challenges
        ↓
filters that actually help
        ↓
a real repository and a real pull request
```

---

## Why SkillIssues?

Because “good first issue” is helpful, but it is not exactly a personality test.

SkillIssues looks at open-source issues and organizes them by things like:
- language and framework;
- difficulty and expected effort;
- repository and issue metadata;
- required skills and contributor preferences;
- issue category, author, and organization.

Then it gives you somewhere less overwhelming to start looking.

---

## What it can do

- **Curate issues** from real open-source repositories.
- **Sort and filter** by skills, languages, frameworks, difficulty, and categories.
- **Recommend challenges** based on your developer profile.
- **Set up workspaces** through the Electron desktop app: fork, clone, configure remotes, create a branch, and open VS Code in one click.
- **Track contributions** from workspace setup to pull request.
- **Verify pull requests** through GitHub before awarding progression.
- **Add a little game layer** with XP, levels, and badges — because spreadsheets are not emotionally rewarding.

---

## How it works

1. `syncd` collects and qualifies GitHub issues.
2. SkillIssues categorizes and scores them.
3. You search, filter, or browse the catalog.
4. You pick an issue that looks interesting — or at least survivable.
5. The desktop app can prepare a local workspace for you.
6. You work in the actual repository and open a pull request.
7. GitHub verifies the contribution and SkillIssues updates your progress.

### XP currently looks like this

| Difficulty | XP |
| :--- | :--- |
| `BEGINNER` | 75 |
| `INTERMEDIATE` | 140 |
| `ADVANCED` | 250 |

No XP for clicking a shiny button. No XP for saying “I will definitely start this tomorrow.” Verified work only.

---

## Architecture

```text
  React + Vite web app           Electron desktop app
         \                                /
          \        HTTP / REST           /
           v                            v
              Go API Gateway :8000
                 |            |
                 v            v
            PostgreSQL      Redis
                 ^            ^
                 |            |
              syncd :8002 + worker
                       ^
                       |
               GitHub API / ETL
```

The main pieces are:

```text
SkillIssues/
├── backend/          # Go API, sync coordinator, worker, and ETL
├── web/              # React + Vite + TypeScript app
├── desktop/          # Electron workspace companion
├── database/         # PostgreSQL schema and migrations
├── k8s/              # Kubernetes manifests
├── compose.yaml      # Local development stack
├── ARCHITECTURE.md   # Architectural deep-dive
├── DEPLOYMENT.md     # Production deployment instructions
├── CONTRIBUTING.md   # Dev setup & pull request guide
└── CODE_OF_CONDUCT.md # Community ground rules
```

For the full technical tour, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## Getting started

### Prerequisites

- [Git](https://git-scm.com/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) 24+ with Compose
- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 20+ and npm
- A GitHub account
- A GitHub token with `read:user` and `repo` scopes for authenticated local flows

### Start the stack

```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
# Add the GitHub credentials you need to .env
docker compose up -d --build
docker compose ps
```

Open:
- **Web app**: [http://localhost:5173](http://localhost:5173)
- **API**: [http://localhost:8000](http://localhost:8000)
- **Metrics**: [http://localhost:8000/metrics](http://localhost:8000/metrics)
- **Sync coordinator**: [http://localhost:8002](http://localhost:8002)

See live service logs with:
```bash
docker compose logs -f go-api syncd worker
```

### Start the desktop app

```bash
cd desktop
npm install
npm start
```

The desktop client expects the API at `http://localhost:8000`.

### Useful checks

```bash
cd backend && go test -v ./...
cd backend && go fmt ./... && go vet ./...
cd web && npm run build
cd desktop && npm run check
```

For native development, environment details, and production deployment, see [CONTRIBUTING.md](CONTRIBUTING.md) and [DEPLOYMENT.md](DEPLOYMENT.md).

---

## Contributing

Found a bug? Have a better way to sort the issue pile? Want to make the desktop flow less cursed?

Please read [CONTRIBUTING.md](CONTRIBUTING.md), then open an issue or send a pull request. Documentation fixes, UI polish, tests, accessibility improvements, and “this error message made me lose five minutes of my life” fixes are all welcome.

New to the repository is fine. Nobody loads a large codebase into their brain on the first clone.

Please also read our [Code of Conduct](CODE_OF_CONDUCT.md). Be curious, be kind, and keep the issue tracker useful for the next person.

---

## Project status

SkillIssues is actively evolving. Some parts are production-minded; some parts are still wearing a tiny TODO sticker. APIs and UI details may change while the project settles.

If something breaks, [open an issue](https://github.com/dizziedbliss/SkillIssues/issues) with:
- what you expected;
- what happened;
- steps to reproduce;
- logs or screenshots with secrets removed;
- your OS and relevant tool versions.

---

## License

SkillIssues is released under the [MIT License](LICENSE).

---

⭐ Star it. Try it. Sort the chaos. Send a pull request.
