# SkillIssues operator runbook

Everything you do by hand to run SkillIssues in production, in the order you'll need it. Copy,
paste, read the "you should see" line. If what you see differs, go to
[Troubleshooting](#7-troubleshooting) before doing anything else.

`DEPLOYMENT.md` is the reference (why things are the way they are). This file is the checklist.

---

## 0. Read this first

**Two machines. Every block below says which one.**

| Name used here | What it is | Its hostname | Login |
|---|---|---|---|
| **PC** | your own computer (WSL) | `somedreams` | you |
| **VPS** | the production server | `winter` | `deploy` (no sudo) or your admin user `dizziedbliss` (has sudo) |

**Not sure which machine a terminal is on? Run `uname -n`.** It prints `somedreams` (PC) or
`winter` (VPS). Both machines have a user called `dizziedbliss`, so the username does not tell
you. Check before running anything that creates keys or changes a password.

`deploy` is the user CI and the scripts use. It has **no sudo** on purpose. Anything needing
`sudo` is done as your admin user.

**Your PC shell is fish; the blocks below are bash.** If a command errors strangely (wildcards,
`$(...)`, `VAR=x cmd`), type `bash` first. On the VPS you're already in bash.

**Five rules that prevent almost every disaster:**

1. **Never edit `/opt/skillissues/.env` on the VPS.** It is rewritten from the encrypted file on
   every deploy, so your edit disappears. Change secrets with `sops edit` ([§4](#4-change-a-secret)).
2. **Never push to `main`.** Branch, push, open a PR, merge. Merging to `main` *is* a deploy.
3. **Private keys never go on the VPS** (except the VPS's own key, which is made *there*). Keys A
   and C live on your PC only.
4. **Don't change a password in `.env` and stop.** The database keeps its old password until you
   change it *inside* Postgres too ([§8](#8-rotate-passwords-and-keys)).
5. **A backup you haven't restored is a hope.** Do the restore check ([§5.3](#53-prove-a-backup-works-do-this-often)).

---

## 1. One-time: stop retyping the ssh options

On the **PC**, add this to `~/.ssh/config` (create it if missing; replace the `<...>`):

```
Host skillissues
    HostName <VPS_IP>
    User deploy
    Port <SSH_PORT>
    IdentityFile ~/.ssh/skillissues_deploy

Host skillissues-admin
    HostName <VPS_IP>
    User <your admin user>
    Port <SSH_PORT>
    IdentityFile <your admin key, if different>
```

```bash
chmod 600 ~/.ssh/config
ssh skillissues 'echo connected as $(whoami) on $(hostname)'
```
You should see `connected as deploy on winter`. From now on `ssh skillissues`, `scp skillissues:...`
and `rsync ... skillissues:...` all work with no flags. (This is also why you never hit the
`-p` vs `-P` mix-up again: scp's port flag is capital `-P`; ssh's is lowercase `-p`.)

---

## 2. The map: what lives where

**On the VPS, in `/opt/skillissues`** (everything below is copied there by the deploy):
`compose.prod.yaml`, `Caddyfile`, `monitoring/`, `db/`, `systemd/`, `scripts`,
`secrets/prod.env.sops` (encrypted), `.env` (generated; plaintext; mode 600), `backups/`.

**The keys: know all four and where each lives.**

| Key | Public half goes in | Private half lives | Lose it and… |
|---|---|---|---|
| **A**: your SOPS key | `.sops.yaml` | PC `~/.secrets/skillissues-sops.key` + password manager | you can't edit secrets (recoverable via B, [§9](#9-emergencies)) |
| **B**: VPS SOPS key | `.sops.yaml` | VPS `/etc/skillissues/age.key` | deploys fail at "decrypt" (recoverable with A) |
| **C**: backup key | `.env` as `BACKUP_AGE_RECIPIENT` | PC `~/.secrets/skillissues-backup.key` + a second safe place | **every backup is unreadable. Not recoverable.** |
| SSH key | the VPS's `authorized_keys` | PC `~/.ssh/skillissues_deploy` | you can't log in |

`age-keygen -y <file>` prints the public key of a private key file (safe to share).

**Alias to use on the VPS** (put it in `~/.bashrc` of `deploy` once):

```bash
alias dc='docker compose -f compose.prod.yaml --env-file .env'
```
It only works inside `/opt/skillissues`, so `cd /opt/skillissues` first.

---

## 3. Deploys

### 3.1 What happens when you merge a PR to `main`

CI runs → images are built → files are copied to the VPS → `deploy-remote.sh` decrypts the
secrets into `.env`, runs `migrate` (database schema), `app-roles` (database passwords), starts
the services, and waits until the API reports healthy. Takes ~5-8 min (about 1 min if images are cached).

**Watch it:** GitHub → Actions → the newest "Build, Publish and Deploy" run. A green tick =
deployed *and* healthy.

### 3.2 Is production healthy right now?

On the **VPS** (`ssh skillissues`):
```bash
cd /opt/skillissues && alias dc='docker compose -f compose.prod.yaml --env-file .env'
dc ps
```
You should see every service `Up`, and `go-api`, `ingestd`, `postgres`, `redis` as `(healthy)`.
`migrate`, `app-roles` and `metrics-role` show `Exited (0)`: that's correct, they're one-shots.

```bash
dc exec -T caddy wget -qO- http://go-api:8000/health/detail
```
You should see JSON with `"status":"ok"`, `"cache":"ok"`, row counts, and `"ingestion_paused":false`.
(`/health/detail` and `/metrics` are blocked from the internet on purpose, so you check them
from inside.)

### 3.3 A deploy failed

1. **Read the log.** Actions → the red run → the red step. The last lines tell you which stage.
2. **Look up the message in [Troubleshooting](#7-troubleshooting).** The common ones are there.
3. **Fix the cause, then re-run only the failed job**: the run page → **Re-run failed jobs**.
   (Or `gh run rerun <run-id> --failed` on the PC.)

A failed deploy *before* "Container ... Started" changes nothing. A failure *after* compose has
recreated containers (for example at `migrate`) can leave `go-api`/`ingestd` created but not
started, so check [§3.2](#32-is-production-healthy-right-now) and fix it quickly.

### 3.4 Roll back to an older version

On the **PC**:
```bash
git log --oneline -10                      # pick the commit you want; its short hash is the tag
gh workflow run deploy.yml -f image_tag=sha-<short hash>
```
Watch it in Actions. **A rollback swaps the program, not the database.** Migrations only go
forward, and some are destructive (`00027` deleted all user data). If the bad release ran a
migration, rolling the code back does not undo it; ask before relying on a rollback in that case.

---

## 4. Change a secret

This is the **only** way to change anything in `.env`: new password, new token, a setting like
`BACKUP_RETENTION_DAYS`. Do it on the **PC**, in the repo.

```bash
cd ~/github/SkillIssues
git checkout main && git pull
git checkout -b chore/secrets-<what you're changing>

env EDITOR=nano SOPS_AGE_KEY_FILE=$HOME/.secrets/skillissues-sops.key \
  sops edit --input-type dotenv --output-type dotenv secrets/prod.env.sops
```
Your editor opens with the **decrypted** file. Change or add lines (`NAME=value`, no spaces around
`=`). Save and exit (nano: `Ctrl+O`, `Enter`, `Ctrl+X`). Then:

```bash
scripts/check-secrets-encrypted.sh && echo "guard ok"
git add secrets/prod.env.sops
git commit -m "chore(secrets): <what changed>"
git push -u origin HEAD
```
Open the PR on GitHub, merge it, watch the deploy. The VPS decrypts the new file into `.env`.

**See what's in the file without editing** (values hidden):
```bash
env SOPS_AGE_KEY_FILE=$HOME/.secrets/skillissues-sops.key \
  sops decrypt --input-type dotenv --output-type dotenv secrets/prod.env.sops | sed 's/=.*/=<hidden>/'
```

**Some secrets need a second step** (the value also lives somewhere else). See [§8](#8-rotate-passwords-and-keys).

---

## 5. Backups

**What runs:** every night at 03:17 UTC the VPS dumps the database, compresses it, encrypts it to
key C's public key, and keeps it in `/opt/skillissues/backups` for `BACKUP_RETENTION_DAYS` days
(default 14; recommended **3**). One dump is ~0.7 GB and takes ~3 minutes.

**Your backups are only as safe as your copies.** The server holds a few days; **the copy you
pull to your PC is the real one.** There is no off-site copy yet (no R2 account).

### 5.1 Pull the backups to your PC (every few days)

On the **PC**:
```bash
mkdir -p ~/skillissues-backups
rsync -av --ignore-existing skillissues:/opt/skillissues/backups/ ~/skillissues-backups/
ls -lh ~/skillissues-backups/
```
**Wait for the prompt to come back** (each file is ~700 MB). You should see `.sql.gz.age` files.
`--ignore-existing` means files you already have are kept even after the server deletes them.

### 5.2 Take a backup right now (optional)

On the **VPS**, as your admin user:
```bash
sudo systemctl start --no-block skillissues-backup.service
sudo journalctl -fu skillissues-backup.service        # Ctrl+C to stop watching
```
You should see `Wrote ... (encrypted)` after ~3 min. (`--no-block` returns immediately; without
it `start` waits and looks frozen.)

### 5.3 Prove a backup works (do this often)

On the **PC** (in bash):
```bash
f=$(ls -t ~/skillissues-backups/skillissues-*.sql.gz.age | head -1)
age -d -i ~/.secrets/skillissues-backup.key "$f" | gunzip | head -5
age -d -i ~/.secrets/skillissues-backup.key "$f" | gunzip | grep -c '^COPY public\.'
```
First command: readable text starting `-- PostgreSQL database dump` (a `Broken pipe` line after is
harmless). Second: a number around 15-20 (one per table). If both look right, the backup is real.

### 5.4 Restore into a scratch database on your PC (every few months, or before risky work)

This is the full test. It loads the whole dump into your local Docker Postgres, in a *separate*
database so nothing real is touched. Needs a few GB of free disk.

```bash
cd ~/github/SkillIssues
docker compose up -d postgres
docker compose exec -T postgres psql -U skillissues -d postgres -c "CREATE DATABASE restore_drill"
f=$(ls -t ~/skillissues-backups/skillissues-*.sql.gz.age | head -1)
age -d -i ~/.secrets/skillissues-backup.key "$f" | gunzip | docker compose exec -T postgres psql -U skillissues -d restore_drill -q 2>&1 | tail -5
docker compose exec -T postgres psql -U skillissues -d restore_drill -tAc "select (select count(*) from users) as users, (select count(*) from issues) as issues, (select count(*) from repositories) as repos"
docker compose exec -T postgres psql -U skillissues -d postgres -c "DROP DATABASE restore_drill"
```
`role "skillissues_api" does not exist`-style errors on `GRANT` lines are expected locally
(those roles only exist on the server) and harmless. The three counts should look like production.

### 5.5 Real restore after a disaster (written from the design; not yet rehearsed)

Use this only if production data is actually lost. Decrypt on the **PC** (§5.3), then:

```bash
gunzip -c <(age -d -i ~/.secrets/skillissues-backup.key "$f") > restore.sql     # on the PC
scp restore.sql skillissues:/tmp/restore.sql
```
On the **VPS** (`cd /opt/skillissues`, alias `dc` set):
```bash
dc stop go-api ingestd
dc stop grafana postgres-exporter        # they hold connections that would block the DROP
dc exec -T postgres psql -U skillissues -d postgres -c "DROP DATABASE skillissues WITH (FORCE)" -c "CREATE DATABASE skillissues"
dc exec -T postgres psql -U skillissues -d skillissues -q < /tmp/restore.sql
shred -u /tmp/restore.sql
dc up -d
```
Then re-run the latest deploy so `app-roles` re-applies the role passwords.

**On a brand-new server** the database roles don't exist yet (a dump doesn't carry roles). Create
them *before* loading the dump:
```bash
dc exec -T postgres psql -U skillissues -d postgres -c "CREATE ROLE skillissues_api NOLOGIN" -c "CREATE ROLE skillissues_ingest NOLOGIN" -c "CREATE ROLE metrics_ro NOLOGIN" -c "ALTER ROLE metrics_ro SET statement_timeout='15s'" -c "ALTER ROLE metrics_ro SET default_transaction_read_only=on" -c "GRANT pg_monitor TO metrics_ro"
```

### 5.6 Housekeeping

After a restore check has worked, on the **VPS** delete leftovers (the encrypted `.age` files stay):
```bash
rm -f /opt/skillissues/backups/skillissues-*.sql.gz      # old UNencrypted dumps from before encryption
rm -f /opt/skillissues/.env.pre-sops                      # old secrets in plaintext
```

### 5.7 Switch to Cloudflare R2 later (when you have an account)

1. R2 → create a private bucket; set a bucket lock (14 days) and a lifecycle delete (30 days) on prefix `db/`.
2. R2 → Manage API tokens → create a token: *Object Read & Write*, **only that bucket**. Copy the
   key id and secret (shown once). Note your Account ID.
3. On the **VPS** (admin user): `sudo apt-get install -y rclone`
4. On the **PC**, [§4](#4-change-a-secret): set `BACKUP_DESTINATION=r2` and add
   `R2_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com`, `R2_BUCKET=<name>`,
   `R2_ACCESS_KEY_ID=...`, `R2_SECRET_ACCESS_KEY=...`. PR, merge.
5. Run a backup ([§5.2](#52-take-a-backup-right-now-optional)); the journal should say `Uploaded ... to r2:...`.

---

## 6. Everyday operations

### 6.1 Logs

On the **VPS** (`cd /opt/skillissues`, alias `dc`):
```bash
dc logs go-api --tail 100
dc logs ingestd --tail 100
dc logs -f ingestd                    # follow live; Ctrl+C to stop
dc logs migrate --tail 30             # why a deploy failed at migrate
```
The service logs are JSON. `"level":"ERROR"` or `"WARN"` lines are the ones to read.

### 6.2 Dashboards (Grafana, Prometheus)

On the **PC**:
```bash
ssh -L 3000:localhost:3000 -L 9090:localhost:9090 skillissues
```
Leave it open, browse `http://localhost:3000` (user `admin`, password = `GRAFANA_ADMIN_PASSWORD`).
Known issue: some panels that count every OPEN issue time out (the dashboard role has a 15 s
limit). They show blank; the data is fine.

### 6.3 Change a live setting (no deploy needed)

Operational values (page sizes, worker counts, intervals) live in the database and change
instantly. On the **VPS**:
```bash
cd /opt/skillissues && alias dc='docker compose -f compose.prod.yaml --env-file .env'

# list them all (values and defaults)
set -a; . ./.env; set +a
dc exec -T caddy wget -qO- --header "Authorization: Bearer $ADMIN_SYNC_TOKEN" http://go-api:8000/admin/config

# change one (the database rejects out-of-range values)
dc exec -T postgres psql -U skillissues -d skillissues \
  -c "UPDATE settings SET value = '3' WHERE key = 'ingest.routine_workers'"
```
Every key, default and range is in `docs/settings.md`. Every change is recorded in
`settings_audit`. Caution: don't set `ingest.max_pages_per_repo` to `0` (unlimited) unless you
are sure no enormous repo is crawlable, since the exclusion list only covers repos you've found.

### 6.4 Exclude a repository (never crawl it)

Quick, no deploy (on the **VPS**): effective on the next eligibility recheck.
```bash
dc exec -T postgres psql -U skillissues -d skillissues \
  -c "INSERT INTO excluded_repositories (full_name, reason) VALUES ('owner/name', 'why')"
```
Permanent and reviewed (preferred): copy `backend/internal/migrate/migrations/00030_exclude_cookies_extension.sql`
to the next number (`00031_...`), change the repo name and reason, PR it, merge. The migration
also marks the repo ineligible immediately; ingestd then deletes its stored issues (never ones a
user saved).

Find candidates: repos with an outsized issue count.
```bash
dc exec -T postgres psql -U skillissues -d skillissues -c \
 "select full_name, issue_count, stars from repositories where is_eligible and issue_count > 50000 order by issue_count desc limit 20"
```

### 6.5 Add a new database table (when a future migration creates one)

The API and ingestd connect as restricted roles that **can't see a new table until it's granted**.
In the same migration, add `GRANT SELECT, INSERT, UPDATE ON <table> TO skillissues_api;` (and/or
`skillissues_ingest`), plus `GRANT USAGE ON SEQUENCE <table>_id_seq ...` if it has an `id`
column. CI fails (`internal/migrate/roles_test.go`) if you forget.

---

## 7. Troubleshooting

Every error we hit while setting this up, with the cause and the fix.

| You see | Where | Cause | Fix |
|---|---|---|---|
| `scp: ./prod.env: No such file or directory` (with `-p 6767`) | PC | lowercase `-p` is "preserve times"; the **port flag is capital `-P`** | `scp -P 6767 ...` (or use the ssh config, [§1](#1-one-time-stop-retyping-the-ssh-options)) |
| `fish: No matches for wildcard '...*...'` | PC | fish refuses a `*` that matches nothing; the files weren't there yet | wait for `rsync` to finish, or run `bash` first |
| `age-keygen: failed to open output file ... no such file or directory` | PC | `~/.secrets` doesn't exist | `mkdir -p ~/.secrets && chmod 700 ~/.secrets` |
| `age-keygen: ... file exists` | VPS | the key already exists; it refuses to overwrite (good) | don't force it. `sudo age-keygen -y /etc/skillissues/age.key` prints its public key |
| `Error response from daemon: error from registry: denied` during deploy | deploy log | the VPS's saved `ghcr.io` login is invalid/expired (even **public** images are denied when docker sends a bad token) | on the VPS: `docker logout ghcr.io`; if the packages are private, `echo "$PAT" \| docker login ghcr.io -u dizziedbliss --password-stdin` with a classic PAT with `read:packages`. Re-run failed jobs |
| `service "migrate" didn't complete successfully: exit 1` | deploy log | usually the DB owner password in `.env` doesn't match the one inside Postgres. Confirm: `dc logs migrate --tail 15` shows `password authentication failed` | [§8.1](#81-the-database-owner-password-postgres_password) |
| `NOTICE: empty string is not a valid password, clearing password` | VPS | a shell variable was empty, so the command *cleared* a password | stop; check the variable first (`echo ${#VAR}` must not be 0); use the guarded command in §8.1 |
| `... must be set in production` (`API_DB_PASSWORD`, `INGEST_DB_PASSWORD`, etc.) | deploy log | a required secret is missing from the encrypted `.env`. The deploy refuses *before changing anything* | add it ([§4](#4-change-a-secret)) |
| `sops is not installed` / `age key ... missing or unreadable` | deploy log | the VPS lacks `sops`, or `/etc/skillissues/age.key` isn't `root:deploy` mode 640 | install sops; `sudo chown root:deploy /etc/skillissues/age.key && sudo chmod 640 ...`; `sudo -u deploy test -r /etc/skillissues/age.key && echo ok` |
| `MAC mismatch` / `Cannot decrypt MAC` | deploy log or PC | the encrypted file was edited by hand or damaged | restore it from git (`git checkout <good commit> -- secrets/prod.env.sops`), then redo your edit with `sops edit` |
| `no matching creation rules found` | PC, `sops encrypt` | file path doesn't match `.sops.yaml` | add `--filename-override secrets/prod.env.sops` |
| `Error unmarshalling input json` | PC, `sops updatekeys` | it guessed JSON from the `.sops` extension | add `--input-type dotenv` |
| `Failed to get the data key ... no identity matched` | PC | wrong or missing `SOPS_AGE_KEY_FILE` | `env SOPS_AGE_KEY_FILE=$HOME/.secrets/skillissues-sops.key sops ...` |
| `journalctl`: "You are currently not seeing messages from other users" / `-- No entries --` | VPS | your user isn't in the journal group | use `sudo journalctl ...` |
| `systemctl start skillissues-backup.service` seems frozen | VPS | it's a "oneshot": `start` waits until the ~3 min backup finishes | wait, or `start --no-block` and watch `journalctl -fu` |
| `BACKUP_DESTINATION must be 'r2' or 'local' (got '')` | backup journal | the setting isn't in the encrypted `.env` | `BACKUP_DESTINATION=local` via [§4](#4-change-a-secret) |
| `backup is not configured: ... unset` | backup journal | a required backup setting is missing | add it ([§4](#4-change-a-secret)) |
| `LOCAL ONLY: no off-site copy exists` | backup journal | **not an error.** a reminder to pull the files | [§5.1](#51-pull-the-backups-to-your-pc-every-few-days) |
| `age: error: no identity matched` | PC | the backup was encrypted to a different public key than your key C | check `BACKUP_AGE_RECIPIENT` equals `grep "public key" ~/.secrets/skillissues-backup.key` |
| API logs `permission denied for table X` | VPS logs | a new table wasn't granted to the service roles | [§6.5](#65-add-a-new-database-table-when-a-future-migration-creates-one) |
| Grafana panel is blank / `canceling statement due to statement timeout` in Postgres logs | Grafana | the dashboard database role has a 15 s limit and the query is heavier | cosmetic; data is fine; needs an index or a cached gauge |
| `gh pr edit` fails with "token has not been granted the required scopes" | PC | `gh`'s token lacks `read:org` | use `gh api -X PATCH repos/dizziedbliss/SkillIssues/pulls/<n> -F body=@file.md` |
| `settings keys absent from the database; using defaults` in logs | VPS logs | a setting is in the code registry but no migration seeded it | add a seeding migration (CLAUDE.md, Configuration) |

---

## 8. Rotate passwords and keys

### 8.1 The database owner password (`POSTGRES_PASSWORD`)

**The trap:** Postgres reads this variable only when its data folder is first created. Changing
it in `.env` changes nothing inside the database, so `migrate` then can't log in. Do **both**,
in this order, so a deploy never sees a mismatch:

1. On the **PC**, make a new value: `openssl rand -hex 32` (hex only: it goes into a URL, so
   `/ + = @ :` would break it). Keep it in your terminal/clipboard.
2. On the **VPS** (`cd /opt/skillissues`), set it inside Postgres. This reads it without putting
   it in your shell history, and refuses to run if it's empty:
   ```bash
   read -rs NEWPW; echo "length: ${#NEWPW}"        # paste the value, Enter; must print 64
   if [ -z "$NEWPW" ]; then echo "EMPTY - stop"; else
     docker compose -f compose.prod.yaml --env-file .env exec -T postgres \
       psql -U skillissues -d skillissues -c "ALTER USER skillissues PASSWORD '$NEWPW'"
     docker compose -f compose.prod.yaml --env-file .env exec -T -e PGPASSWORD="$NEWPW" postgres \
       psql -h 127.0.0.1 -U skillissues -d skillissues -tAc "select 'password login works'"
   fi
   unset NEWPW
   ```
   You want `ALTER ROLE` (with **no** NOTICE line) and `password login works`.
3. On the **PC**, [§4](#4-change-a-secret): set `POSTGRES_PASSWORD=<the same value>`. PR, merge, deploy.

Running services keep working between steps 2 and 3 (they use their own database roles); only
a deploy in that window would fail at `migrate`, so don't merge anything else in between.

### 8.2 The API and ingestd database passwords (`API_DB_PASSWORD`, `INGEST_DB_PASSWORD`)

Easy, because the `app-roles` step applies them on every deploy: new `openssl rand -hex 32`,
[§4](#4-change-a-secret), merge. Nothing to do inside Postgres.

### 8.3 Other secrets

| Secret | Do this |
|---|---|
| `ADMIN_SYNC_TOKEN` | [§4](#4-change-a-secret) (new `openssl rand -hex 32`) |
| `GITHUB_WEBHOOK_SECRET` | change it in **GitHub** (the App's webhook settings) *and* in [§4](#4-change-a-secret), same value |
| `GITHUB_LOGIN_CLIENT_SECRET` | GitHub → your Login OAuth App → generate a new secret → [§4](#4-change-a-secret) |
| `GRAFANA_ADMIN_PASSWORD` | only read when Grafana first starts; afterwards: on the VPS `dc exec grafana grafana cli admin reset-admin-password '<new>'` |
| `METRICS_DB_PASSWORD` | [§4](#4-change-a-secret); `metrics-role` re-applies it on deploy |
| GitHub App private key | replace `/opt/skillissues/secrets/github-app-private-key.pem` on the VPS, then redeploy |

### 8.4 SOPS recipients (the keys)

To **add or remove** a key from the encrypted file: edit `.sops.yaml` (public keys only), then on
the PC:
```bash
env SOPS_AGE_KEY_FILE=$HOME/.secrets/skillissues-sops.key \
  sops updatekeys -y --input-type dotenv secrets/prod.env.sops
```
Commit `.sops.yaml` and the file, PR, merge. **Rotating the VPS key (B):** make the new key on the
VPS (`sudo age-keygen -o /etc/skillissues/age.key.new`), add its public key to `.sops.yaml` *next
to* the old one, `updatekeys`, merge and deploy, then swap the key file into place
(`sudo mv` it over `age.key`, same `root:deploy` 640), then remove the old public key and
`updatekeys` again. If a key was ever exposed, also rotate the secrets themselves: old versions
stay readable in git history.

---

## 9. Emergencies

**The site/API is down.** `ssh skillissues`, `cd /opt/skillissues`, `dc ps`, then `dc logs go-api --tail 50`.
If `go-api`/`ingestd` are `Created` but not `Up` (after a failed deploy): fix the cause, then
re-run the failed job, or start them by hand: `IMAGE_TAG=sha-<tag> dc up -d` (use the tag of the
last good deploy).

**Disk full.** `df -h /opt/skillissues`; `docker system df`; `ls -lh /opt/skillissues/backups`.
First thing to free: old backups you've already pulled (`rm` the oldest `.age` files).
The crawler stops itself when the database passes `ingest.max_db_size_mb` (30000 by default).

**You lost key A** (your SOPS key): the VPS can still decrypt. On the VPS as the admin user:
`sudo env SOPS_AGE_KEY_FILE=/etc/skillissues/age.key sops decrypt --input-type dotenv --output-type dotenv /opt/skillissues/secrets/prod.env.sops`.
Make a new key A on the PC, put its public key in `.sops.yaml` replacing the old one, and run `sops updatekeys`
using the VPS key (copy the encrypted file to the VPS, run it there, copy it back).

**You lost key B** (the VPS key): make a new one on the VPS, put its public key in `.sops.yaml`,
`updatekeys` with key A on the PC, merge. Until then deploys fail at "decrypt" (nothing breaks).

**You lost key C** (the backup key): old backups are **gone for good**. Make a new key C, put the new public
key in `BACKUP_AGE_RECIPIENT` ([§4](#4-change-a-secret)), and take a fresh backup.

**You think a secret leaked.** Rotate it ([§8](#8-rotate-passwords-and-keys)). If it was a GitHub token, revoke it on GitHub first.

---

## 10. Routine: what to do and when

| When | Do | Where | Section |
|---|---|---|---|
| After every merge to `main` | check the Actions run is green | GitHub | [§3.1](#31-what-happens-when-you-merge-a-pr-to-main) |
| Every 2-3 days | pull backups | PC | [§5.1](#51-pull-the-backups-to-your-pc-every-few-days) |
| Weekly | `dc ps` + `health/detail`; glance at `df -h` | VPS | [§3.2](#32-is-production-healthy-right-now) |
| After pulling backups, weekly | decrypt-and-count check | PC | [§5.3](#53-prove-a-backup-works-do-this-often) |
| Every few months / before risky work | full restore into a scratch DB | PC | [§5.4](#54-restore-into-a-scratch-database-on-your-pc-every-few-months-or-before-risky-work) |
| When you find a junk repo | exclude it | VPS or PR | [§6.4](#64-exclude-a-repository-never-crawl-it) |
| Any secret change | `sops edit` → PR → merge | PC | [§4](#4-change-a-secret) |
| When you get a Cloudflare account | switch backups to R2 | PC + VPS | [§5.7](#57-switch-to-cloudflare-r2-later-when-you-have-an-account) |

---

## 11. Words you'll see

- **age**: a small encryption tool. A *public key* (`age1...`) encrypts; only the matching *private key* decrypts.
- **sops**: edits an encrypted file in place, encrypting to a list of age public keys.
- **migration**: a numbered SQL file that changes the database schema. Applied once, in order, by the `migrate` one-shot. Never edited after it has run anywhere; you add a new one.
- **one-shot**: a container that does a job and exits (`migrate`, `app-roles`, `metrics-role`). `Exited (0)` is success.
- **role**: a database login. `skillissues` (owner, can do anything), `skillissues_api`, `skillissues_ingest` (limited), `metrics_ro` (read-only, Grafana).
- **eligible repository**: a repo the crawler is allowed to crawl (stars, recent activity, not archived, not excluded).
- **oneshot** (systemd): a service that runs to completion; `systemctl start` waits for it.
