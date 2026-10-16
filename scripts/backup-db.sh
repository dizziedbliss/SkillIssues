#!/usr/bin/env bash
# Runs ON THE VPS via the systemd timer in systemd/skillissues-backup.timer.
# Dumps the production database to /opt/skillissues/backups and prunes
# dumps older than the retention window.
#
# A dump holds no GitHub credential (the server stores none) but does hold every
# user's id, login, avatar and saved issues: treat it as personal data.
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -f .env ]; then
	echo ".env not found in $(pwd)" >&2
	exit 1
fi

# shellcheck disable=SC1091
set -a; source .env; set +a

BACKUP_DIR="${BACKUP_DIR:-$(pwd)/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
mkdir -p "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
dest="$BACKUP_DIR/skillissues-${timestamp}.sql.gz"

cid="$(docker compose -f compose.prod.yaml --env-file .env ps -q postgres)"
if [ -z "$cid" ]; then
	echo "postgres container not running — nothing to back up" >&2
	exit 1
fi

docker exec "$cid" pg_dump -U "${POSTGRES_USER:-skillissues}" "${POSTGRES_DB:-skillissues}" \
	| gzip > "$dest"

echo "Wrote $dest ($(du -h "$dest" | cut -f1))"

find "$BACKUP_DIR" -name 'skillissues-*.sql.gz' -mtime "+${RETENTION_DAYS}" -print -delete
