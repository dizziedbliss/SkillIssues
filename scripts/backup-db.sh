#!/usr/bin/env bash
# Runs ON THE VPS via the systemd timer in systemd/skillissues-backup.timer.
#
# Dumps the production database, compresses it, and encrypts it with age to a
# public key whose private half is held OFF this host. The host can therefore
# write backups but cannot read them back: someone who takes the server gets
# neither a decryptable backup nor the key to make one.
#
# Where the encrypted dump goes is an explicit choice, BACKUP_DESTINATION:
#   r2     upload to Cloudflare R2 (needs the R2_* settings); retention is an R2
#          lifecycle rule. The off-site default for production.
#   local  keep it in BACKUP_DIR on this host and prune it after
#          BACKUP_RETENTION_DAYS (default 14). There is NO off-site copy: it
#          is safe from a thief reading it, not from this disk or VPS being
#          lost. Pull the files to another machine (DEPLOYMENT.md, "Backups").
# There is no default: an unset destination is an error, so nobody ends up
# with a backup policy they did not choose.
#
# A dump holds no GitHub credential (the server stores none) but does hold every
# user's id, login, avatar and saved issues: treat it as personal data.
#
# Nothing here falls back. If a setting or tool is missing, or the dump or upload
# fails, it exits non-zero (the systemd unit shows failed) rather than keeping an
# unencrypted copy or reporting success. An encrypted dump that could not be
# uploaded stays in BACKUP_DIR and is retried on the next run.
#
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -f .env ]; then
	echo ".env not found in $(pwd)" >&2
	exit 1
fi

# shellcheck disable=SC1091
set -a; source .env; set +a

destination="${BACKUP_DESTINATION:-}"
case "$destination" in
r2) required=(BACKUP_AGE_RECIPIENT R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY) ;;
local) required=(BACKUP_AGE_RECIPIENT) ;;
*)
	echo "BACKUP_DESTINATION must be 'r2' or 'local' (got '${destination}') — DEPLOYMENT.md, 'Backups'" >&2
	exit 1
	;;
esac

missing=()
for var in "${required[@]}"; do
	[ -n "${!var:-}" ] || missing+=("$var")
done
if [ ${#missing[@]} -gt 0 ]; then
	echo "backup is not configured: ${missing[*]} unset in .env (DEPLOYMENT.md, 'Backups')" >&2
	exit 1
fi
case "$BACKUP_AGE_RECIPIENT" in
age1*) ;;
*)
	echo "BACKUP_AGE_RECIPIENT must be an age PUBLIC key (age1...), never the private key" >&2
	exit 1
	;;
esac
tools=(age gzip)
[ "$destination" = r2 ] && tools+=(rclone)
for tool in "${tools[@]}"; do
	command -v "$tool" >/dev/null 2>&1 || { echo "$tool is not installed on this host" >&2; exit 1; }
done

BACKUP_DIR="${BACKUP_DIR:-$(pwd)/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
R2_PREFIX="${R2_PREFIX:-db}"
umask 077
mkdir -p "$BACKUP_DIR"

if [ "$destination" = r2 ]; then
	# rclone configured entirely from the environment: no config file holding the
	# R2 key to leave behind or to be world-readable.
	export RCLONE_CONFIG_R2_TYPE=s3
	export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
	export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
	export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
	export RCLONE_CONFIG_R2_ENDPOINT="$R2_ENDPOINT"
	# A token scoped to one bucket cannot create or list buckets.
	export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true
fi

cid="$(docker compose -f compose.prod.yaml --env-file .env ps -q postgres)"
if [ -z "$cid" ]; then
	echo "postgres container not running — nothing to back up" >&2
	exit 1
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
dest="$BACKUP_DIR/skillissues-${timestamp}.sql.gz.age"
partial="${dest}.partial"
trap 'rm -f "$partial"' EXIT

docker exec "$cid" pg_dump -U "${POSTGRES_USER:-skillissues}" "${POSTGRES_DB:-skillissues}" \
	| gzip \
	| age --recipient "$BACKUP_AGE_RECIPIENT" --output "$partial"

# An empty database still dumps a few KiB of schema; anything this small is a
# failed dump that pipefail did not catch.
size="$(stat -c %s "$partial")"
if [ "$size" -lt 1024 ]; then
	echo "dump is only $size bytes — refusing to treat it as a backup" >&2
	exit 1
fi
mv "$partial" "$dest"
trap - EXIT
echo "Wrote $dest ($(du -h "$dest" | cut -f1), encrypted)"

if [ "$destination" = local ]; then
	echo "LOCAL ONLY: no off-site copy exists. Pull $BACKUP_DIR to another machine (DEPLOYMENT.md, 'Backups')." >&2
	find "$BACKUP_DIR" -name 'skillissues-*.sql.gz.age' -mtime "+${RETENTION_DAYS}" -print -delete
	failed=0
else
	# Upload this dump and any earlier one whose upload failed. A file is removed
	# locally only after rclone reports it uploaded and verified.
	failed=0
	shopt -s nullglob
	for f in "$BACKUP_DIR"/skillissues-*.sql.gz.age; do
		name="$(basename "$f")"
		if rclone copyto "$f" "r2:${R2_BUCKET}/${R2_PREFIX}/${name}"; then
			rm -f "$f"
			echo "Uploaded $name to r2:${R2_BUCKET}/${R2_PREFIX}/"
		else
			echo "UPLOAD FAILED for $name; kept locally, will retry on the next run" >&2
			failed=1
		fi
	done
fi

# Plaintext dumps from before backups were encrypted are not touched, but the
# operator must hear about them.
if compgen -G "$BACKUP_DIR/skillissues-*.sql.gz" >/dev/null; then
	echo "WARNING: unencrypted dumps remain in $BACKUP_DIR (skillissues-*.sql.gz); delete them once an R2 backup restores cleanly" >&2
fi

exit "$failed"
