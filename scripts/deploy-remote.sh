#!/usr/bin/env bash
# Runs ON THE VPS, not in CI. deploy.yml scp's this file (plus
# compose.prod.yaml and Caddyfile) to $DEPLOY_PATH and then ssh's in to
# execute it there — doing the pull/up/health-wait as a single local script
# avoids the quoting nightmare of embedding this logic in an SSH heredoc
# from inside a GitHub Actions expression.
#
# Needs decrypt-env.sh beside it (deploy.yml copies both).
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

# The tag reaches a shell on the host and names an image to pull: accept
# nothing but a commit-derived tag.
if ! [[ "$1" =~ ^sha-[0-9a-f]{7,40}$ ]]; then
	echo "image tag must match sha-<hex>, got: $1" >&2
	exit 1
fi

cd "$(dirname "$0")"

export IMAGE_TAG="$1"
echo "Deploying image tag: ${IMAGE_TAG}"

# .env comes from secrets/prod.env.sops, which deploy.yml copied here still
# encrypted: CI never sees the plaintext. A file that is present but will not
# decrypt fails the deploy rather than falling back to a stale .env — that would
# quietly keep running on secrets you believed you had rotated. Only a host with
# no encrypted file at all (not yet migrated) keeps the .env already on it.
if [ -f secrets/prod.env.sops ]; then
	./decrypt-env.sh secrets/prod.env.sops .env
elif [ -f .env ]; then
	echo "WARNING: secrets/prod.env.sops not found; using the .env already on this host." >&2
	echo "         Move it into SOPS (DEPLOYMENT.md, 'Secrets') so CI-delivered secrets stay encrypted." >&2
else
	echo "neither secrets/prod.env.sops nor .env found in $(pwd) — see DEPLOYMENT.md for first-time host setup" >&2
	exit 1
fi

COMPOSE=(docker compose -f compose.prod.yaml --env-file .env)

# Fail before touching anything if .env lacks a required secret. Compose's
# own error names the key; never fall back to reusing another secret.
if ! "${COMPOSE[@]}" config --quiet; then
	echo "compose.prod.yaml does not resolve against .env — add the key named above." >&2
	echo "Nothing was changed; production is still on the previous release." >&2
	echo "See DEPLOYMENT.md, 'First-time host setup' step 2, for every required key." >&2
	exit 1
fi

# Only images that deploy.yml built and signed on main are allowed to run.
# A tag is mutable, so: pull the tag, take the digest it resolved to, verify the
# cosign signature on that digest, and run that digest (compose.prod.yaml's
# *_IMAGE overrides). Anyone who can push to GHCR can move a tag; they cannot
# produce a signature whose certificate names this workflow on main.
if ! command -v cosign >/dev/null 2>&1; then
	echo "cosign is not installed on this host — see DEPLOYMENT.md, 'First-time host setup'." >&2
	echo "Nothing was changed; production is still on the previous release." >&2
	exit 1
fi
IMAGE_PREFIX="ghcr.io/dizziedbliss"
SIGNER_IDENTITY='^https://github\.com/dizziedbliss/SkillIssues/\.github/workflows/deploy\.yml@refs/heads/main$'
SIGNER_ISSUER='https://token.actions.githubusercontent.com'
for name in api ingestd flowd migrate web; do
	repo="${IMAGE_PREFIX}/skillissues-${name}"
	docker pull --quiet "${repo}:${IMAGE_TAG}" >/dev/null
	digest_ref="$(docker inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "${repo}:${IMAGE_TAG}" | grep -F "${repo}@sha256:" | head -n 1)"
	if ! [[ "$digest_ref" =~ ^${repo}@sha256:[0-9a-f]{64}$ ]]; then
		echo "could not resolve a digest for ${repo}:${IMAGE_TAG}" >&2
		exit 1
	fi
	if ! cosign verify \
		--certificate-identity-regexp "$SIGNER_IDENTITY" \
		--certificate-oidc-issuer "$SIGNER_ISSUER" \
		"$digest_ref" >/dev/null; then
		echo "signature check FAILED for ${digest_ref} — refusing to deploy." >&2
		echo "Nothing was changed; production is still on the previous release." >&2
		exit 1
	fi
	echo "verified ${digest_ref}"
	upper="$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')"
	export "${upper}_IMAGE=${repo}:${IMAGE_TAG}@${digest_ref#*@}"
done

"${COMPOSE[@]}" pull
"${COMPOSE[@]}" up -d --remove-orphans

# `up` only recreates containers whose image or compose config changed. Caddy
# and Prometheus read bind-mounted config files the scp step just replaced,
# and an unchanged container keeps serving the config it loaded at start —
# so a Caddyfile or prometheus.yml change would otherwise never go live.
# Caddy validates before applying and keeps the old config on error; the
# non-zero exit fails the deploy so the error is seen.
"${COMPOSE[@]}" exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
"${COMPOSE[@]}" kill -s SIGHUP prometheus

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
