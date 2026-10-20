# SkillIssues Desktop

Electron 44 app (electron-vite, TypeScript strict). Find an open-source issue, then fork, fix and
open a pull request from one window:
1. Sign in with GitHub.
2. Pick an issue and press **Start work**. The app forks the repository to your account, clones your
   fork into your workspace folder and switches to a branch for the issue.
3. Open it in VS Code and make your change.
4. Commit and push, then open the pull request.

## Run it

Dependencies are installed once at the **repo root** (`web`, `desktop` and `packages/*` share one
lockfile):

```bash
npm ci                 # repo root
cd desktop
npm run dev            # hot reload
```

The app talks to `https://skillissues-api.dizziedbliss.lol`; set `SKILLISSUES_API_URL` to use another
API (for example `http://localhost:8000` with the local stack). `npm run build && npm start` runs the
built app without hot reload.

## Checks and release builds

```bash
npm run check          # tsc over main, preload, shared and renderer
npm test               # the security rules
npm run build
npm run dist:mac       # or dist:linux, dist:win, or dist for all three
```

Artifacts go to `desktop/dist/`. Signing and notarization come from the release environment's
standard electron-builder variables; never commit certificates. `.github/workflows/release.yml`
builds releases.

## How it is put together

- `src/shared/contract.ts` is the only interface between the page and the main process. The preload
  exposes one `window.skillissues` method per channel, and nothing else.
- `src/main/security.ts` holds the boundaries as pure functions, and `security.test.ts` pins them:
  - the window's flags (`contextIsolation`, `sandbox`, no Node)
  - the API calls the page may make
  - the links that may open in the browser
  - repository and branch names
  - keeping workspaces inside the workspace folder
  - how the push token reaches git
- Sign-in is a GitHub device flow that asks for no permissions. Its token is posted once to the API,
  which checks it and drops it; the app keeps only a SkillIssues session.
- Pushing needs a second device flow, for `public_repo`. That token never leaves your computer and
  must belong to the account you signed in with.
- Tokens are kept in the OS keychain (`safeStorage`). On Linux without a keyring they are kept in
  memory only, and you sign in again each launch; they are never written to disk in clear.
- git runs as async child processes, with argument lists and no shell. The push token reaches git
  through its environment, never the command line, and is removed from anything the app shows.
- Cloned code is never executed.

**One setting on GitHub:** desktop sign-in needs **Enable Device Flow** ticked on the Login OAuth
app (GitHub → Settings → Developer settings → OAuth Apps → the Login app).
