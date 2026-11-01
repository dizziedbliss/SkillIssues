# Desktop (Electron 44, electron-vite, TypeScript strict)

`src/main/` the main process · `src/preload/` the bridge · `src/shared/contract.ts` the interface between
them · `src/renderer/src/` the web app's screens plus desktop-only ones.

## Workflow

```bash
npm ci                    # once, at the REPO ROOT (one workspace with web and packages/*)
cd desktop
npm run dev               # renderer HMR; expects the API at http://localhost:8000
npm run check             # tsc over main, preload, shared and renderer (fast)
npm test                  # the security rules (src/main/security.test.ts)
npm run build && npm start   # the built app
npm run dist              # release artifacts (CI builds these; you rarely need to)
```

Before a PR: `npm run check && npm run build && npm test`.

## Rules

- **`src/shared/contract.ts` is the only interface** between page and main process: every channel, its
  arguments and its `Result` (errors cross as data). The preload exposes exactly one `window.skillissues`
  method per channel; the main process won't start with a channel unhandled (`src/main/ipc.ts`); each
  handler checks the caller is the app's own top frame.
- **`src/main/security.ts` holds the boundaries** as pure functions pinned by `security.test.ts`:
  `webPreferences` (`contextIsolation`, `sandbox`, no Node), the API paths the page may reach through the
  proxy, the links that may open in the browser, repository and branch names, workspace containment,
  how the push token reaches git. A new boundary rule goes here with a test. Loosen one only
  deliberately, and say why in the PR.
- **Sign-in** is a no-scope device flow on the Login app, posted once to `POST /auth/github/device`; the
  app keeps only the session. **Pushing** takes a second device flow for `public_repo`, whose token never
  leaves the machine and must belong to the signed-in account.
- **Tokens** are kept with `safeStorage`. Where the OS has no real encryption (Linux `basic_text`) they
  stay in memory only; **never written in clear**.
- **Git runs as async child processes with argument lists, never a shell.** The push token reaches git
  only through `GIT_CONFIG_*` environment entries (`http.extraheader`), never argv, and is redacted from
  anything shown. Every workspace path is re-checked to be inside the workspace folder after resolving
  links.
- **The renderer is the web app's screens:** it imports `web/src/app` as `@web` under a `HashRouter`,
  after `setApi` (the IPC proxy) and `setPlatform`. **Anything that differs from the web goes through the
  platform, never a fork of a screen.** Desktop-only screens: Work on it here (`WorkOnIt.tsx`), Working on
  (`WorkspacesPage.tsx`), Settings → Desktop app.
- The renderer's CSP reaches no network; images only from GitHub. GitHub reads go through the main
  process (`githubGet`, `isAllowedGitHubRead`), without credentials. The page's clipboard is off with
  every other permission; a sign-in code is copied through the main process (`copyText`).
- `skillissues://issue/<id>` (the site's "Work on it in the desktop app") is a contract with the web: the
  main process keeps the link and the page takes it (`takeLink`) on start and focus.
- **Only `electron-updater` is a runtime dependency.** The renderer is bundled, so everything else is a
  dev dependency and stays out of the installer.
- **Updates ask first:** `autoDownload` is off; the person chooses to download. macOS isn't signed, so
  there "download" opens the release page.

## Releases

- The desktop has its own version (SemVer) and changelog, `release-notes.md` (Keep a Changelog:
  `## [Unreleased]`, then `## [x.y.z] - date`, compare links at the bottom). A change people will notice
  adds a line under `[Unreleased]` in the same PR.
- A `vX.Y.Z` tag builds the release (`.github/workflows/release.yml`), which refuses a tag without its
  section (`scripts/release-notes.sh`). The website's `content/docs/changelog.md` summarises it for people.
- The NSIS installer (`build.nsis`, `build/installer.nsh`, `build/*.bmp` as 24-bit BMP) is dark like the
  app, with the wordmark and no mark. Builds are unsigned until `WIN_CSC_LINK`/`WIN_CSC_KEY_PASSWORD`
  exist. The setup most people download is `installer/` (its own guide).
