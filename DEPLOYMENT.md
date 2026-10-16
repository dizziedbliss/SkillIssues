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
ingestd:8002 ──→ postgres:5432                           (internal network only; POST /sync/*
                                                            takes ADMIN_SYNC_TOKEN; holds no
                                                            user-token, webhook or OAuth secret)
migrate                                                   — one-shot, runs to completion first

monitoring (see "Monitoring" below):
prometheus ──→ go-api:8000/metrics, ingestd:8002/metrics, caddy:9180, exporters
grafana    ──→ prometheus, postgres (as read-only metrics_ro)
127.0.0.1:3000 (grafana), 127.0.0.1:9090 (prometheus) — loopback only, reach over ssh -L
```

Everything except `caddy` is unreachable from outside the Docker network. `go-api` and
`ingestd` have no published ports in `compose.prod.yaml` at all; Grafana and Prometheus publish
on the host's loopback interface only.

## CI/CD pipeline

```
push to main
   │
   ▼
.github/workflows/deploy.yml
   │
   ├─ job `ci`          — uses: ./.github/workflows/ci.yml (backend/web/desktop/images)
   ├─ job `build-push`  — needs: ci — builds the api/ingestd/migrate/web targets, pushes
   │                       ghcr.io/dizziedbliss/skillissues-{api,ingestd,migrate,web}
   │                       tagged sha-<short> AND latest (linux/amd64 only)
   └─ job `deploy`      — needs: build-push — scp's compose.prod.yaml, Caddyfile,
                           monitoring/ and scripts/deploy-remote.sh to the host, then
                           ssh's in to run it
```

A deploy cannot happen without CI passing — `build-push` and `deploy` both `needs:` their way
back to the `ci` job. `scripts/deploy-remote.sh` does the actual
`pull && up -d --remove-orphans`, reloads Caddy and Prometheus (an unchanged container keeps
the config it loaded at start, so a new Caddyfile or `prometheus.yml` would otherwise never take
effect; an invalid Caddyfile fails the deploy and Caddy keeps its old config), waits for
`go-api`'s healthcheck, and prunes old images —
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

The shape of the production environment. Before you move to SOPS (*Secrets*, below) you create
this by hand on the host, **mode `600`, owned by the deploy user**; after, it is generated from
`secrets/prod.env.sops` on every deploy and this block is what that file contains:

```bash
ENV=production
DOMAIN=your.production.domain
FRONTEND_URL=https://your.production.domain
GITHUB_REDIRECT_URI=https://your.production.domain/api/auth/github/callback

POSTGRES_DB=skillissues
POSTGRES_USER=skillissues
POSTGRES_PASSWORD=<generate: openssl rand -base64 32 | tr -d '/+='>

ADMIN_SYNC_TOKEN=<generate: openssl rand -hex 32>
GITHUB_WEBHOOK_SECRET=<generate: openssl rand -hex 32>

GITHUB_LOGIN_CLIENT_ID=<production Login app client ID>
GITHUB_LOGIN_CLIENT_SECRET=<production Login app client secret>

GITHUB_APP_ID=<production GitHub App ID>
GITHUB_APP_PRIVATE_KEY_HOST_PATH=/opt/skillissues/secrets/github-app-private-key.pem

GITHUB_TOKEN=<optional PAT, used as a server-crawl credential fallback>

GRAFANA_ADMIN_PASSWORD=<generate: openssl rand -base64 24 | tr -d '/+='>
GRAFANA_SECRET_KEY=<generate: openssl rand -hex 32>
METRICS_DB_PASSWORD=<generate: openssl rand -base64 32 | tr -d '/+='>
API_DB_PASSWORD=<generate: openssl rand -base64 32 | tr -d '/+='>
INGEST_DB_PASSWORD=<generate: openssl rand -base64 32 | tr -d '/+='>

IMAGE_TAG=latest   # deploy.yml always overrides this with an explicit sha- tag; this
                   # default exists only so the file is runnable by hand for testing
```

`FRONTEND_URL` **must** start with `https://` — the session cookie is only marked `Secure`
when it does, and that's what makes the OAuth flow work same-origin. `ENV=production` makes
the config guard in `internal/config/env.go` refuse to start on a placeholder
`ADMIN_SYNC_TOKEN`/`GITHUB_WEBHOOK_SECRET`, and `compose.prod.yaml`
itself refuses to start without `POSTGRES_PASSWORD` set at all (`:?` in the compose file) — a
missing production secret fails closed rather than silently running on a dev default.

Register the **Login app** at <https://github.com/settings/applications/new> as an OAuth App
with callback `https://<domain>/api/auth/github/callback`, **Enable Device Flow** ticked, and
request no scopes. Then register the GitHub App at <https://github.com/settings/apps/new> for
the crawl's installation token only: no callback URL, no user authorization, permissions
Metadata read and Issues read.

A SkillIssues session ends only on logout, expiry (`api.session.ttl_hours`), "log out
everywhere" or account purge. The server holds no GitHub token, so revoking the Login app on
GitHub (<https://github.com/settings/applications>) does not end a session.

### Secrets (SOPS + age)

The production `.env` lives in the repo as **`secrets/prod.env.sops`, encrypted** to two age keys:
yours (so you can edit it) and the VPS's (so the host can decrypt it). `deploy.yml` copies the
file to the host still encrypted, and `deploy-remote.sh` decrypts it into `.env` (mode 600) with
`decrypt-env.sh`. **CI never holds a key, so a compromised workflow sees only ciphertext.** A file
that is present but will not decrypt fails the deploy; it never falls back to a stale `.env`. A
host with no encrypted file at all keeps the `.env` already on it and logs a warning, which is how
you migrate without a flag day.

Once migrated, **`.env` on the host is generated: edits made there are overwritten on the next
deploy.** Change secrets in the encrypted file.

One-time setup:

1. **Your key** (on your machine): `age-keygen -o ~/.secrets/skillissues.key`. Note the public key
   (`age1...`); keep the private key in a password manager.
2. **The VPS key** (root, once):
   ```bash
   sudo install -d -m 750 -o root -g deploy /etc/skillissues
   sudo age-keygen -o /etc/skillissues/age.key       # prints the public key
   sudo chown root:deploy /etc/skillissues/age.key && sudo chmod 640 /etc/skillissues/age.key
   ```
   and install `age` and `sops` (the sops release binary from
   <https://github.com/getsops/sops/releases>, `age` from `apt`).
3. **Put both public keys in `.sops.yaml`** and commit it.
4. **Encrypt the current `.env`** (copy it off the host over ssh, encrypt, then shred the copy):
   ```bash
   scp deploy@host:/opt/skillissues/.env ./prod.env
   sops encrypt --filename-override secrets/prod.env.sops \
     --input-type dotenv --output-type dotenv prod.env > secrets/prod.env.sops
   shred -u prod.env
   git add secrets/prod.env.sops && git commit -m "chore(secrets): encrypt production .env" && git push
   ```
   `*.env` is gitignored, but check the plaintext is gone from your disk. CI runs
   `scripts/check-secrets-encrypted.sh`, which fails the build if anything under `secrets/` is
   not SOPS-encrypted or an age private key is committed anywhere.

Day to day:

```bash
SOPS_AGE_KEY_FILE=~/.secrets/skillissues.key \
  sops edit --input-type dotenv --output-type dotenv secrets/prod.env.sops   # then commit and push
```

**Rotating the VPS key:** generate a new key on the host, add its public key to `.sops.yaml`, run
`sops updatekeys --input-type dotenv secrets/prod.env.sops`, commit and deploy, then remove the old
public key and `updatekeys` again. **Revoking a recipient** is the same last step; note that anyone
who held that key could already have read the secrets in earlier versions of the file in git
history, so rotate the secrets themselves if a key was exposed.

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

Each night the host dumps the database, compresses it and **encrypts it with age to a public key
whose private half never touches the server**. The server can write backups but cannot read them
back, so taking the server gets an attacker neither a usable backup nor the means to decrypt one.

Where the encrypted dump goes is a choice you make explicitly with `BACKUP_DESTINATION` in the
encrypted `.env` (there is no default; unset is an error):

| `BACKUP_DESTINATION` | What happens | Survives the VPS dying? |
|---|---|---|
| `local` | Kept in `$DEPLOY_PATH/backups` on the server, pruned after `BACKUP_RETENTION_DAYS` (default 14). **You pull them to another machine.** | **Only once you have pulled them** |
| `r2` | Uploaded to a Cloudflare R2 bucket; retention is an R2 lifecycle rule | Yes |

Start with `local` if you have no R2 account yet, and switch later by changing that one line
(plus the `R2_*` keys). **While on `local`, the only off-site copies are the ones you pull**, so
do it regularly. A run prints `LOCAL ONLY: no off-site copy exists` as a reminder.

One-time setup:

1. **Make the backup key pair on your own machine** (not the VPS):
   `mkdir -p ~/.secrets && chmod 700 ~/.secrets && age-keygen -o ~/.secrets/skillissues-backup.key`
   prints the public key (`age1...`). Keep the private key in a password manager and a second
   place; **losing it means every backup is unreadable.** Put only the public key in `.env` as
   `BACKUP_AGE_RECIPIENT`, and `BACKUP_DESTINATION=local`.
2. **Install the tools and the timer on the host** (root, once; `rclone` only for `r2`):

```bash
ssh <admin>@host
sudo apt-get install -y age            # plus rclone if you will use R2
sudo cp /opt/skillissues/systemd/skillissues-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now skillissues-backup.timer
sudo systemctl start skillissues-backup.service    # first run now
journalctl -u skillissues-backup.service -n 20     # should say "Wrote ... (encrypted)"
```

3. **Pull the files to your machine** (from your PC; use your ssh port and `DEPLOY_PATH`):

```bash
mkdir -p ~/skillissues-backups
rsync -av --ignore-existing -e "ssh -p <port>" deploy@host:/opt/skillissues/backups/ ~/skillissues-backups/
```

   `--ignore-existing` keeps what you already have even after the server prunes it. Run it by
   hand or from a scheduled task on a machine that is on most days; a laptop that is off for two
   weeks misses the 14-day window. The files are age-encrypted, so the copy needs no more care
   than any other file, but only the private key can open them.

**Using R2 later.** Create a private bucket, an API token scoped to that one bucket with *Object
Read & Write* (R2 has no write-only permission, so protect the backups with a bucket lock rule
and expire them with a lifecycle rule), then set `BACKUP_DESTINATION=r2`, `R2_ENDPOINT`
(`https://<account id>.r2.cloudflarestorage.com`), `R2_BUCKET`, `R2_ACCESS_KEY_ID` and
`R2_SECRET_ACCESS_KEY`, and `sudo apt-get install -y rclone` on the host.

`deploy.yml` keeps `backup-db.sh` and the unit files current; only installing them is manual.

The script never keeps an unencrypted copy and never reports success it did not achieve: a
missing setting or tool, a failed dump or (for `r2`) a failed upload exits non-zero, so the unit
shows `failed` (`systemctl --failed`). A dump that could not be uploaded stays on the host, still
encrypted, and is retried on the next run. Plaintext `.sql.gz` dumps from before this change are
left alone (the script warns about them): delete them once a restore has worked.

A dump holds no GitHub credential (the server stores none) but does hold every user's GitHub id,
login, avatar, preferences and saved issues, so treat it as personal data.

**Do a restore drill** before relying on any of this; see *Restore*.

## Changing a live setting

Everything that isn't a secret and isn't fixed at process start (see `CLAUDE.md`'s
*Configuration* section for the exact boundary) lives in the `settings` table and can be
changed on a running system, no restart, no redeploy:

`/admin/*` is internal only: Caddy returns 404 for `/api/admin/...`, so it is not reachable
from the internet. From the host, go through the Docker network:

```bash
ssh deploy@host
cd /opt/skillissues
set -a; . ./.env; set +a    # for ADMIN_SYNC_TOKEN
docker compose -f compose.prod.yaml --env-file .env exec caddy \
  wget -qO- --header "Authorization: Bearer $ADMIN_SYNC_TOKEN" http://go-api:8000/admin/config
```

To change a value, use `psql` on the host (hits the same validation trigger — useful when a bad value
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
The Grafana *Ingestion* dashboard plots `pg_database_size()` against the cap and shows whether
ingestion is paused, so the ceiling is visible before it's hit. Without Grafana,
`/health/detail` reports `db_size_mb`, `max_db_size_mb` and `ingestion_paused` (internal only —
see *Monitoring* for how to reach it).

## Database roles

Three database users exist, and only one is powerful:

| Role | Used by | Can |
|---|---|---|
| `$POSTGRES_USER` (the owner, a superuser) | `migrate`, the `app-roles` and `metrics-role` one-shots, `pg_dump` backups, you | everything |
| `skillissues_api` | `go-api` | read and write what the API serves: accounts, sessions, saved issues, notifications, webhook state changes, settings edits |
| `skillissues_ingest` | `ingestd`, and `cmd/repoimport` if you run it with this role | the crawl: repositories, issues, ingest jobs, the GitHub budget; reads settings, saved issues and the exclusion list |
| `metrics_ro` | Grafana, postgres-exporter | read-only, see *Monitoring* |

Neither service role owns a table, can `DROP`, `ALTER`, `TRUNCATE` or create anything, or run
commands on the host (`COPY ... PROGRAM`). A SQL injection in the API can therefore change
rows it could already change; it cannot destroy the schema or read the crawl's internals.

Passwords come from `API_DB_PASSWORD` and `INGEST_DB_PASSWORD`; the `app-roles` one-shot applies
them on every deploy, after `migrate` and before the services start, so rotating one is a
`.env` edit and a redeploy. A deploy refuses to start (changing nothing) while either is unset.

Grants are explicit per table, with no default privileges. **A migration that adds a table
must grant it** to the roles that use it (and `USAGE` on its sequence), or the service gets
`permission denied` at runtime. `internal/migrate/roles_test.go` fails in CI when a table is
granted to neither role and not listed there as deliberately ungranted.

## Monitoring

Prometheus, Grafana, node-exporter, cAdvisor, postgres-exporter and redis-exporter run as part
of `compose.prod.yaml`. Nothing is public: Grafana and Prometheus bind to `127.0.0.1`, and
Caddy returns 404 for `/api/metrics`, `/api/health/detail` and `/api/admin/*`.

**Open the dashboards**

```bash
ssh -L 3000:localhost:3000 -L 9090:localhost:9090 deploy@host
# then browse http://localhost:3000 — user admin, password GRAFANA_ADMIN_PASSWORD
# Prometheus targets: http://localhost:9090/targets
```

Six provisioned dashboards, in the *SkillIssues* folder. They're read-only in the UI; edit the
JSON under `monitoring/grafana/dashboards/` and redeploy.

| Dashboard | Shows |
|---|---|
| Overview | every scrape target up/down, req/s, 5xx ratio, p95, GitHub points left, ingestion paused, DB size, signed-in users |
| API & Network | status classes, p50/p95/p99, busiest and slowest routes, Caddy edge traffic, host NIC and per-container network |
| Ingestion | queue by status/type, stuck leases, sync coverage, job outcomes, issues/min, worker pools, failure classes, DB size vs cap |
| GitHub API | points remaining per lane vs the floor, reset time, secondary-limit pauses, requests by operation/code, latency |
| Product | users, sign-ups, sessions, contributions started/merged, funnel, levels, XP, catalog by difficulty, most-saved repos |
| Infrastructure | host CPU/memory/disk, container memory vs `mem_limit`, Go runtime, DB pool, Postgres, Redis, settings changes |

Product panels query Postgres as `metrics_ro` (migration 00017): read-only, a 15s
`statement_timeout`, and no access to `auth_sessions.token` or `oauth_states`. The `metrics-role`
one-shot sets its password from `METRICS_DB_PASSWORD` on every deploy. A new table is
invisible to dashboards until a migration grants it.

Prometheus keeps `PROMETHEUS_RETENTION_TIME` (15d) or `PROMETHEUS_RETENTION_SIZE` (2GB),
whichever is hit first. No alerting is configured yet.

**Rotating the monitoring secrets.** `METRICS_DB_PASSWORD` is re-applied by `metrics-role` on
every deploy, so changing it in `.env` and redeploying is enough. `GRAFANA_ADMIN_PASSWORD` is
only read the first time Grafana initialises its volume; to change it afterwards:

```bash
dc exec grafana grafana cli admin reset-admin-password '<new password>'
```

`GRAFANA_SECRET_KEY` encrypts what Grafana stores — today only the `metrics_ro` datasource
password, which provisioning rewrites (with the current key) on every start. So changing it in
`.env` and redeploying is enough; this was tested on an initialised volume, with both datasources
healthy afterwards. A secret added by hand in the UI would be lost on a key change. If Grafana
logs decryption errors after one, start it on a fresh volume — everything it needs comes back
from `monitoring/grafana/provisioning` and `.env`, including `GRAFANA_ADMIN_PASSWORD`:

```bash
dc stop grafana && dc rm -f grafana
docker volume rm skillissues-prod_grafana_data
dc up -d grafana
```

Every one of these must be its own value — never reuse `POSTGRES_PASSWORD` or
`ADMIN_SYNC_TOKEN`. `scripts/deploy-remote.sh` refuses to deploy (and changes nothing) when a
required key is missing from `.env`.

**From the command line** (on the host, `cd $DEPLOY_PATH`; the api/ingestd images are
distroless, so in-network requests go through the Caddy container's `wget`):

```bash
alias dc='docker compose -f compose.prod.yaml --env-file .env'
dc ps                                                    # every service, with health
docker stats --no-stream                                 # CPU / memory / net I/O per container
dc exec caddy wget -qO- http://go-api:8000/health/detail
dc exec caddy wget -qO- http://ingestd:8002/sync/status  # worker pools, DB size, paused?
dc exec caddy wget -qO- http://go-api:8000/metrics  | grep -v '^#'
dc exec caddy wget -qO- http://ingestd:8002/metrics | grep -E '^(ingest_|github_)'
dc logs --since 1h caddy | jq -r 'select(.status>=500) | .request.uri' | sort | uniq -c
dc exec postgres psql -U skillissues -d skillissues      # then the SQL below
```

```sql
SELECT job_type, status, count(*) FROM ingest_jobs GROUP BY 1, 2 ORDER BY 1, 2;
SELECT last_error_class, count(*) FROM ingest_jobs WHERE status = 'failed' GROUP BY 1 ORDER BY 2 DESC;
SELECT lane, remaining, rate_limit, reset_at, secondary_until FROM github_budget WHERE identity = 'installation';
SELECT state, count(*) FROM contributions GROUP BY 1;
SELECT count(*) FILTER (WHERE created_at > now() - interval '7 days') AS new_7d, count(*) AS total FROM users;
SELECT pg_size_pretty(pg_database_size(current_database()));
SELECT key, old_value, new_value, changed_at, changed_by FROM settings_audit ORDER BY changed_at DESC LIMIT 20;
```

## Restore

Backups are files named `skillissues-<timestamp>.sql.gz.age`: in `~/skillissues-backups` if you
pull them, or in R2 under `db/` (download in the R2 dashboard or with `rclone copyto`). Decrypt on
**your own machine**, where the private key is, never on the server:

```bash
age -d -i ~/.secrets/skillissues-backup.key ~/skillissues-backups/skillissues-<timestamp>.sql.gz.age | gunzip > backup.sql
scp backup.sql deploy@host:/tmp/restore.sql    # delete it from the host afterwards
```

Then on the host, stop the services so nothing writes mid-restore, load the dump, and start them
again:

```bash
cd /opt/skillissues
docker compose -f compose.prod.yaml --env-file .env stop go-api ingestd
docker compose -f compose.prod.yaml --env-file .env exec -T postgres \
  psql -U skillissues -d skillissues < /tmp/restore.sql
shred -u /tmp/restore.sql
docker compose -f compose.prod.yaml --env-file .env start go-api ingestd
```

To rehearse without touching production, create a scratch database first
(`CREATE DATABASE restore_drill`) and load the dump into that instead; check `SELECT count(*)
FROM users, issues` looks right. A backup you have never restored is a hope, not a backup.

## Production security checklist

- [ ] Secrets are in `secrets/prod.env.sops`, not hand-edited on the host, and the VPS age key is
      `root:deploy` mode `640` at `/etc/skillissues/age.key`.
- [ ] A restore from a backup has been rehearsed, the backup private key is held in two places off
      the server, and (if `BACKUP_DESTINATION=local`) the files are being pulled off the server.
- [ ] `.env` is mode `600`, owned by the deploy user, and every `:?`-required secret in
      `compose.prod.yaml` has a real value (the file refuses to start otherwise).
- [ ] DNS is A-record-direct to the VPS, no proxying CDN in front of Caddy.
- [ ] The Login app callback URL matches the production domain.
- [ ] `dc exec caddy wget -qO- http://go-api:8000/health/detail` reports `"cache":"ok"` and
      sane repository/issue counts, and `curl https://<domain>/api/metrics` returns **404**.
- [ ] Grafana opens over the SSH tunnel and the *Overview* dashboard shows every target UP;
      `ss -tlnp | grep -E ':(3000|9090)'` shows them bound to `127.0.0.1` only.
- [ ] `docker compose -f compose.prod.yaml --env-file .env ps` shows every service healthy and
      `migrate` exited `0`.
- [ ] The backup timer is enabled (`systemctl status skillissues-backup.timer` on the host).
