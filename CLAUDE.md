# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

**This is production code.** It runs against the real GitHub API with a real registered GitHub
App, crawls real repositories, and holds real people's GitHub identities. It is
not a prototype or a demo, and "it works on my machine" is not a finished state. Concretely,
that means:

- **Nothing swallows an error.** A discarded `err`, a `catch(() => [])`, an ignored `UPDATE`
  result, or a handler that returns success after a failed write is a bug, not a style
  preference. A failure the user cannot see is worse than a crash.
- **No operational value is hardcoded.** See *Configuration* below; thresholds, intervals,
  page sizes and worker counts are live settings, and a literal in the code is a defect.
- **Migrations are append-only.** Never edit one that has been applied anywhere.
- **Failures are observable.** `/metrics` and `/health/detail` must reflect reality — a
  component that is down must not be logged as connected.
- **Credentials are never logged**, not even a prefix or a length.

**This file describes what is true now.** Do not document planned features here — that is
how a previous version came to describe a Chi router, a `kaggle_etl` binary and a migration
policy that did not exist. Where a designed component is deliberately not built yet, this file
says so explicitly. Work in flight lives in the plan files under `~/.claude/plans/`.

---

## Commands

### Full stack (Docker — recommended)
```bash
cp .env.example .env          # first time only
docker compose up -d --build
docker compose ps
docker compose logs -f go-api ingestd
```

The `migrate` service applies the schema and exits; `go-api` and `ingestd` wait for it to
complete successfully. A fresh `up` needs **no manual SQL**.

### Production (VPS, `compose.prod.yaml`)
Pulls pre-built GHCR images; never builds on the host. Deployed by CI (`.github/workflows/deploy.yml`)
on every push to `main` — see *Deployment* below and `DEPLOYMENT.md` for the full runbook,
including first-time host setup, rollback and restore.

### Hybrid dev (databases in Docker, services native)
```bash
docker compose up -d postgres redis

export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"

cd backend
go run ./cmd/migrate          # apply schema first

export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="ghp_your_token_here"
export PORT=8000
go run ./cmd/api

npm ci            # root install — see *Web (React + TypeScript)* below
cd web && npm run dev
```

### Repository metadata import
```bash
cd backend
# Streams the Kaggle dataset (data/repo_metadata.json, ~3GB) and stores every
# record. Resumable and idempotent — safe to re-run, and a re-run is how a
# refreshed dataset lands.
DATABASE_URL=... go run ./cmd/repoimport --file ../data/repo_metadata.json
DATABASE_URL=... go run ./cmd/repoimport --batch-size 1000 --log-every 100000
```

### Backend (Go)
```bash
cd backend
go test ./...                 # all tests
go test ./internal/issue/...  # single package
go test -short ./...          # skips integration tests needing a live API
go fmt ./... && go vet ./...
```

### Web (React + TypeScript)
`web`, `desktop` and `packages/*` are one npm workspace (root `package.json`) — `npm ci`
(never `npm install` — dependencies are pinned) runs **once, at the repo root**, not inside
`web/` or `desktop/`. There is one lockfile for the whole tree, not one per package.
```bash
npm ci            # repo root — not `npm install`, and not inside web/
cd web
npm run dev       # Vite dev server
npm run build     # tsc -b && vite build
npm run typecheck # tsc --noEmit
```

### Desktop (Electron)
```bash
# npm ci already ran at the repo root above.
cd desktop
npm run dev       # electron-vite dev server, renderer HMR
npm run build && npm start   # build then run the built app (no HMR)
npm run check     # tsc --noEmit over main, preload, shared and renderer
npm test          # the security rules (src/main/security.test.ts)
npm run dist      # build, then electron-builder release artifacts
```

---

## Architecture

### Backend (`backend/`)
Entrypoints under `backend/cmd/`:

- **`api`** — HTTP API on `:8000`. Uses the **standard library `http.NewServeMux`** with Go
  1.22 method patterns (`"GET /issues"`) and `{id}` path params. There is no third-party
  router; do not add one without a reason routing actually causes.
- **`ingestd`** — GitHub GraphQL crawler on `:8002`. Owns the ingestion coordinator, its
  worker pools and the claim/cycle loop. See *Ingestion policy*.
- **`flowd`** — relay for the public live visualizer on `:8003`. See *Flow tap*.
- **`repoimport`** — offline importer for the repository metadata dataset. Streams the JSON
  array and stores only records that pass `eligible()`. Touches only `repositories` and
  `import_skipped`.
- **`migrate`** — one-shot; applies the embedded schema and exits.
- **`healthcheck`** — tiny binary used as the Docker `HEALTHCHECK` command.

Middleware chain, outermost first: RequestID → Logger → FlowTap → MaxBody → CORS → CSRF →
OptionalAuth → RateLimiter.
`/admin/` and `/webhooks/` are mounted outside that chain (they authenticate themselves and
have no browser callers).

Use `pgxpool` for all database access — never open individual connections. Always thread
`ctx context.Context` through to database, Redis and HTTP calls.

### Metrics (`backend/internal/metrics/`)
`prometheus/client_golang`, served at `GET /metrics` by **both** `api` (:8000) and `ingestd`
(:8002), and by `flowd` (:8003). Each binary registers only the families it produces
(`RegisterAPI`, `RegisterIngest`, `RegisterGitHub`, `RegisterDBPool`, `RegisterFlowPublisher`,
`RegisterFlowd`), so a process never exports another's gauge as a misleading zero.

- HTTP metrics are labelled by the **matched `ServeMux` pattern** (`route="GET /issues/{id}"`),
  never the raw path — `middleware.RecordRoute` must wrap each mux directly, because the mux
  sets `r.Pattern` on a request the outer middleware only sees as a clone. Unmatched paths are
  `route="unmatched"`.
- Every outbound GitHub call goes through `metrics.InstrumentedTransport` on the shared
  `github.Client`; tag new client methods with `metrics.WithOperation(ctx, "Name")`.
- ingestd's Postgres-backed gauges (`ingest_jobs`, `repositories`,
  `github_rate_limit_remaining{lane}`) refresh every `ingest.metrics_refresh_seconds`, not per
  claim cycle and not per scrape.
- A failure you log on an operational path should also increment a counter
  (`ingest_cycle_errors_total`, `github_budget_errors_total`, …) so it shows on a dashboard.

### Flow tap (`backend/internal/flow/`, `internal/flowd/`, `cmd/flowd`)
The public live visualizer (a separate site, `flow.dizziedbliss.lol`) shows data moving
through the system. go-api and ingestd describe each hop as a `flow.Event` and call
`flow.Emit`; a `Publisher` batches them to Redis (`PUBLISH flow:events`) and flowd relays them
to viewers at `GET /stream` (SSE; `?format=ndjson` for `curl -N`). The taps reuse existing
seams: `middleware.FlowTap` (route pattern, status, size), `flow.QueryTracer` on every pool
(`db.New`; verb and first table only, never SQL or arguments), `flow.Transport` on the GitHub
client and the outbound client (operation name and status), the Redis cache wrapper, and one
emit after each committed crawl or discovery page (repository, counts, first title).

- **Redaction lives only in `flow.Emit`.** A context marked by `flow.MarkUser` (a request
  presenting a session, valid or not, and every `/auth/` request) or `flow.UserLinked` (the
  tracked-issue check) is published as random symbols of random length. Never build a label
  from anything a caller could not show a stranger, and never add an identifying field to
  `flow.Event` — `TestEventHasNoIdentifyingFields` pins the field set.
- **Fire-and-forget.** `Emit` never blocks or fails its caller; a full buffer, a Redis error or
  no listening flowd drops events and counts them (`flow_events_dropped_total{reason}`). Redis
  being down changes nothing else, and with no publisher installed (`migrate`, `repoimport`,
  flowd itself) `Emit` is a no-op.
- flowd relays at most `flow.max_events_per_second` labelled events; every event still counts
  towards its link's density line. It reads its settings as `skillissues_flow` and re-encodes
  every event through `flow.Event`, so an unknown field never reaches a viewer. Links nothing is
  instrumented on get ambient rates from cAdvisor via Prometheus (`PROMETHEUS_URL`).
- `flow.enabled` (live) turns the tap off. The visualizer site is not in this repository.

### Shared packages (`packages/`)
npm workspace members, imported as `@skillissues/*` (a real `node_modules` symlink, not a relative
path reaching outside the importing project). Both apps are built on `client`; the web app also on
`theme`.
- **`packages/client`** — the typed API client: response types, `ApiError` (`status`, `problems`;
  status 0 = never answered), abort support, a pluggable `Transport` (`fetchTransport` for the
  browser). Cursor paging always sends `?cursor=`, even empty: it is what selects the shuffled feed.
- **`packages/theme`** — user themes on the client: the TOML schema and stylesheet allowlist
  mirroring `backend/internal/theme`, run on the same fixtures (`packages/theme/fixtures`, CI runs
  both sides), the presets, and `compileTheme`/`applyTheme`, which re-validates a theme and applies
  it through constructed stylesheets scoped to the app's root. `npm test` runs the checks.
- **`packages/markdown`** — sanitised Markdown rendering (`markdown-it` + DOMPurify). Issue bodies
  are attacker-controlled; never hand-roll rendering in either client. The only `class` that survives
  is `language-…` on `<code>` (a DOMPurify hook); `highlightCode` colours those blocks afterwards with
  highlight.js (its own lazily loaded chunk, a fixed set of languages), reading their text, never HTML.

### Web (`web/src/`)
- **`landing/`** — the landing page, the maintainer's design. Its look is **locked**
  (`landing/tokens.module.css`, light and a dark version) and outside the theme system: no user theme reaches it. Centred,
  over the backlog field (`BacklogField.tsx`, canvas: a faint dot grid whose cells light red or green in today's
  arrived/closed ratio from `GET /stats`, and lift near the pointer; still under reduced motion). The hero (`Hero.tsx`)
  is one column: "Someone opened this N days ago. Nobody has fixed it yet." over a real issue, chosen
  from today's For you picks (unassigned, one per repository, no account) by `waitingOrder`: never a repository flagged
  slow, never older than a year, welcoming repositories first, then the longest waiting; language chips, a level and
  "Another one" swap it, under the promise as the h1 ("Not this one? Find an issue you can solve, in ten seconds."),
  and [Find an issue] opens Explore with those choices; then four live numbers (open, arrived and closed today, closed
  since counting began). Until the issue arrives (and in the prerendered page) the live open count stands in: never an
  invented issue. Then full-width bands with no boxes: feature rows with
  live miniatures (`Widgets.tsx`: levels, the desktop's steps, what sign-in can't do, latest activity), the repository
  finder (`RepoFinder.tsx`, styled with `Finder.module.css`): a name searches the Repository category, owner/name or a
  GitHub link asks `GET /repositories/lookup` and says why one isn't listed, and a dead end offers the
  `repository_coverage.yml` issue form, prefilled. After it, a comparison by kind of tool (issue lists, repo finders;
  never named products, cells hedged), why it's free (sponsor maintainers instead), developers and maintainers, and a
  closing that says why we made it. Body text is never under 16px. Motion shares one easing (`--ease`, `--fast`, `--slow`): buttons fade to their
  hover colour, links draw their underline, live counts glide between polls; all of it stops under reduced
  motion. Colours are the brand tokens on `:root` in `styles/base.css`, the one place they are written
  (warm paper and ink, not pure black and white; one saturated red per mode at the brightness that
  passes AA as text and under paper-coloured text): `tokens.module.css` reads them, and the copies CSS
  can't reach (the Default and Midnight themes, `favicon.svg`, `theme-color`) change with them. Fonts are
  self-hosted through `@fontsource` (Instrument Serif headings and nav, JetBrains Mono everything
  else but long reading, Inter for the docs' and legal pages' prose (`--sans`), Krona One for the wordmark: SkillIssues in the ink, the "i" of Skill red, one ink underline across,
  `Logo.tsx`). The project's mark (a ring, a rough dashed ring, a red "!") is the favicon and the app
  icons only (the desktop's window and installer icon included), never beside the wordmark, in the web app,
  the desktop app or the installer (`public/favicon.svg`, its ink and red lighter in a dark browser theme;
  PNGs rendered from it). In the site and app headers the logo swaps with the live count every few
  seconds (`LogoLive.tsx`: closed against open, `GET /stats`; never under reduced motion).
- **`pages/`** — the written pages; every claim must describe what the code does. The docs are Markdown in
  `web/content/docs/` (frontmatter `slug`, `title`, `section`, `summary`), read and rendered at build
  time by `docs.ts` (markdown-it, raw HTML off: first-party text, so not through `packages/markdown`,
  whose DOMPurify needs a DOM) and shown by `DocPage.tsx` with a sidebar by section, a search
  over every page (`DocSearch.tsx`, `/` or Ctrl/⌘K), an "On this page" index of the page's h2s
  (collected by `docs.ts`; sticky on the right, folded above the text under 1200px, marking the section
  being read) and the previous and next pages in the sidebar's order. The four legal pages
  (privacy, terms, security, licenses) stay in `content.ts` by the maintainer's choice. Pages marked
  `draft` say so on the page. The footer links only to these pages and `/app`.
- **`app/`** — the signed-in app, being built from the maintainer's designs in stages (plan:
  `~/.claude/plans/quirky-splashing-key.md`). `AppShell.tsx` is its themed root (`data-app`): the
  header (`AppHeader.tsx`: the places you work, Explore, Learning path, the desktop's Working on and
  Search with its `/` key; then Profile with the avatar, whose menu holds what's about you: Profile,
  Saved issues, Your contributions, Reports for moderators; then Settings, Appearance, Language; then
  Sign out — "Sign in" while signed out. Nothing is in both), the page for the route, and the shared footer. `theme.tsx` applies the
  chosen theme to that root only: a built-in by name or a stored theme, kept by the server signed in
  (`GET/PUT /me/theme`) and by the browser signed out; nothing chosen is Midnight. Every theme has light and dark
  halves (`[light]`/`[dark]`, folded per layer in `packages/theme/src/modes.ts`; every built-in has
  both); the mode is the person's, kept by the browser, dark by default, and the footer's theme
  button switches it. The browser keeps the last few choices: Appearance in the
  menu lists them to switch between (no switch), with "See more themes". `explore/` is the catalog: one search bar
  (`explore/SearchBar.tsx`, shared with the search page and the palette), then one toolbar (the levels
  as a segmented control, then the profile's languages, a repository or an owner as chips, each with a ×).
  Its state lives in the address (`query.ts`: absent `difficulty` means the profile's level, empty
  means all), the filter panel (`FilterPanel.tsx`: a row of pills, each saying what it means at the top
  of its dropdown) takes `/issues/facets`, and the pager (`useCatalog.ts`) pages forward on cursors
  and back over the pages seen, reading on through short pages. `IssuePage.tsx` (`/app/issues/:id`)
  reads the body from GitHub in the browser and asks `GET /issues/{id}/body` only when GitHub
  rate-limits it (`lib/issueBody.ts`), rendered through `packages/markdown`. Bookmarks
  (`SaveButton.tsx`, `saved.ts`) save optimistically and say so when the API refuses. A card opens its issue anywhere
  you click it; its repository's name opens Explore narrowed to that repository (`?repo=`), and the
  issue page also offers its owner (`?owner=`), both easiest first: the shuffled feed would show one
  issue per repository. The issue page says at once when GitHub reports the issue closed, from the
  same answer that brought its description, and shows six labels before "+n more". "Work on it in the desktop app" opens
  `skillissues://issue/<id>` and, if the page keeps focus, offers the download or GitHub.
  Search (`search/SearchOverlay.tsx`: the nav, `/` or Ctrl/⌘K) is a palette over the blurred page: the
  shared bar on top, results as you type grouped by kind, each with View more; Esc or a click outside
  closes it. It leads to
  the search page (`search/SearchPage.tsx`, `/app/search?search=…&in=repository|title|language`),
  never to Explore: every kind, or one kind paged on its own, with the same filters. Its box shares
  a `view-transition-name` with the search page's (and Explore's), so one glides into the other.
  The wordmark inside the app goes to Explore, afresh; the Explore nav item and the issue page's back
  link return to Explore as it was left, filters and page (`explore/where.ts`: the address per tab, the
  pager's place per filter set in memory).
  The issue page has a beginner checklist (`BeginnerCheck.tsx`: labelled, described, unassigned, no
  open linked pull request via the timeline, recent, short discussion; thresholds documented in
  `/docs/good-first-issues`, keep both in step) and report links (`ReportButton.tsx`). Settings →
  Themes has a Community tab over `GET /themes/public`. Anything that can't simply be undone (giving up
  a path, releasing a claim, deleting a theme or a workspace) asks first through `useConfirm`
  (`Confirm.tsx`). Search also finds people: a term shaped like a GitHub username asks for that public
  profile (`search/PersonResult.tsx`). The download page (`/download`) adds the newest release for the
  visitor's system, the others and older versions, read from GitHub's releases API in the browser
  (`pages/Downloads.tsx`); its prerendered HTML is the releases link alone. Page changes inside the app run the theme's `[motion] transition` through View
  Transitions (`transition.ts`, `transition.css`; never under reduced motion). The profile
  (`profile/`: `/app/profile`, and `/app/contributions`, Your contributions in the menu) counts the person's public
  pull requests from GitHub's search **in the browser**, with no token (`lib/github.ts`, cached per
  browser, GitHub's rate limit shown as such), so the server still holds no user token. Beside it,
  SkillIssues XP comes from the server (`profile/xp.tsx`: badges, per-language levels, quests,
  tracked pull requests with "Check now", latest XP, and a one-time reveal after the backfill,
  remembered per browser). The profile page is a header band (`profile/Header.tsx`, shared with public
  profiles: the level large with its bar, merges, languages and streak), then Next up (the closest
  quests, the learning path, claims, open pull requests in review) and one list of pull requests
  (`profile/activity.tsx`: the tracker's and GitHub's merged by URL, the tracker's XP shown) beside a
  rail of languages and earned badges (the rest behind "N more to earn"). Hairlines, no boxes; the
  graph, the stat boxes and every quest are on `/app/contributions`. `/app/saved` lists saved issues under their notifications (closed issues
  and paid merges), and
  Profile shows a red dot while any are unread. `lib/devPreview.ts` (development builds only,
  `?preview=signed-in`) answers the account endpoints locally, since GitHub sign-in returns to
  production. `settings/` is Settings: the profile form (`PUT /profile`, refused whole with the
  reason past the `api.profile.*` limits; each setting a row, its name and help on the left, the control
  on the right; levels as segmented controls; languages show your picks and the most common, then
  "Show all"), one view per address beside a menu (`/app/settings/<view>`):
  themes in tabs (each theme drawn as a miniature of its own colours; built-ins are read-only: Use applies one as it is, Customize makes an editable
  copy; yours: use, edit, duplicate, delete, new as a full copy; shared by id: use or duplicate) and
  an editor page (`themes/:id`) that checks as you type, with a live preview of the app's own
  components in a separate stylesheet slot and a token map; the desktop app; the account (sign out
  everywhere, delete; the Privacy policy points there); support. Components carry stable `data-ui`
  hooks for theme stylesheets (documented in the themes guide); don't rename one casually. Sign-in returns to `/?auth=ok` or `/?error=<code>`, which `App.tsx`
  forwards to `/app`.
- **`i18n/`** — every interface string of the landing page, footer, header and app is a key in
  `en.ts`; a translation is a file of the same shape registered in `LOCALES`, and a missing key shows
  in English. The docs and legal pages stay English. The first render is English (what is
  prerendered); a stored choice (`si.locale`) applies after hydration. The footer and Profile's menu
  hold the picker.
- **Light and dark; dark by default.** The landing, docs and written pages have a locked dark
  version (`tokens.module.css`, `html[data-mode="dark"]`, set in `index.html`; `public/mode.js` takes
  it off before the first paint when the browser chose light; `lib/siteMode.ts`), and the app starts
  on Midnight; the footer's theme button switches the site outside the app, and the theme's
  light and dark halves inside it (`lib/themeToggle.ts`). User themes still never reach those pages.
- **Prerendered**: `npm run build` renders `/`, the legal pages and every docs page to HTML
  (`scripts/prerender.mjs`, `src/entry-server.tsx`); the client hydrates a page only when its
  `data-route` matches. Vercel serves `shell.html` (no markup) for every other route.
  Each prerendered page gets its head from `src/seo.ts`: description, canonical address, Open Graph and
  Twitter tags with `public/og.png`, and JSON-LD (WebSite with search and Organization on `/`,
  SoftwareApplication on `/download`, TechArticle on docs). `shell.html` gets the defaults with no
  canonical. The build also writes `sitemap.xml` and `robots.txt` (private app pages disallowed);
  `public/manifest.webmanifest` makes it installable. In the app, `useDocumentTitle` keeps the tab's
  title right as it navigates. Every claim in `seo.ts` must be true of the site, like the docs.
- CSS Modules over custom properties; no Tailwind, no CSS-in-JS. Live counts poll `GET /stats`
  every 10s (React Query, paused in background tabs).
- The CSP (`vercel.json`) allows fetching nothing from elsewhere but the API host and GitHub's API,
  and images only from GitHub (`github.com`, `*.githubusercontent.com`, and the S3 bucket
  `github.com/user-attachments` redirects to: avatars and the images in issue descriptions; the
  desktop's CSP lists the same). A theme stylesheet is someone else's code; widen it only deliberately.

Runs **React 19**.

### Desktop (`desktop/`)
`electron-vite`, TypeScript strict throughout. **`src/shared/contract.ts` is the only interface between
the page and the main process**: every channel, its arguments and its `Result` (errors cross as data).
The preload exposes exactly one `window.skillissues` method per channel, and the main process will not
start with a channel unhandled (`src/main/ipc.ts`); each handler checks the caller is the app's own
top frame.
- **`src/main/security.ts`** holds the boundaries as pure functions, pinned by `security.test.ts`
  (`npm test`, CI): the window's `webPreferences` (`contextIsolation`, `sandbox`, no Node), the API
  paths the page may reach through the main process's proxy (`api.ts`), the links that may open in
  the browser (GitHub and the site), repository and branch names, workspace containment, and how the
  push token reaches git. Loosen one only deliberately.
- **Sign-in** (`auth.ts`): a no-scope GitHub device flow on the Login app (client id from `GET
  /auth/github/device`), posted once to `POST /auth/github/device`; the app keeps only the session.
  **Pushing** takes a second device flow for `public_repo`, whose token never leaves the machine and
  must belong to the signed-in account.
- **Tokens** (`store.ts`) are kept with `safeStorage`. Where the OS has no real encryption (Linux's
  `basic_text` backend) they stay in memory only and the person signs in each launch; they are never
  written in clear.
- **Git** (`git.ts`, `workspace.ts`) runs as async child processes with argument lists, never a shell.
  The push token reaches git through `GIT_CONFIG_*` environment entries (an `http.extraheader` for
  github.com), never argv, and is redacted from anything shown. A workspace is the person's fork,
  cloned into the workspace folder, on `skillissues/<number>-<slug>`, with the original as `upstream`;
  every path is re-checked to be inside that folder after resolving links. VS Code opens through its
  `vscode://` link.
- **The renderer is the web app's screens** (`src/renderer/src`): it imports `web/src/app` as `@web`
  and renders `AppShell` under a `HashRouter`, after swapping the API client (`setApi`, over the IPC
  proxy) and the platform (`web/src/app/platform.ts`, `setPlatform`): sign-in is a device-flow dialog,
  GitHub reads go through the main process (`githubGet`, `isAllowedGitHubRead`: an issue, its
  timeline and pull request search, no credentials), site pages open in the browser (`SiteLink`), and
  "Work on it here" (`WorkOnIt.tsx`: fork, then commit with a title and optional description, push
  only commits, and a pull request the person writes, "Closes #n" an unchecked option), Working on
  (`WorkspacesPage.tsx`, `/app/work`, a nav item only the desktop's platform adds) and Settings →
  Desktop app are its own. The page's clipboard is off with every other permission; a sign-in code is
  copied through the main process (`copyText`). Only `electron-updater` is a runtime dependency: the
  renderer is bundled, so everything else is a dev dependency and stays out of the installer.
  Anything that differs between web and desktop goes through the platform, never a fork of a screen.
  Its CSP lets it reach no network; images may come from GitHub. `skillissues://issue/<id>` (the
  site's button) opens that issue: the main process keeps the link and the page takes it (`takeLink`)
  on start and focus.

- **Updates ask first** (`main/updater.ts`, `Updates.tsx`): electron-updater checks GitHub releases on
  start and every 6 h but never downloads on its own (`autoDownload` off): the title bar says **Update
  available**, the dialog shows that version's notes (`releaseNotes`, its GitHub release read without
  credentials) and downloads only when asked; then restart, or it installs on quit. The first start
  after an update shows What's new once (`whatsNew`, from `lastVersion` in state.json). macOS isn't
  signed, so there "download" opens the release page.
- **Release notes are in the repository**: `desktop/release-notes.md`, one `## x.y.z` section per
  version. `release.yml` refuses a tag without one (`scripts/release-notes.sh`) and puts it on the
  GitHub release.
- **The Windows installer is assisted** (`build.nsis`): a welcome page (`build/installer.nsh`), the
  license and terms (`build/license.txt`), per-user or per-machine, and the folder. It's dark, like the
  app (`MUI_BGCOLOR`/`MUI_TEXTCOLOR` in `installer.nsh`), with the wordmark and tagline on the
  welcome and finish pages' sidebar and the wordmark alone in a header strip (`build/*.bmp`, 24-bit as NSIS needs them;
  no mark in them: the mark is the app icon and favicon only, like everywhere else). CI builds it on
  every desktop change (`desktop-installer`). Builds are unsigned, which is why Windows SmartScreen
  warns; `release.yml` signs once `WIN_CSC_LINK`/`WIN_CSC_KEY_PASSWORD` secrets exist.

Runs **Electron 44**. Release builds: `.github/workflows/release.yml`.

### Database (`backend/internal/migrate/migrations/`)
Goose migrations, embedded into the binaries with `go:embed`. **This directory is the single
source of truth for the schema** — there is no separate `schema.sql`.

### Docker images
One root **`Dockerfile`** with named targets (`api`, `ingestd`, `flowd`, `migrate`, `repoimport`),
built from the **repo root**. The site has no image: it deploys to Vercel. Root
**`.dockerignore`** keeps the build context small (excludes `data/`, `.git`,
`node_modules`, secrets). `compose.prod.yaml` (prod) only ever pulls pre-built images from
GHCR and never builds anything on the host; the deploy pipeline builds against these targets.
**`web/Dockerfile`** is the one exception — a dev-only Vite HMR image used by `compose.yaml`'s
`frontend` service, built from the repo root because it needs the npm workspace (`packages/*`).
Don't add a third one back.

---

## Ingestion policy

Design of record: `docs/ingestion-architecture.md`. Read it before changing
anything in `backend/internal/ingest/`.

**Collect broadly first; analyse later.** Coverage comes before prioritisation. Scope is open
issues, most-recently-updated first — not every comment, reaction, event or closed issue.

### The pipeline

```
Kaggle repository-metadata dataset      Discovery Sweeper (ingestd, off by default)
  (cmd/repoimport)                        GitHub repository Search over pushed-date
  ~4.2M records streamed,                 windows; full sweep monthly, incremental daily
  filtered at import                             │
        │  only passers stored                   │  only passers stored
        ▼                                        ▼
eligible repositories  (~278k after the first full crawl)  ◀── archive candidates
        │                                                       (looked up, passers stored)
        ▼                                                              ▲
Initial  — one first page of open issues per eligible repository       │
        │  hasNextPage?                                                │
        ▼                                                              │
Deep Diver — every remaining page, from the stored cursor, until exhausted
        │                                                              │
        ▼                                                              │
Freshness — once a repository's last sync is older than ingest.freshness_interval_seconds,
             fetch only what changed since its last completed pass: open issues
             updated since (upserted) and issues closed since (marked CLOSED)
                                                                       │
GH Archive reader (ingestd, off by default) — hourly public event files; issue events on
             stored repositories applied when newer, unknown repositories → candidates.
             While it is current, Freshness runs every ingest.archive.repair_interval_seconds.
```

The coordinator (`internal/ingest/coordinator.go`) runs one cycle every
`ingest.claim_interval_seconds`: recompute eligibility (throttled), reclaim expired leases
(throttled), seed never-ingested repositories, re-queue stale ones, then
check tracked (saved / in-progress) issues on GitHub, then claim and submit work to the Initial
and Deep pools, then — if `ingest.discovery.enabled` — start a due discovery sweep and claim its
windows for the Discovery pool. If `ingest.archive.enabled`, the same cycle (before the disk-guard
early return) queues newly ended archive hours, claims due ones for the Archive pool, and looks up
one batch of repository candidates. The first full crawl (every eligible repository through one pass)
took 3 days 23 h for 7.19M open issues; see §9 of the architecture doc.

### Rules

- **Every eligible repository enters the pipeline.** No repository priority, ranking, activity
  score or demand score anywhere in the crawl path. Ordering is FIFO on `next_retry_at`, and
  the freshness cycle orders by oldest successful sync — that is cycle fairness, not a
  judgement about repository value. Data-science ranking comes later, from the collected data.
- **Eligibility is abuse control and pacing, not quality.** The star floor exists because
  anyone can open an issue on a throwaway repository to burn crawl budget.
- **Some repositories are excluded by name.** `excluded_repositories` (migration 00028) lists
  repositories that pass every threshold and are still not worth crawling — an issue tracker
  used as a database, a public archive. A listed repository is ineligible (`excluded` in
  `eligibility_reason`): `ingest.RecomputeEligibility` applies the list, `cmd/repoimport`
  checks it before `eligible()` so a re-import cannot store one, and `ClaimJobs` only claims
  jobs of eligible repositories, so a job queued earlier does not run: it stays `pending`
  (parked, `ingest_jobs_parked`), and runs if the repository qualifies again. The coordinator also
  deletes the stored issues of excluded, **archived** and `duplicate` repositories (never ones a user saved)
  on the closed-issue prune interval. Add one with
  `INSERT INTO excluded_repositories (full_name, reason) VALUES ('owner/name', 'why')`.
- **Only eligible repositories are stored.** The dataset is ~4.2M records and ~288k pass the
  filter; storing the rest cost ~2.8 GB of rows nothing is ever allowed to crawl. A full
  import pass is ~16 minutes and idempotent, so **loosening** a threshold means re-running
  `cmd/repoimport`, which is cheaper than warehousing the dataset.
- **Tightening is live.** `ingest.RecomputeEligibility` re-tests the rows that *are* stored
  against the current thresholds on a throttled cycle, so raising the star floor, or metadata
  going stale, takes repositories out of the crawl without a re-import. The rule it applies
  must stay identical to `ingest.Eligible` (used by `cmd/repoimport` and the Discovery Sweeper)
  — if one changes, both change. It never re-judges `not_found` or `duplicate` rows.
- **New repositories come from discovery.** The Discovery Sweeper (`internal/ingest/discovery.go`)
  searches `stars:>=min_stars fork:false archived:false pushed:<window>` and stores what passes
  `ingest.Eligible` and is not excluded; `EnqueueInitialJobs` takes it from there. A full sweep
  (whole pushed-within range) runs every `ingest.discovery.full_interval_seconds`, an incremental
  one (since the last `done` sweep) every `ingest.discovery.incremental_interval_seconds`. A
  window over GitHub's 1,000-result search cap is replaced by its halves down to
  `ingest.discovery.min_window_seconds`. Windows (`discovery_windows`) are a leased, fenced queue
  like `ingest_jobs`; a sweep with a failed window ends `failed` and does not advance the
  watermark, and is reopened `ingest.discovery.failed_retry_seconds` later (`RetryFailedSweep`:
  its failed windows back to pending, attempts reset), rather than waiting for the next full
  sweep. A GitHub 502/504 on search counts as too expensive and halves the page size, like
  `RESOURCE_LIMITS_EXCEEDED`. Discovery checks the shared `crawler` budget against its own, higher
  `ingest.discovery.rate_limit_floor`, so it yields before the crawl. `POST /sync/discovery`
  (ingestd, admin) starts a full sweep.
- **`github_database_id` is a repository's identity; `full_name` is display data.** Discovery
  and the first page of every crawl pass record GitHub's repository id. A row found by id under a
  new name is renamed. An id-less row that is the same repository as one holding its id (a
  dataset row imported under the post-rename name) is marked `duplicate`: never crawled, never
  re-judged, its unsaved issues pruned, nothing deleted. A name now held by a different
  repository is reported (`repository_identity_conflicts_total`) and left as it is.
- **Unknown is not a negative.** A NULL metadata field means "not imported yet", never
  "fails the check" — the same principle as unlabeled issues not being `BEGINNER`.
- **Cursors advance only after the rows they describe are committed**, in the same
  transaction, along with `repositories.last_synced_at`. Never mark a repository complete on a
  truncated run.
- **The watermark only ever trails the truth.** `repositories.issues_synced_through` is the
  *enqueue* time of the last pass that completed (`ingest_jobs.pass_started_at`), set in the
  transaction of the pass's last page. A freshness pass fetches changes after it (`ingest_jobs.since`);
  NULL means no pass has completed and the next one is a full crawl. A pass pages two
  connections in lockstep (`cursor`/`open_done`, `closed_cursor`/`closed_done`) and is done
  only when both are exhausted or the page cap is hit.
- **Closed issues are detected, never inferred.** The crawl asks GitHub for issues closed since
  the watermark and marks exactly those `CLOSED`. "Not seen in this pass" is not evidence of
  closure — the page cap means many open issues are never seen. Issues a user saved or is
  working on are also re-checked directly (`ingest.tracked_issue_check_seconds`), so their
  closure shows up within that interval rather than at the repository's next pass.
- **Closed issues are pruned, unless a user holds them.** Once closed on GitHub for
  `ingest.closed_issue_retention_days`, an issue is deleted — never one in `saved_issues`
  (`saved_issues` would cascade).
- **Repository metadata is live once crawled.** The first page of every pass writes GitHub's
  current stars, forks, watchers, open-issue count, archived flag and last push over the
  dataset snapshot (`repositories.metadata_refreshed_at`; NULL = still the snapshot), so
  eligibility follows live values. `issue_count` stays the dataset's open+closed total — the
  import's `no_issues` rule reads it; `open_issue_count` is the live open count.
- **One failing repository must never block the others.** `NOT_FOUND` marks a repository
  unavailable; `RESOURCE_LIMITS_EXCEEDED` halves the page size and retries down to 1;
  secondary rate limits pause the lane; anything else gets exponential backoff and, past
  `ingest.max_attempts`, lands in `failed`.
- **Workers share one GitHub budget** (`github_budget`, lanes `crawler` and `interactive`) and
  claim work atomically with `FOR UPDATE SKIP LOCKED` plus a lease. Never assume a worker owns
  the whole API budget.
- **The crawler never spends the interactive reserve.** Both lanes draw on one counter: GitHub
  counts per credential, and `CanProceed` reads every lane's row as one. The crawler lane (crawl,
  discovery) stops while `ingest.interactive_reserve_percent` of the limit is still left. Its floors
  are measured above that reserve. Work a user is waiting on (the tracked-issue check) runs on
  `interactive` and spends down to `ingest.rate_limit_floor`. A secondary limit pauses both lanes.
  New user-facing GitHub work goes on `interactive`; new background work goes on `crawler`. There
  is one GitHub App installation: never add an identity to get more limit.
- **Every job write is fenced on its lease.** Expired leases are reclaimed continuously
  (`ingest.lease_reclaim_interval_seconds`), so a stalled worker's job can be re-claimed while
  it still runs. Any write that finishes a job must match `status = 'running' AND
  lease_owner = <claimer>` and treat zero rows as `ErrLeaseLost` (rolling back the page's
  transaction) — never a bare `WHERE id = $1`.
- **Postgres is the queue.** `ingest_jobs` is the durable state store; Redis is not involved.
- **The archive carries freshness only while it is current.** The GH Archive reader
  (`internal/ingest/archive.go`) reads one gzipped file per hour from GH Archive, as a stream.
  IssuesEvent and IssueCommentEvent carry the whole issue, and those for stored, eligible
  repositories go through `writeIssueTx`: inserted, or updated only if newer than what is
  stored. A closed issue that was never stored is not inserted, and while the disk guard is on
  nothing is. Pull requests are skipped. `archive_hours` is a leased, fenced queue like
  `ingest_jobs`. While every hour up to `ingest.archive.max_lag_seconds` ago is done or
  `missing`, the freshness crawl drops to `ingest.archive.repair_interval_seconds` and the
  incremental discovery sweep stops. A `failed` hour holds coverage back, so the daily crawl
  resumes on its own (`archive_fallback`). Reset a failed hour with `UPDATE archive_hours SET
  status = 'pending', attempts = 0 WHERE hour = ...`. Archive writes never touch
  `repositories.last_synced_at` or `issues_synced_through`: the watermarks stay crawl
  watermarks.
- **Unknown repositories in the archive are candidates, not repositories.** `repo_candidates`
  holds repositories with issue activity that are not stored. They are looked up with
  `github.LookupRepositories` (aliased, ≤100 per call, on the crawler lane at discovery's floor)
  and stored through the Discovery Sweeper's `upsertDiscoveredTx`, so eligibility, exclusion and
  identity rules are the same. One that was not stored is looked up again after
  `ingest.archive.candidate_recheck_days`, if it was active since. A quiet one is deleted then
  (`PruneRepoCandidates`), which keeps the table from growing with every small repository.

### Not built

- **`'discovery'` job type** — `ingest_jobs.job_type` permits it, but the Discovery Sweeper has its
  own tables (`discovery_sweeps`, `discovery_windows`); nothing writes it. Tests use it as a job
  type no running coordinator claims.
- **Freshness job type** — stale repositories are re-queued as `job_type='initial'`; nothing
  writes `'freshness'`.
- **Pull requests from the archive** — the reader skips `PullRequestEvent` (and its `merged`
  action). Nothing tracks pull requests yet. Bodies in issue events only refresh bodies already
  cached; they never add one.

---

## Deployment

Single VPS, `compose.prod.yaml`, Caddy, for everything but the site, which is on Vercel.
Full runbook: `DEPLOYMENT.md` (local only).

- **The site is on Vercel; the API has its own host.** `vercel.json` (repo root) builds the npm
  workspace, serves `web/dist` with the CSP and security headers, and falls back to `shell.html`.
  deploy.yml's `web` job builds and deploys it with the Vercel CLI after the API deploy succeeds,
  skipped until the `VERCEL_ORG_ID`/`VERCEL_PROJECT_ID` variables exist; Vercel's own deploys from
  git are off, so nothing ships without CI. The bundle calls `VITE_API_URL`
  (`https://skillissues-api.dizziedbliss.lol`, Caddy's `{$API_DOMAIN}`, routes at the root) cross-origin:
  the two hosts are one site, so the Lax session cookie travels, and go-api's CORS and CSRF trust
  `FRONTEND_URL`. The job fails if `vercel.json`'s `connect-src` does not list that host. The
  Vercel project is linked by id only: its git integration is disconnected, and must stay so.
  Rollback for the site is Vercel's Instant Rollback.

**Local-only documents.** `DEPLOYMENT.md` and `docs/runbook.md` (the operator runbook) describe
how production is run, so they live only on the maintainer's machine: they are in `.gitignore`, were
removed from the index, and must **never be committed or pushed**. Do not `git add -f` them. The
same applies to any new operational document of that kind (host setup, access, rotation or
restore procedures): create it, add it to `.gitignore`, and keep it out of tracked files. Do not
link to them from tracked docs (`README.md`, `ARCHITECTURE.md`); code comments may name them. They
remain in earlier git history, which is not rewritten.

- `.github/workflows/ci.yml` gates everything: backend (vet/gofmt/build/test), web/desktop
  (typecheck then a real build, not just a type check), and a buildx build of the four
  deployed Dockerfile targets (`api`, `ingestd`, `flowd`, `migrate` — not `repoimport`) plus
  `docker compose config` on both compose files and `caddy validate` on the Caddyfile. It runs on pull requests and as the deploy
  gate, not on plain pushes, and a `changes` job (`dorny/paths-filter`) skips any job whose
  paths did not change. A skipped job counts as passing, so when adding a job, add its paths
  to the filter. `workflow_dispatch` runs everything.
- `.github/workflows/deploy.yml` runs on push to `main`: `ci` → `build-push` (builds and pushes
  `ghcr.io/dizziedbliss/skillissues-{api,ingestd,flowd,migrate}` tagged `sha-<short>` and
  `latest`) → `deploy` (scp + ssh, runs `scripts/deploy-remote.sh` on the host) → `web` (Vercel). A deploy cannot
  happen without CI passing, and runs queue (`concurrency: production-deploy`): two at once on the host
  replace each other's containers. Rollback is `workflow_dispatch` with an older `sha-` tag — deploys
  always pin that, never `latest`. `build-push` attests provenance and cosign-signs each image by
  digest; `deploy-remote.sh` resolves the tag to a digest, runs `cosign verify` (signer: `deploy.yml`
  on `refs/heads/main`) and deploys that digest, so the host needs `cosign`. Actions are pinned to
  commit SHAs, workflows default to `contents: read`, `govulncheck` gates backend changes, and
  Dependabot (`.github/dependabot.yml`) keeps all of it current.
- Caddy takes the visitor's address from `CF-Connecting-IP` only when the peer is in
  Cloudflare's ranges (`trusted_proxies` in the `Caddyfile`), and sends go-api exactly that address
  in `X-Forwarded-For` (`header_up X-Forwarded-For {client_ip}`). That works with or without
  Cloudflare proxying: without it the header is ignored and the peer is the client. Keep the
  ranges current with https://www.cloudflare.com/ips/.
- Caddy is the only service publishing a public port. `{$API_DOMAIN}` proxies to `go-api` at the
  root and returns **404 for `/metrics`, `/health/detail` and `/admin/*`** — those are internal only
  (admin is reached over ssh, see `DEPLOYMENT.md`). Cloudflare's "Always Use HTTPS" must stay off:
  it redirects the ACME HTTP-01 challenge before it reaches Caddy, which redirects plain HTTP
  itself. A second site, `{$STREAM_DOMAIN}`, proxies only `GET /stream`
  to `flowd` (`flush_interval -1`) and returns 404 for everything else. `go-api`, `ingestd` and
  `flowd` have no published ports in prod.
- **Secrets are in git, encrypted.** `secrets/prod.env.sops` (SOPS + age, to the maintainer's key and
  the VPS's key; `.sops.yaml`). `deploy.yml` ships it as ciphertext and `deploy-remote.sh` runs
  `decrypt-env.sh` on the host to write `.env`; CI never holds a key. A present file that will not
  decrypt fails the deploy rather than falling back to a stale `.env`. `scripts/check-secrets-encrypted.sh`
  (CI) fails on a plaintext file under `secrets/` or a committed age private key. Never commit a plaintext secret.
- **Backups are encrypted on the host.** `scripts/backup-db.sh` pipes `pg_dump | gzip | age -r <public
  key>`; the private key is off the server, so the host can write backups but not read them.
  `BACKUP_DESTINATION` is required and has no default: `r2` uploads with rclone (retention is an R2
  lifecycle rule), `local` keeps encrypted files in `BACKUP_DIR` (pruned after `BACKUP_RETENTION_DAYS`)
  with **no off-site copy** unless the operator pulls them. It fails loudly (non-zero) on any missing
  setting, failed dump or failed upload, never keeps an unencrypted copy, and retries an unuploaded
  `r2` dump next run.
- Monitoring runs in both compose files: Prometheus, Grafana (seven provisioned dashboards in
  `monitoring/grafana/dashboards/`), node-exporter, cAdvisor, postgres- and redis-exporter, and the
  log pipeline. **Logs outlive the containers a deploy recreates:** Grafana Alloy
  (`monitoring/alloy/config.alloy`) ships every container's output to Loki
  (`monitoring/loki/loki.yml`, the `loki_data` volume), labelled `service` and, for the Go
  services' JSON, `level`; kept for `LOKI_RETENTION_PERIOD` (336h in prod). **No IP address is
  stored**: Alloy replaces every IPv4/IPv6 address with `<ip>` (Caddy's access log carries the
  visitor's), which keeps the Privacy policy's "logs with IP addresses go within days" true; only
  Docker's rotating json-file logs hold them. Alloy reaches Docker only through
  `docker-socket-proxy` (GETs for containers, networks and logs; `POST: 0`) on the internal
  `docker-api` network: a socket mounted `:ro` still answers every API call, which is root on the
  host. cAdvisor still mounts `/var/run` (and so the socket) read-only. Read it in the
  "SkillIssues · Logs" dashboard or Explore. Loki has no shell for a healthcheck, so Prometheus
  scrapes both (`up{job=~"loki|alloy"}`). Both run with `GOMEMLIMIT`: Go ignores a container's
  memory limit otherwise. The deploy prunes unused images older than `IMAGE_PRUNE_AFTER_HOURS` (72).
  Grafana (:3000) and Prometheus (:9090) bind to `127.0.0.1` and are reached over `ssh -L`.
  `deploy.yml` scp's `monitoring/` alongside the compose file. Runbook: `DEPLOYMENT.md`,
  *Monitoring*.
- **The services do not run as the database owner.** `go-api` connects as `skillissues_api`,
  `ingestd` as `skillissues_ingest` (migration 00029) and `flowd` as `skillissues_flow` (00033,
  `SELECT` on `settings` only): no ownership, no DDL, no `TRUNCATE`, no `COPY ... PROGRAM`.
  Passwords come from `API_DB_PASSWORD` / `INGEST_DB_PASSWORD` / `FLOW_DB_PASSWORD` via the
  `app-roles` one-shot (`db/app-roles.sql`). `migrate` and backups use the owner. Grants are
  explicit per table with no default privileges: **a migration that adds a table grants it** to
  the roles that use it, plus `USAGE` on its sequence — remember triggers run as the invoker, so
  a table a trigger writes needs granting to whoever fires it. `internal/migrate/roles_test.go`
  fails when a table is granted to neither role and not listed as deliberately ungranted.
- Grafana's Postgres datasource and postgres-exporter connect as **`metrics_ro`** (migration
  00017): read-only, 15s `statement_timeout`, explicit per-table grants. **Never grant it a
  credential column**, and a new table a dashboard needs must be granted in a new migration.
  Its password comes from `METRICS_DB_PASSWORD` via the `metrics-role` one-shot service.
- Two disk guardrails exist specifically because the host is 40GB: `ingest.max_db_size_mb`
  (coordinator stops enqueuing/claiming once `pg_database_size()` exceeds it) and
  `ingest.max_pages_per_repo` (caps the Deep Diver per repository regardless of `hasNextPage`).
  Both are live settings — see *Configuration* below.

---

## Conventions

These encode decisions already made. Please do not quietly reverse them.

### Configuration
Three layers, in precedence order:

| Layer | Source | Changeable | Holds |
|---|---|---|---|
| 1. Bootstrap | env (`internal/config`) | restart | `DATABASE_URL`, `REDIS_URL`, `PORT`, `FRONTEND_URL` |
| 2. Secrets | env (`internal/config`) | restart | tokens, client secrets, webhook secret |
| 3. Operational | `settings` table (`internal/settings`) | **live** | limits, page sizes, TTLs, intervals, worker counts, thresholds |

**Nothing operational is fixed.** If a number changes how the system behaves — a threshold, an
interval, a batch size, a page size, a worker count, a cap — it is a setting, not a literal.
The precedent is `ingest.eligibility.min_stars`, `ingest.eligibility.pushed_within_days`,
`ingest.eligibility.recheck_seconds`, `ingest.freshness_interval_seconds`,
`ingest.enqueue_batch_size`, `ingest.graphql_page_size`, `ingest.routine_workers`, `ingest.priority_workers`, `ingest.rate_limit_floor`,
`ingest.max_pages_per_repo`, `ingest.max_db_size_mb`, `ingest.metrics_refresh_seconds`,
`ingest.claim_interval_seconds`, `ingest.lease_seconds`, `api.session.ttl_hours`.
Every key with its default and bounds: `docs/settings.md`.

- **Never hardcode an operational value.** Add a key to `settings.Registry` **and** a seeded
  row in a migration. A key missing from the table falls back to its default and is logged
  as missing on every settings load.
- **Secrets never go in the `settings` table.** It is readable through an admin endpoint and
  reproduced in `settings_audit` and in database backups.
- Read values through `live.Current()` and hold the returned snapshot for the duration of an
  operation, so a mid-flight reload cannot change values underneath you.
- Change a value at runtime:
  ```bash
  curl localhost:8000/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"
  curl -XPUT localhost:8000/admin/config/ingest.routine_workers \
       -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 4}'
  ```
  `psql` works too, and hits the same validation trigger — which matters when a bad value has
  made the API itself unhealthy.
- Bounds live in the database trigger, which is the authoritative boundary; the registry
  mirrors them so the API can reject early with a useful message.
- Anything gated on concurrency must use `worker.Limiter`, not a fixed goroutine count, or the
  setting cannot actually be resized.

### Migrations
- **Never edit a migration that has been applied anywhere.** Add a new one.
- Keep migrations idempotent (`IF NOT EXISTS`) and pair every `Up` with a `Down`.

### Credentials and identity
- **The server holds zero user GitHub tokens.** Not encrypted, not hashed, not for a minute.
  Sign-in uses the **Login app**, a no-scope OAuth App: the server reads the numeric `id`,
  login, profile name and avatar, then discards the token. Desktop sign-in is a device flow on the same app,
  posted once to `POST /auth/github/device`, which asks GitHub (`POST
  /applications/{client_id}/token`) whether the Login app issued it. Fork, push and pull
  requests are the desktop client's job, with a token that stays on the user's machine.
- Server-side work (crawling) uses the server credential: the GitHub App's installation token,
  or `GITHUB_TOKEN` as a fallback. It never acts for a user.
- **`github_id` is the only identity.** A username is display data GitHub lets another account
  take, so `auth.UpsertUser` never matches on it; a colliding username on another row is
  cleared, not merged.
- A session ends only on logout, expiry (`api.session.ttl_hours`), `POST /auth/logout-all` or
  purge. Revoking the Login app on GitHub does not end it — there is no token to invalidate.
- Browser requests: a state cookie (`__Host-oauth_state`) binds an OAuth callback to the
  browser that began it; `http.CrossOriginProtection` refuses cross-site mutations; a request
  carrying the session cookie must send `Content-Type: application/json`. Bearer requests are
  exempt. Callback errors redirect with a fixed error code, never reflected text.
- Never log a token, a prefix of one, or its length — a prefix leaks secret bytes and the
  length identifies the credential type.

### Data model
- Issue identity is `UNIQUE(repository_id, number)`. Upserts must conflict on that, not on
  `github_id`.
- Issue `updated_at` comes from GitHub's `updatedAt`, never from import time or a DB default.
  When *we* fetched it is separate: `first_seen_at` (set on insert, never changed) and
  `last_synced_at` (every write of GitHub data to the row — crawl, on-demand check, webhook).
  Both are NULL for rows that predate migration 00019; that means unknown.
- **Unlabeled issues are not `BEGINNER`.** Absence of a difficulty label is `UNCLASSIFIED` —
  unknown difficulty, not easy difficulty — and `difficulty_score` is NULL for it.
- Preserve raw GitHub labels; derived tags are not a substitute.
- **Notifications are written by the database.** The `issues_notify_closed` trigger inserts an
  `issue_closed` row in `notifications` for everyone who saved the issue, on the `OPEN` →
  `CLOSED` transition — whichever path closed it.
  Never add a Go code path that writes them for issue closure; `internal/notification` only
  reads and marks them read (`GET /notifications`, `POST /notifications/{id}/read`,
  `POST /notifications/read-all`). The web app shows them on Saved issues, with a dot on Profile while any are unread.

### Frontend
- Use a maintained Markdown library with sanitisation; never hand-roll Markdown rendering.
  Issue bodies are attacker-controlled — anyone can open an issue on a crawled repo.

### Interface design
The maintainer's direction for every screen (landing, docs, app, desktop). Hold new work to it; the maintainer
may override any of it, but don't drift back on your own.
- **Hairlines, not boxes.** Sections are split by space and 1px rules. No bordered cards, no red
  frames around panels. A box is for something that floats (a menu, a dialog, the search palette).
- **The accent is rare, and means something:** the main button on a page, where you are (the nav item),
  what's earned or live (level bars, XP, badges), and headings. Section headings (h2) and small
  group labels are accent-coloured; page titles (h1) and all body text stay in the text colour.
  Short large text can carry colour without tiring the eye; long text can't (and saturated red
  text on dark flickers).
- **Rules and chips are neutral.** `--rule`, `--chip` and `--chip-strong` (`app/App.module.css`)
  mix from the text colour, not the accent, so every theme stays calm. Labels are quiet chips.
- **Light mode is soft, not sharp.** Warm, slightly dim paper (`#f1eee7`) and softened ink
  (`#26231f`, 13.5:1): past AAA without the glare of near-white. Never pure white or black.
  Brand colours live in `styles/base.css` and are copied into the Default and Midnight presets and
  the favicon: change them together.
- **Fewer words on screen.** No paragraph explaining a control that can explain itself. A filter's
  meaning is the first line of its dropdown; a search box's hints are its placeholder and a `/` key.
- **One component per job.** The search bar is `explore/SearchBar.tsx` everywhere (Explore, the
  search page, the palette); a profile's header is `profile/Header.tsx` (own and public). Reuse,
  don't restyle a copy.
- **Whole rows are targets.** A card or a row opens its thing anywhere you click; no extra "View"
  button beside it. Targets are at least 44px where they stand alone.
- **Red buttons use bold text** (JetBrains Mono 700 is loaded): lighter weights break up on red.
- **Check both modes and a phone** (390px) before calling a screen done, with real data, and
  keep `data-ui` hooks stable: themes select them (`web/content/docs/themes.md`).

### Working with AI tools
This project is built with AI coding tools, mostly Claude, under the maintainer's direction and review.
`CONTRIBUTING.md` (*Using AI tools*, *What gets a pull request closed, even when CI passes*) holds
the rules for contributors who use them, and `AGENTS.md` is the short version of this file for
agents that read that instead. When a rule here changes, change `AGENTS.md` with it.

### Testing
- **Tests must pass with Redis stopped.** Redis is cache and rate limiting only; sessions and
  authentication fall through to Postgres.

---

## Key design decisions

- **Balanced search**: `GET /issues?search=` matches three categories in precedence order:
  - **Repository:** the name contains the term.
  - **Title:** full-text match on the title and labels.
  - **Language:** the repository's main language is the term, or one of its tech is (main
    language first).

  Authors are not searched. Each category is its own indexed query, run concurrently
  (`issue/search_categories.go`). The categories are interleaved rank by rank (`interleave`), each
  issue once, at the first rank any category reaches it, so every page is a slice of one list
  however deep it is read. Each category reads at least `api.search.per_category_cap` × 3 and twice
  as deep as the page asked for, up to `api.search.sample_size` — twice, because a category can lose
  at most as many issues as it contributes, which keeps a shallow read a prefix of a deep one. A
  named `sort=easiest|newest` sorts the whole read (`sortMatches`); none keeps match order. Never
  fold them back into one query that ORs the matches across the join: no index serves it, and on
  the full catalog it ran past the query timeout. `?category=repository|title|language` returns
  one category alone, paged on its own up to `api.search.sample_size` (the web app's "view more").
  - Title orders a bounded sample (`api.search.sample_size`) by stars.
  - Repository and Language take at most `api.search.per_repo_cap` issues per repository.
  - Terms under 3 characters skip the Repository category, since its trigram index can't serve
    them.
- **Catalog browsing reads in index order**: the catalog order (`difficulty_score`, `updated_at
  DESC`, `id DESC`) is exactly `issues_open_browse_idx` / `issues_open_difficulty_browse_idx`
  (migration 00035). Change one and you must change the other, or every page sorts every open issue.
  `issue.BrowseIssues` serves the first page and every `?cursor=` page as a run of ordered index
  ranges, never a UNION, which Postgres sorts in full. `?page` > 1 is the OFFSET query.
  A language with fewer than `api.issues.rare_language_threshold` open issues is sorted instead,
  because the planner cannot tell a rare language from a common one. Browse queries use
  `pgx.QueryExecModeExec` so they are never run from a cached generic plan. Newest with a difficulty
  filter reads each difficulty's range on its own and merges them (`newestByDifficulty`; rated ones
  through `issues_open_rated_newest_idx`, 00063): walking every open issue by `created_at` to find the
  few rated ones ran past the timeout.
- **Issue bodies are the client's to fetch; the server is the fallback.** Clients read bodies
  from GitHub (`ApiClient.loadIssueBody`) and call `GET /issues/{id}/body` only when GitHub
  refuses them for their own rate limit. That route serves `issue_bodies` (a bounded cache,
  `ingest.bodies.cache_max_mb`, saved issues' bodies never evicted). On a miss it is capped per
  caller (`api.bodies.fallback_misses_per_hour`, refused when Redis cannot count) and fetched by
  ingestd (`POST /bodies/{id}`, admin token) on the `interactive` lane. go-api holds no GitHub
  credential and never writes a body. Bodies are attacker-controlled; render them only through
  `packages/markdown`.
- **The default feed is For you, fair to the whole backlog** (`issue/foryou.go`, the maintainer's decision
  2026-10-21, replacing the plain shuffle): `sort=foryou` (the cursor-paging default) walks repositories
  in `repositories.shuffle_key` order, each one's best open issue (one slot per repository, however
  many issues it has), starting at a point seeded per person per UTC day (`forYouSeed`; person 0 is
  everyone signed out, one draw a day, so pages cache), wrapping round the end once. The walk is read
  in chunks of `api.foryou.pool_repos`; each chunk is ordered by a Gumbel-top-k draw (Plackett-Luce)
  on Fit × Doable × Need (`forYouScore`, each part raised to its `api.foryou.weight_*_percent`,
  noise scaled by `api.foryou.randomness_percent`). Fit is personal at every level (profile languages
  and tech, level and target, interests, languages and repositories merged in before); Doable is what
  predicts resolution (labelled, short discussion, recent, contributing guide, repository health);
  Need is waiting time and few recent serves. A drawn chunk is kept in Redis (`cache.ForYou`,
  `api.foryou.chunk_ttl_seconds`) and serves are counted there per day (`api.foryou.exposure_*`),
  both failing open. The cursor carries the seed, start, chunk position and offset, so every page of a
  walk agrees. Issues carry `for_you` reasons, shown on cards. Explore offers For you, Easiest and
  Newest; `shuffle` stays in the API (learning paths pick from it). Signed in, Explore's languages
  default to the profile's (absent `language` = the profile's, empty = all), like difficulty. Don't add
  popularity weighting: stars never enter the score.
- **Catalog filters share one builder** (`issue/filters.go`): any-of within a list, all-of across
  filters. Labels match case-insensitively through `catalog_lower_labels()`, tech through the
  generated `repositories.tech`. A filtered page whose matches are rare (`issue.isRare`, under
  `api.issues.rare_language_threshold`) is read through the filters' own indexes and sorted,
  rather than by walking an order index. That count is itself bounded (`api.issues.rare_probe_ms`):
  matches too many to count quickly are too many to sort. A difficulty filter is stated on
  `difficulty_score` alone (fixed per difficulty by `sync.ClassifyLabels`, NULL for UNCLASSIFIED) so the
  order indexes can range over it. Never add `difficulty = ...` beside it: two conditions for one fact
  made the planner multiply estimates (994 BEGINNER rows expected, 188,970 there) and read every match
  through the wrong index, 25 s. Expression statistics on `catalog_lower_labels(labels)` and a larger sample for
  `repositories.tech` (00048) keep it from guessing 0.5% for a label, which made it probe the labels index
  once per repository of a tech (bug + Go: 7s, 41ms once estimated). The shuffled feed walks repositories
  without a rarity count (only a languages-only filter checks the cached counts), finds each one's
  labelled issues a batch of repositories at a time (`repository_id = ANY(batch)`: a common label's GIN
  posting list read once per batch, not once per repository), after
  `api.issues.shuffle_walk_max_repos` repositories reads the rest of the page from the matches, and
  after `api.issues.shuffle_walk_budget_ms` returns the page short with a cursor. **A short page with
  a non-null `next_cursor` is not the end**; only a null cursor is. go-api's connections run with `DB_WORK_MEM_MB` (32 in prod)
  so label bitmaps stay exact: a lossy one re-checked ~550k rows and took 8s. The dropdown values are `catalog_facets`, recomputed by
  ingestd every `ingest.catalog.facets_refresh_seconds`; never count them per request.
- **Assignees are recorded, not inferred**: `issues.assignees` (00053) is GitHub's assignee count from
  the crawl (`assignees { totalCount }`) and from GH Archive events that carry assignees; a source
  that doesn't say keeps the stored value (`COALESCE`), and NULL (not read since) counts as
  unassigned. `?unassigned=true` (the web app's default) filters on it while walking the order
  indexes; it is deliberately left out of `hasFilters`, so it never triggers the rarity probe.
  `repositories.contributing_url` (00056) is read the same way (`contributingGuidelines { url }` on a
  pass's first page, only an https github.com page is kept) and linked first on the issue page.
- **XP is a ledger of verified merges, read with the server's credential** (plan:
  `~/.claude/plans/xp-and-tracking.md`). ingestd reads each signed-up person's public pull requests
  on the **interactive** lane (`internal/ingest/prs.go`: `user(login) { pullRequests(orderBy:
  UPDATED_AT DESC) }`, several people per request), keeps only those to catalog repositories
  (`pull_requests`, `pull_request_issues` via `closingIssuesReferences`), and pages a pass down to
  `pr_sync.synced_through`; the first pass is the backfill. The watermark moves only when a pass
  completes, and a login whose `databaseId` isn't the account's `github_id` stores nothing. When a
  pass completes, `internal/xp.Plan` (pure, every number an `xp.*` setting) plans the awards over
  everything stored and they are written to `xp_events` (append-only; unique per person, source and
  pull request), so "first merge in a repository" and caps never depend on page order. Levels,
  per-language levels, streaks (UTC ISO weeks) and badges are computed from the ledger on read
  (`xp.Summarise`, `internal/progress`, `GET /me/xp`). A paid merge notifies through a trigger
  (`pr_merged`). The rules are documented in `/docs/xp`: keep both in step.
- **Quests and learning paths ride the same pass** (migration 00059): after paying merges, ingestd
  pays the quests the ledger now completes (`xp.Quests`, only `merged` rows, key `quest:<key>`) and
  marks `learning_path_steps` done from merged pull requests that close them, finishing and paying the
  path (`path:<id>`). go-api starts, swaps and abandons paths (`internal/learnpath`, issues picked by
  `issue.Service.Pick` off the shuffled feed from a random point, wrapping once: unassigned,
  unclaimed, one per repository). An issue swapped out is kept in `learning_path_skips` (00062) and
  never offered again on that path.
- **Repository health is read with the stats, flagged on read** (migration 00061): a pass's first page
  also reads the last `ingest.health.sample` merged pull requests (`healthOf`: merges by people who
  aren't OWNER/MEMBER/COLLABORATOR, and the median hours to merge; GitHub's first-timer associations
  don't survive the merge, so they aren't used). `issue.Service` turns the stored numbers into
  `repository_health.welcoming`/`slow` against `api.health.*` per request, so a threshold change
  needs no re-crawl.
- **Public profiles are opt-in; the leaderboard is off** (migration 00060): `users.public_profile`
  (default false, set through `PUT /profile`) gates `GET /users/{login}` (a private or missing account
  is the same 404) and every leaderboard row. `GET /leaderboards?period=month|all&language=` answers
  404 until `api.leaderboards.enabled`. Both are `no-store`, so turning a profile off takes effect at
  once. Web: `/app/u/:login`, `/app/leaderboard` (`app/people/`).
- **Claims are SkillIssues-only and held in memory for lists** (`internal/claim`, migration 00058):
  one active claim per issue, `api.claims.max_active` per person; active until released, or past
  `api.claims.days` only while the claimer has an open tracked pull request that closes the issue
  (the rule is one SQL predicate, `claim.active`). go-api re-reads the claimed set every
  `api.claims.refresh_seconds` (`cmd/api/claims.go`; a claim made here applies at once) and
  `issue.Service` marks `claimed` and, with `unassigned`, drops them after the cursor, like reports.
  A person's own claim comes from `GET /issues/{id}/claim`, so issue pages stay cacheable.
- **Reports are private and hide by count, never by query**: a signed-in person reports an issue, a
  repository or a public theme (`POST /reports`, `internal/report`, migration 00052; one open report
  per person per target, `api.reports.per_user_per_day`). Targets reported by
  `api.reports.hide_after` different people, or upheld, are read into memory every
  `api.reports.hidden_refresh_seconds` (`cmd/api/hidden.go`; a failed read keeps the last and counts
  `reports_hidden_refresh_errors_total`) and dropped from browse, search and recommendations at
  `issue.Service`'s exported methods, after the cursor is taken, so a page that loses every issue
  still continues. Never filter them inside the catalog SQL: it would change the index plans. The
  community theme listing (`GET /themes/public`) leaves hidden themes out. Review is
  `GET/POST /admin/reports[/resolve]`: upheld, a repository goes into `excluded_repositories`, a
  theme becomes private, an issue stays hidden; dismissed, it comes back. The same review is in the app for
  moderators (`/app/moderation`, Reports in the Profile menu): `GET /moderation/reports` and `POST
  /moderation/reports/resolve` answer 404 unless `users.moderator` (00065), which only an operator sets,
  in the database; nothing in the API writes it. A decision re-reads the hidden set at once.
- **Live counts are kept, never counted per request**: `GET /stats` (open, closed since tracking
  began, arrived/closed today, indexed issues and repositories, signed-up developers, recent
  activity) reads `catalog_stats`. Statement-level triggers on `issues` (migration 00043) append one
  delta row per repository a statement touched, whichever path wrote it, and ingestd folds them in
  every `ingest.stats.rollup_seconds` and recounts exactly every `ingest.stats.recount_seconds`, in
  one REPEATABLE READ snapshot so nothing is counted twice (`ingest/stats.go`). `/stats` is public
  to any origin (the flow site reads it). `GET /repositories/lookup?name=owner/repo` says whether a
  repository is indexed and why not.
- **Catalog responses are cacheable only when signed out**: public `Cache-Control` + `ETag` and a
  Redis copy shared through `singleflight`. A signed-in response is ordered by the caller's
  languages and is `private, no-store`.
- **One account per GitHub id**: web and desktop sign-in both resolve to the `github_id`
  row. The case-insensitive unique index on `lower(username)` is why a username that moved to
  another account is cleared on the old row first.
- **Rate limiting**: Redis `INCR` counter per user (signed in) or per client (signed out; IPv6 keyed by /64, `X-Forwarded-For` read from the right past `TRUSTED_PROXIES`, which must parse or the API will not start) —
  180 / 1,000 per `api.rate_limit.window_seconds`. Only `/health`, `/health/detail`,
  `/metrics` and `GET /stats` are exempt; `/auth/*` (login) **is** limited. `/stats` is exempt because the
  landing polls it every 10s (counted, one open tab spent a whole signed-out hour and every other request
  from that address then failed); go-api keeps its answer in memory for `api.stats.max_age_seconds`
  (at least 1s), so an unlimited caller costs one database read per lifetime. Fixed window: the first request sets
  the key's expiry with `EXPIRE … NX`, so later requests never push it back. Fails open
  when Redis is down — within `api.cache.redis_timeout_ms`, and after a failure Redis is skipped
  (`cache_skipped_total`) except one probe per `api.cache.redis_retry_seconds`.

- **Themes are someone else's code on your page**: a theme (`internal/theme`) is a TOML file of
  design tokens checked against a fixed schema — an unknown key is an error — plus an optional
  stylesheet checked token by token against an allowlist: no `url()` but base64 raster `data:`
  images, no `@import`/`@font-face`, no function off the list, no escapes. Themes are per account
  and shareable by id once public; making one private takes it off every other account. They
  restyle the signed-in app only, never the landing page. `packages/theme` repeats every check on
  the same fixtures (`packages/theme/fixtures`), and the web app's CSP blocks fetches regardless.
  Routes: `GET/POST /themes`, `POST /themes/validate` (a dry run, no sign-in), `GET/PUT/DELETE
  /themes/{id}`, `GET/PUT /me/theme` (`{"id"}`, `{"preset"}` for a built-in as it is, kept in
  `users.active_preset`, migration 00051, or null). Built-ins (`Presets`, both sides) include
  well-known palettes. Problems carry their line (`line 7: …`, `stylesheet line 3: …`), word for word
  the same on both sides (`fixtures/problems`). Tokens added after the first release (`colors.link`,
  `info`, `accent-hover`, `code-background`, `[light]`/`[dark]`, typography case and tracking,
  `sidebar-width`, `shadow.focus`, `motion.stagger`, background image placement) are optional and
  the app falls back without them; Default doesn't set them, so presets inherit nothing wrong.
  `@property` is allowed only for `--theme-*` names and is lifted out of `@scope` by `applyTheme`;
  `scroll()`/`view()` are allowed for scroll-driven animation. The app reads back what CSS can't
  branch on (`transition`, `reduce`, `[background] pattern`) as `data-*` attributes on its root.

### Not built
- **Live issue state on demand.** The server answers no freshness check; ingestd's
  tracked-issue check keeps saved issues' state correct, and the archive reader everyone else's.

---

## Branching and commits

Branch naming: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `refactor/<topic>`,
`chore/<topic>`, `refactor/<topic>`.

Commit style: [Conventional Commits](https://www.conventionalcommits.org/) —
`feat(search): …`, `fix(electron): …`, `perf(ingestd): …`.

**Push after every commit.** `git commit` and `git push` are one step on the working branch,
so work is never stranded on one machine. Never push directly to `main` — PRs gate that.
