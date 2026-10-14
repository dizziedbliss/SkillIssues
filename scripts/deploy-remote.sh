#!/usr/bin/env bash
# Runs ON THE VPS, not in CI. deploy.yml scp's this file (plus
# compose.prod.yaml and Caddyfile) to $DEPLOY_PATH and then ssh's in to
# execute it there — doing the pull/up/health-wait as a single local script
# avoids the quoting nightmare of embedding this logic in an SSH heredoc
# from inside a GitHub Actions expression.
#
# Usage: deploy-remote.sh <image-tag>
#   image-tag: a sha-<short> tag (never `latest` — see compose.prod.yaml's
#   own comment on why) published by the build-push job, or an older one
#   supplied via workflow_dispatch for a rollback.
set -euo pipefail

if [ $# -ne 1 ] || [ -z "$1" ]; then
	echo "usage: $0 <image-tag>" >&2
	exit 1
fi

cd "$(dirname "$0")"

export IMAGE_TAG="$1"
echo "Deploying image tag: ${IMAGE_TAG}"

if [ ! -f .env ]; then
	echo ".env not found in $(pwd) — see DEPLOYMENT.md for first-time host setup" >&2
	exit 1
fi

# Export host env so fallback interpolation can use existing required secrets.
set -a
# shellcheck disable=SC1091
source .env
set +a

# Backward-compatible fallbacks for hosts whose .env predates monitoring vars.
# Keep them in-process only; don't rewrite the host .env.
if [ -z "${METRICS_DB_PASSWORD:-}" ] && [ -n "${POSTGRES_PASSWORD:-}" ]; then
	export METRICS_DB_PASSWORD="$POSTGRES_PASSWORD"
fi
if [ -z "${GRAFANA_ADMIN_PASSWORD:-}" ] && [ -n "${ADMIN_SYNC_TOKEN:-}" ]; then
	export GRAFANA_ADMIN_PASSWORD="$ADMIN_SYNC_TOKEN"
fi

COMPOSE=(docker compose -f compose.prod.yaml --env-file .env)

"${COMPOSE[@]}" pull
"${COMPOSE[@]}" up -d --remove-orphans

# Wait for go-api to report healthy before declaring success — a container
# that's merely "Up" may still be failing its startup DB/Redis connection.
healthy=""
for _ in $(seq 1 30); do
	cid="$("${COMPOSE[@]}" ps -q go-api)"
	if [ -n "$cid" ]; then
		status="$(docker inspect --format='{{.State.Health.Status}}' "$cid" 2>/dev/null || echo starting)"
		if [ "$status" = "healthy" ]; then
			healthy=1
			break
		fi
	fi
	sleep 2
done

if [ -z "$healthy" ]; then
	echo "go-api never became healthy — deploy failed, not rolling back automatically" >&2
	echo "--- go-api logs (last 200 lines) ---" >&2
	"${COMPOSE[@]}" logs go-api --tail=200 >&2 || true
	exit 1
fi

echo "go-api is healthy."
docker image prune -f
echo "Deploy of ${IMAGE_TAG} complete."
