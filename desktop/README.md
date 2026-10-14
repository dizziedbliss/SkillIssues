# SkillIssues Desktop

Electron 44 app (electron-vite) with a React + TypeScript renderer built on the shared
`@skillissues/ui` and `@skillissues/api-client` packages.

It does what the web app does — browse, search, recommendations, saved issues, contributions —
plus the local workspace flow: inspect an issue, press **Start challenge**, and it forks the
repository with your own GitHub account, clones it into your workspace folder, points `origin`
at your fork and `upstream` at the original, lets you create a branch and push it, and opens
the folder in VS Code.

## Run it

Dependencies are installed once at the **repo root** (`web`, `desktop` and `packages/*` share
one lockfile):

```bash
npm ci                 # repo root
cd desktop
npm run dev            # hot reload; expects the API at http://localhost:8000
```

`npm run build && npm start` runs the built app without hot reload (`electron-vite preview`),
which is closer to a packaged release.

## Checks and release builds

```bash
npm run check          # tsc over main, preload and renderer
npm run build
npm run dist:mac       # or dist:linux, dist:win, or dist for all three
```

Artifacts go to `desktop/dist/`. Signing and notarization come from the release environment's
standard electron-builder variables; never commit certificates. `.github/workflows/release.yml`
builds releases.

## Security model

- The renderer runs with `contextIsolation`, `sandbox` and no Node integration, behind a
  Content Security Policy. It reaches the API only through a narrow preload bridge (`api:request`
  and a fixed set of workspace actions) — backend secrets never enter it.
- A single-instance lock; only allow-listed GitHub URLs open externally; the `skillissues://`
  protocol is registered for deep links from the web app.
- The workspace folder defaults to `~/Documents/SkillIssues` and can be changed in the app. Git
  commands refuse to run outside it, run as bounded async child processes, and are logged to
  `.skillissues/commands.log` inside each clone.
- Cloned code is never executed.
