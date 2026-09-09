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
- `services/ingestion`: streams `repo/repo_metadata.json`, cleans and qualifies repositories, and upserts a bounded catalog into PostgreSQL. It does not load the multi-gigabyte source into memory or require Kaggle.
- `services/recommendation`: service boundary reserved for extracting recommendation computation as traffic grows.
- `services/contribution`: service boundary for PR workflows.
- `services/worker`: simple priority-refresh heartbeat, intentionally no queue or cache infrastructure.
- `database`: PostgreSQL schema and demo seed.

The API recommendation score is explainable and combines skill overlap, interest overlap, language preference, difficulty fit, and repository quality. GitHub sync and rate-limit-aware ingestion are deliberately kept as the next increment rather than blocking the vertical slice.

## Local repository import

The repository metadata export is mounted read-only into the ingestion container. Qualification requires a non-archived repository, `stars > 50`, `forkingAllowed == true`, and a non-empty license. Import size is bounded by `MAX_REPOSITORIES` (default `1000`) so the demo remains predictable.

```bash
docker compose run --rm -e RUN_ONCE=true -e MAX_REPOSITORIES=100 ingestion
```

The source path and threshold are configurable with `REPOSITORY_METADATA_PATH` and `MIN_STARS`. The large metadata export is intentionally ignored by Git and must exist locally when running the importer.

## Core API

`GET /health`, `GET /me`, `PUT /profile`, `GET /issues`, `GET /issues/{id}`, `GET /recommendations`, `POST /contributions`, `PUT /contributions/{id}`, and `POST /contributions/{id}/verify`.
