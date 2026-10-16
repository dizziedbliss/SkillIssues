#!/usr/bin/env bash
# CI guard: nothing under secrets/ may be committed in plaintext, and no age
# private key may be committed anywhere. A SOPS dotenv file always carries
# sops_mac= metadata; a plaintext file renamed *.sops does not.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

status=0

while IFS= read -r f; do
	case "$f" in
	secrets/README*) continue ;;
	esac
	if ! grep -q '^sops_mac=' "$f"; then
		echo "secrets/ file is not SOPS-encrypted: $f" >&2
		status=1
	fi
done < <(git ls-files secrets)

if git grep -lI 'AGE-SECRET-KEY-' -- . ':!scripts/check-secrets-encrypted.sh' ':!DEPLOYMENT.md' ':!CLAUDE.md' >/tmp/age-hits.$$ 2>/dev/null; then
	echo "an age private key is committed:" >&2
	cat /tmp/age-hits.$$ >&2
	status=1
fi
rm -f /tmp/age-hits.$$

exit "$status"
