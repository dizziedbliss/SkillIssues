# SkillIssues

SkillIssues is a focused developer-growth MVP: it matches a developer profile to appropriately challenging open-source issues, starts a contribution workspace, and verifies merged pull requests against GitHub before updating progress.

## Run the demo

Prerequisites: Docker Desktop with Compose.

```bash
docker compose up --build
```

Open http://localhost:5173 for the dashboard and http://localhost:8000/docs for the API. The first API startup creates the PostgreSQL schema and loads representative seed data, so the demo works without a GitHub token.

Stop the stack with `docker compose down`. Add `GITHUB_TOKEN` to a local `.env` to enable real pull request verification; the endpoint never treats a client claim as proof of merge.

## Architecture

- `frontend`: React + TypeScript + Vite dashboard.
- `services/api`: FastAPI control plane, profile, issue, recommendation, contribution, and GitHub verification endpoints.
- `services/ingestion`: ingestion boundary for future Kaggle/GitHub synchronization; seed SQL is deterministic and local for the demo.
- `services/recommendation`: service boundary reserved for extracting recommendation computation as traffic grows.
- `services/contribution`: service boundary for PR workflows.
- `services/worker`: simple priority-refresh heartbeat, intentionally no queue or cache infrastructure.
- `database`: PostgreSQL schema and demo seed.

The API recommendation score is explainable and combines skill overlap, interest overlap, language preference, difficulty fit, and repository quality. GitHub sync and rate-limit-aware ingestion are deliberately kept as the next increment rather than blocking the vertical slice.

## Core API

`GET /health`, `GET /me`, `PUT /profile`, `GET /issues`, `GET /issues/{id}`, `GET /recommendations`, `POST /contributions`, `PUT /contributions/{id}`, and `POST /contributions/{id}/verify`.
