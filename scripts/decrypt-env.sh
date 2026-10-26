#!/usr/bin/env bash
# Runs ON THE VPS (called by deploy-remote.sh). Decrypts a SOPS dotenv file into
# a mode-600 .env, atomically: the old .env is replaced only once decryption
# succeeded, and a failure leaves it untouched.
#
# Usage: decrypt-env.sh <secrets-file> <output-file>
#   SOPS_AGE_KEY_FILE  the VPS's age private key (default /etc/skillissues/age.key)
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: $0 <secrets-file> <output-file>" >&2
	exit 1
fi
secrets="$1"
out="$2"
key_file="${SOPS_AGE_KEY_FILE:-/etc/skillissues/age.key}"

if ! command -v sops >/dev/null 2>&1; then
	echo "sops is not installed on this host" >&2
	exit 1
fi
if [ ! -r "$key_file" ]; then
	echo "age key $key_file is missing or unreadable by $(id -un)" >&2
	exit 1
fi

umask 077
tmp="$(mktemp "$(dirname "$out")/.env.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

SOPS_AGE_KEY_FILE="$key_file" sops --decrypt --input-type dotenv --output-type dotenv "$secrets" > "$tmp"

if [ ! -s "$tmp" ]; then
	echo "decrypting $secrets produced an empty file" >&2
	exit 1
fi
chmod 600 "$tmp"
mv -f "$tmp" "$out"
trap - EXIT
echo "Decrypted $secrets -> $out"
