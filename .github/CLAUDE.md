# CI, deploy and releases (`.github/`)

`workflows/ci.yml` gates everything · `workflows/deploy.yml` ships `main` · `workflows/release.yml`
builds desktop releases from `vX.Y.Z` tags · `dependabot.yml` keeps actions and dependencies current.

## Checks

The compose and Caddy checks need placeholder environment values: copy the `Validate Docker Compose
configuration` and `Validate Caddyfile` steps from `ci.yml` (Caddy is validated with the image `compose.prod.yaml`
runs, so an unknown directive fails).

Parse a workflow change before pushing (`actionlint` if you have it); a YAML mistake costs a full
deploy cycle. Watch a run to the end: `env -u GITHUB_TOKEN gh run watch`.

## Rules

- **CI runs on pull requests and as the deploy gate**, not on plain pushes. A `changes` job
  (`dorny/paths-filter`) skips jobs whose paths didn't change, and **a skipped job counts as passing**:
  a new job needs its paths added to the filter. `workflow_dispatch` runs everything.
- CI builds the four deployed Dockerfile targets (`api`, `ingestd`, `flowd`, `migrate`; not
  `repoimport`), runs `docker compose config` on both compose files and `caddy validate`, and
  `govulncheck` on backend changes.
- **Deploy chain:** `ci` → `build-push` (GHCR `skillissues-{api,ingestd,flowd,migrate}` tagged
  `sha-<short>` and `latest`, provenance attested, cosign-signed by digest) → `deploy` (scp + ssh,
  `scripts/deploy-remote.sh`, which resolves the tag to a digest and runs `cosign verify` before
  deploying it) → `web` (Vercel CLI). Deploys queue (`concurrency: production-deploy`).
- **Deploys always pin a `sha-` tag, never `latest`.** Rollback is `workflow_dispatch` with an older tag;
  the site's rollback is Vercel's Instant Rollback.
- **Vercel deploys only through CI.** Its git integration is disconnected and must stay so. The `web` job
  fails if `vercel.json`'s `connect-src` doesn't list the API host.
- **Actions are pinned to commit SHAs** (with the version as a comment). Workflows default to
  `contents: read`. Never add `pull_request_target`, widen `permissions`, or swap a SHA for a tag.
- **Secrets:** CI holds no decryption key. `secrets/prod.env.sops` (SOPS + age) ships as ciphertext and
  is decrypted on the host; a file that won't decrypt fails the deploy. `check-secrets-encrypted.sh`
  fails on plaintext under `secrets/` or a committed age private key.
- **Releases:** the website and API share one version (`CHANGELOG.md`, tag `web-vX.Y.Z` on the last `main`
  commit of the release; no workflow runs on it). The desktop has its own (`desktop/release-notes.md`,
  tag `vX.Y.Z`, which `release.yml` builds and refuses without a notes section).

## Gotchas

- **cosign on the runner is v3.** Its default bundle format isn't read by a v2 verifier, so signing passes
  `--use-signing-config=false --new-bundle-format=false`. Two fixes in a row failed on legacy v2 flags:
  check `cosign <cmd> --help` for the installed version before changing a flag.
- Branches are kept on merge, so a stacked PR merges into its parent branch, not `main`. Retarget it to
  `main` before merging.
