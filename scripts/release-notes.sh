#!/usr/bin/env bash
# Prints one version's release notes: the section under "## <version>" in
# desktop/release-notes.md, up to the next "## ". Fails when there is none, so
# a release can't go out without notes (release.yml).
set -euo pipefail
version="${1:?usage: release-notes.sh <version, e.g. 1.2.0>}"
file="$(dirname "$0")/../desktop/release-notes.md"
notes="$(awk -v v="$version" '
  /^## / { if (found) exit; if ($2 == v) { found = 1; next } }
  found { print }
' "$file")"
if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "::error::desktop/release-notes.md has no '## $version' section" >&2
  exit 1
fi
printf '%s\n' "$notes"
