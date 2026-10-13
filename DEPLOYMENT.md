# Production Deployment

This is the actual runbook for the one supported production path: a single VPS running
`compose.prod.yaml`, with Caddy terminating TLS and GitHub Actions building and deploying on
every push to `main`. There is no Kubernetes path and no certbot/nginx path — if you're looking
for either, they don't exist; `k8s/` was deleted because it referenced images nothing built,
pods nothing could probe, and a committed plaintext database password.

## Architecture

```
Internet
   │  80/443
   ▼
caddy            — the only published port; automatic Let's Encrypt
   │
   ├─ /api/* ──→  go-api:8000     (prefix stripped; go-api's routes have no /api of their own)
   └─ /*     ──→  frontend:80     (nginx serving the Vite build)

go-api:8000 ──→ postgres:5432, redis:6379, ingestd:8002 (internal network only)
ingestd:8002 ──→ postgres:5432, redis:6379               (internal network only; no auth of
                                                            its own on /sync/*, so it must
                                                            never be reachable from outside)
migrate                                                   — one-shot, runs to completion first
```

Everything except `caddy` is unreachable from outside the Docker network. `go-api` and
`ingestd` have no published ports in `compose.prod.yaml` at all.

## CI/CD pipeline

```
push to main
   │
   ▼
.github/workflows/deploy.yml
   │
   ├─ job `ci`          — uses: ./.github/workflows/ci.yml (backend/web/desktop/images)
   ├─ job `build-push`  — needs: ci — builds all four root Dockerfile targets, pushes
   │                       ghcr.io/dizziedbliss/skillissues-{api,ingestd,migrate,web}
   │                       tagged sha-<short> AND latest (linux/amd64 only)
   └─ job `deploy`      — needs: build-push — scp's compose.prod.yaml, Caddyfile and
                           scripts/deploy-remote.sh to the host, then ssh's in to run it
```

A deploy cannot happen without CI passing — `build-push` and `deploy` both `needs:` their way
back to the `ci` job. `scripts/deploy-remote.sh` does the actual
`pull && up -d --remove-orphans`, waits for `go-api`'s healthcheck, and prunes old images —
it's a committed, independently-runnable script rather than inline SSH-heredoc logic, so it can
also be run by hand on the host for debugging.

**Rollback**: `workflow_dispatch` on `deploy.yml` with `image_tag` set to an older `sha-<short>`
tag. That skips the `ci`/`build-push` jobs entirely and just redeploys the named tag. Find past
tags under the repo's GHCR packages, or `git log --oneline` the commit you want and prefix it
with `sha-`.

## First-time host setup

Done once per host, by hand — **secrets never pass through CI**, matching
[Conventions → Credentials](CLAUDE.md#credentials-and-attribution).

### 1. GitHub Actions secrets

Set on the repo (`gh secret set NAME` or Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `VPS_HOST` | the VPS's IP or hostname |
| `VPS_USER` | the deploy user (not root — no sudo, by design) |
| `VPS_SSH_PORT` | the VPS's SSH port |
| `VPS_SSH_KEY` | the deploy user's **private** key, full contents |
| `VPS_KNOWN_HOSTS` | `ssh-keyscan -p <port> -t ed25519 <host>`'s output — **verify the printed
  fingerprint (`ssh-keygen -lf`) against what the host's provisioner gave you before trusting
  it**, every time a host is reprovisioned |
| `DEPLOY_PATH` | the directory on the host everything gets copied into, e.g. `/opt/skillissues` |

### 2. `$DEPLOY_PATH/.env`

Create by hand on the host, **mode `600`, owned by the deploy user**:

```bash
ENV=production
DOMAIN=your.production.domain
FRONTEND_URL=https://your.production.domain
GITHUB_REDIRECT_URI=https://your.production.domain/api/auth/github/callback
GITHUB_APP_CALLBACK_URL=https://your.production.domain/api/auth/github-app/callback

POSTGRES_DB=skillissues
POSTGRES_USER=skillissues
POSTGRES_PASSWORD=<generate: openssl rand -base64 32 | tr -d '/+='>

TOKEN_ENCRYPTION_KEY=<generate: openssl rand -base64 48 | tr -d '/+='>
ADMIN_SYNC_TOKEN=<generate: openssl rand -hex 32>
GITHUB_WEBHOOK_SECRET=<generate: openssl rand -hex 32>

GITHUB_CLIENT_ID=<production OAuth App client ID>
GITHUB_CLIENT_SECRET=<production OAuth App client secret>

GITHUB_APP_ID=<production GitHub App ID>
GITHUB_APP_CLIENT_ID=<production GitHub App client ID>
GITHUB_APP_CLIENT_SECRET=<production GitHub App client secret>
GITHUB_APP_PRIVATE_KEY_HOST_PATH=/opt/skillissues/secrets/github-app-private-key.pem

GITHUB_TOKEN=<optional PAT, used as a server-crawl credential fallback>

IMAGE_TAG=latest   # deploy.yml always overrides this with an explicit sha- tag; this
                   # default exists only so the file is runnable by hand for testing
```

`FRONTEND_URL` **must** start with `https://` — the session cookie is only marked `Secure`
when it does, and that's what makes the OAuth flow work same-origin. `ENV=production` makes
the config guard in `internal/config/env.go` refuse to start on a placeholder
`TOKEN_ENCRYPTION_KEY`/`ADMIN_SYNC_TOKEN`/`GITHUB_WEBHOOK_SECRET`, and `compose.prod.yaml`
itself refuses to start without `POSTGRES_PASSWORD` set at all (`:?` in the compose file) — a
missing production secret fails closed rather than silently running on a dev default.

Register the production OAuth App at <https://github.com/settings/applications/new> (callback:
`https://<domain>/api/auth/github/callback`) and the GitHub App at
<https://github.com/settings/apps/new> (callback: `https://<domain>/api/auth/github-app/callback`;
permissions: repository Contents read, Pull requests read/write, Metadata read).

### 3. GitHub App private key

```bash
scp downloaded-key.pem deploy@host:$DEPLOY_PATH/secrets/github-app-private-key.pem
ssh deploy@host chmod 644 $DEPLOY_PATH/secrets/github-app-private-key.pem
```

**Must be PKCS1** (`-----BEGIN RSA PRIVATE KEY-----`) — `githubapp.LoadPrivateKeyFile` only
parses `x509.ParsePKCS1PrivateKey`; a PKCS8 key (`BEGIN PRIVATE KEY`) fails to load.

**Must be world-readable** (`644`, not `600`) — the `api`/`ingestd` images run as the
distroless `nonroot` user (uid `65532`), which has no relation to the `deploy` user that owns
the file on the host filesystem; a `600` root- or deploy-owned file silently breaks the App
with no useful error until you look at the container's own startup log.

### 4. DNS

Point the production domain's **A record directly at the VPS's IP**, DNS-only (no CDN/proxy in
front of it). Caddy's automatic HTTPS needs the ACME HTTP-01 challenge to reach port 80 on the
origin directly; a proxying CDN in front of it (Cloudflare's orange-cloud, for example)
intercepts that and the certificate issuance fails.

### 5. First deploy

Push to `main`, or run `deploy.yml` manually (Actions tab → Run workflow, leave `image_tag`
blank to build fresh). Watch the `deploy` job's logs — `scripts/deploy-remote.sh` prints
`go-api is healthy.` and `Deploy of <tag> complete.` on success, or the container's own last
200 log lines on failure.

### 6. Backups

```bash
ssh deploy@host
sudo cp /opt/skillissues/systemd/skillissues-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now skillissues-backup.timer
```

Needs root — the `deploy` user has none, by design, so this one step is done by whoever
provisioned the host. Writes a daily gzip'd `pg_dump` to `$DEPLOY_PATH/backups`, 14-day
retention (`BACKUP_RETENTION_DAYS` in `.env` to change it).

**A dump is useless without `TOKEN_ENCRYPTION_KEY`** — it decrypts `users.github_access_token`
at rest, and it lives only in `.env`, which these dumps don't include. Back that value up
separately (a password manager, not this directory) the moment you generate it, not after the
first incident.

## Changing a live setting

Everything that isn't a secret and isn't fixed at process start (see `CLAUDE.md`'s
*Configuration* section for the exact boundary) lives in the `settings` table and can be
changed on a running system, no restart, no redeploy:

```bash
curl https://<domain>/api/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"
curl -XPUT https://<domain>/api/admin/config/ingest.max_db_size_mb \
     -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 32000}'
```

Or directly via `psql` on the host (hits the same validation trigger — useful when a bad value
has made the API itself unhealthy):

```bash
ssh deploy@host
cd /opt/skillissues
docker compose -f compose.prod.yaml --env-file .env exec postgres \
  psql -U skillissues -d skillissues \
  -c "UPDATE settings SET value = '32000' WHERE key = 'ingest.max_db_size_mb'"
```

## Disk

The host is 40 GB. Two live guardrails exist specifically so ingestion can't fill it:
`ingest.max_db_size_mb` (default 30000 — stops the coordinator from enqueuing or claiming new
work once `pg_database_size()` crosses it) and `ingest.max_pages_per_repo` (default 100 — caps
how many pages the Deep Diver will fetch for one repository, regardless of `hasNextPage`).
`GET /api/health/detail` reports `db_size_mb`, `max_db_size_mb` and `ingestion_paused` so the
ceiling is visible before it's hit.

## Monitoring

`GET /api/metrics` (Prometheus format) and `GET /api/health/detail` are exempt from rate
limiting and reachable through Caddy. `ingestd`'s own `/sync/status` is internal-only (no
Caddy route to it) — `ssh` in and `curl localhost:8002/sync/status` from the host, or add a
route if you want it exposed.

## Restore

```bash
gunzip -c /opt/skillissues/backups/skillissues-<timestamp>.sql.gz | \
  docker compose -f compose.prod.yaml --env-file .env exec -T postgres \
  psql -U skillissues -d skillissues
```

Stop `go-api` and `ingestd` first (`docker compose -f compose.prod.yaml --env-file .env stop
go-api ingestd`) so nothing writes mid-restore, then start them back up afterward. The restored
database only decrypts existing users' GitHub tokens correctly if `TOKEN_ENCRYPTION_KEY` in the
current `.env` matches what was active when the backup was taken.

## Production security checklist

- [ ] `.env` is mode `600`, owned by the deploy user, and every `:?`-required secret in
      `compose.prod.yaml` has a real value (the file refuses to start otherwise).
- [ ] `TOKEN_ENCRYPTION_KEY` is backed up somewhere other than this host.
- [ ] DNS is A-record-direct to the VPS, no proxying CDN in front of Caddy.
- [ ] GitHub OAuth App and GitHub App callback URLs match the production domain.
- [ ] `GET /api/health/detail` reports `"cache":"ok"` and sane repository/issue counts.
- [ ] `docker compose -f compose.prod.yaml --env-file .env ps` shows every service healthy and
      `migrate` exited `0`.
- [ ] The backup timer is enabled (`systemctl status skillissues-backup.timer` on the host).
