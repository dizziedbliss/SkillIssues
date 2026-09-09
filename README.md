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
- `services/ingestion`: reads `repo/repo_metadata.json` in bounded batches, uses pandas for vectorized qualification, and upserts a bounded catalog into PostgreSQL. It does not materialize the multi-gigabyte source in memory.
- `services/recommendation`: service boundary reserved for extracting recommendation computation as traffic grows.
- `services/contribution`: validates PR repository ownership and GitHub author attribution for contribution workflows.
- `services/worker`: scheduled priority-refresh caller; it triggers ingestion without adding a queue or cache infrastructure.
- `database`: PostgreSQL schema and demo seed.

The recommendation service scores skill overlap, interest overlap, language preference, difficulty fit, and repository quality. The API forwards scoring to that service with a local fallback if the service is unavailable.

## Local repository import

The repository metadata export is mounted read-only into the ingestion container. Qualification requires a non-archived repository, `stars > 50`, `forkingAllowed == true`, and a non-empty license. Import size is bounded by `MAX_REPOSITORIES` (default `1000`) so the demo remains predictable.

```bash
docker compose run --rm -e RUN_ONCE=true -e MAX_REPOSITORIES=100 ingestion
```

For a clean test catalog of exactly 100 sampled repositories, reset the local demo catalog explicitly:

```bash
docker compose run --rm -e RUN_ONCE=true -e RESET_CATALOG=true -e MAX_REPOSITORIES=100 -e SAMPLE_SEED=skillissues-demo ingestion
```

Do not use `RESET_CATALOG=true` in a production refresh job; it removes repository, issue, saved-issue, and contribution rows before importing.

The source path and threshold are configurable with `REPOSITORY_METADATA_PATH` and `MIN_STARS`. The large metadata export is intentionally ignored by Git and must exist locally when running the importer. The ingestion image installs pandas and ijson; pandas filters each 25,000-record batch and the seeded reservoir sampler keeps only the configured sample size.

Download the requested Kaggle archive with:

```bash
curl -L -o ~/Downloads/github-repository-metadata-with-5-stars.zip https://www.kaggle.com/api/v1/datasets/download/pelmers/github-repository-metadata-with-5-stars
```

The archive contains `repo_metadata.json` and is several gigabytes after extraction. Extract it to the ignored `repo/` directory before running the importer:

```bash
mkdir -p repo
unzip -o ~/Downloads/github-repository-metadata-with-5-stars.zip repo_metadata.json -d repo
docker compose run --rm -e RUN_ONCE=true -e MAX_REPOSITORIES=100 ingestion
```

## Core API

`GET /health`, `GET /auth/github`, `POST /auth/demo`, `GET /me`, `PUT /profile`, `GET /issues`, `GET /issues/{id}`, `GET /recommendations`, `GET /saved`, `POST /saved`, `DELETE /saved/{id}`, `GET /working`, `GET /progress`, `POST /contributions`, `PUT /contributions/{id}`, and `POST /contributions/{id}/verify`.

## Current runbook

```bash
docker compose up --build
docker compose ps
docker compose logs -f api ingestion worker
docker compose down
docker compose down -v
```

Open http://localhost:5173 for the dashboard and http://localhost:8000/docs for the API. With `GITHUB_TOKEN` configured in a local `.env`, run a bounded real issue sync:

```bash
docker compose run --rm -e RUN_ONCE=true -e SYNC_ISSUES=true -e MAX_REPOSITORIES=10 ingestion
```

The worker periodically calls ingestion. A `409` from a manual sync request means another sync is already running and is intentional overlap protection.

## Electron desktop client

Start Docker first so the API is available, then launch the desktop client separately:

```bash
cd electron
npm install
npm start
```

The desktop workflow is functional and independent from the dashboard: select a recommended issue, choose an existing local destination, clone the public repository, and inspect local Git status. The Electron renderer has no access to API secrets.

## What is complete

The project now has the Compose monorepo, PostgreSQL schema, local streaming repository catalog import, GitHub GraphQL issue sync with pagination/incremental timestamps/rate-limit state, heuristic issue analysis, service-backed recommendations, session-backed demo/OAuth boundaries, signed webhook intake, profile/progress tracking, contribution state, server-side PR verification, React dashboard, and a working Electron challenge, branch, push, and command-log workflow.

## Remaining roadmap

GitHub App installation authentication, encrypted OAuth token storage, richer issue skill extraction, persistent job history, and deployment automation remain. The MVP deliberately keeps these focused rather than adding Kafka, Redis, Kubernetes, or automatic code execution.

## Debugging runbook

Check the service graph first:

```bash
docker compose ps
docker compose logs --tail=100 api frontend ingestion worker
curl http://localhost:8000/health
curl http://localhost:8000/health/detail
```

For demo authentication, click `Use demo profile` in the web client. The API creates the demo user and sets an HttpOnly `skillissues_session` cookie. Browser requests must include credentials; do not copy session cookies or GitHub tokens into frontend source.

For GitHub OAuth, configure `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `GITHUB_REDIRECT_URI` in `.env`. Register the exact callback URL with GitHub. The login flow uses a short-lived state cookie and PKCE, then redirects back to `FRONTEND_URL`; GitHub access tokens are never returned to the browser.

For local GitHub App testing, the GitHub App settings must contain this exact **Callback URL**:

```text
http://localhost:8000/auth/github/callback
```

Set the local environment to the same value:

```env
GITHUB_REDIRECT_URI=http://localhost:8000/auth/github/callback
```

The callback URL is not the frontend URL. `http://localhost:5173` belongs in `FRONTEND_URL`; GitHub must redirect to the API callback on port `8000`. After changing `.env`, restart the API with `docker compose up -d --build api frontend`. For a deployed environment, register the HTTPS API callback URL instead, for example `https://api.example.com/auth/github/callback`, and set `GITHUB_REDIRECT_URI` to that exact HTTPS URL. GitHub compares this value exactly, including scheme, host, port, path, and trailing slash.

For contribution work in Electron, connect GitHub with the `public_repo` permission before solving an issue. Electron then creates or reuses a fork, clones the fork into `Documents/SkillIssues`, pushes the branch with an ephemeral authorization header, and creates the pull request through the API. A contribution only changes progress after GitHub reports that the PR is merged. Demo mode can browse and save issues, but cannot fork, push, or create pull requests.

The Electron client never opens a folder picker. It clones into `Documents/SkillIssues/<repository>` after `Start challenge`; use `Open workspace` after cloning. If clone or Git operations fail, inspect the command log at `<workspace>/.skillissues/commands.log` and confirm that Git is installed.

If Docker commands fail with a daemon connection error, start Docker Desktop and rerun `docker compose up --build`. The frontend can still be checked independently with `npm --prefix frontend run typecheck` and `npm --prefix frontend run build`.
