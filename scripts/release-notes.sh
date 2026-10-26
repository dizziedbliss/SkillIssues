#!/usr/bin/env bash
# Prints one version's release notes: the section under "## [<version>] - <date>"
# (Keep a Changelog; a bare "## <version>" works too) in desktop/release-notes.md,
# up to the next "## " or the link references at the end. Fails when there is none, so
# a release can't go out without notes (release.yml).
set -euo pipefail
version="${1:?usage: release-notes.sh <version, e.g. 1.2.0>}"
file="$(dirname "$0")/../desktop/release-notes.md"
notes="$(awk -v v="$version" '
  /^## / { if (found) exit; if ($2 == v || $2 == "[" v "]") { found = 1; next } }
  found && /^\[[^]]+\]: / { exit }
  found { print }
' "$file")"
if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "::error::desktop/release-notes.md has no '## [$version] - <date>' section" >&2
  exit 1
fi
printf '%s\n' "$notes"
