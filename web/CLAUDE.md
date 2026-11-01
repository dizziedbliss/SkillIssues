# Web (React 19 + Vite, on Vercel)

`src/landing/` the landing page · `src/pages/` docs and legal pages · `src/app/` the signed-in app
(also the desktop's screens) · `src/i18n/` strings · `content/docs/` the docs as Markdown.

## Workflow

```bash
npm ci                 # once, at the REPO ROOT: web, desktop and packages/* are one workspace, one lockfile
cd web
npm run dev            # Vite; the API at VITE_API_URL (default http://localhost:8000)
npm run typecheck      # fast
npm run build          # before a PR: tsc -b, the client and SSR builds, then prerender
```

- Never `npm install` (dependencies are pinned) and never install inside `web/`.
- Signed-in screens without GitHub sign-in (dev builds only): `?preview=signed-in` (`lib/devPreview.ts`).
- Check every screen in dark and light and at 390px wide, with real data, before calling it done.

## Rules

- **CSS Modules over custom properties.** No Tailwind, no CSS-in-JS.
- **Every visible string is a key in `src/i18n/en.ts`.** Translations are files of the same shape in
  `LOCALES`; a missing key shows English. The docs and legal pages stay English. The first render is
  English (it's what is prerendered); a stored locale (`si.locale`) applies after hydration.
- **The landing page's look is locked** (`landing/tokens.module.css`, light and dark): no user theme
  reaches it, or the docs and legal pages. Body text is never under 16px. The hero never shows an
  invented issue: until a real one arrives, the live open count stands in.
- **Brand colours are written once**, on `:root` in `styles/base.css`. The copies CSS can't reach (the
  Default and Midnight presets, `public/favicon.svg`, `theme-color`) change with them.
- **The mark** (the "i" as a bang on an ink tile) is the favicon and app icons only, never beside the
  wordmark (`Logo.tsx`). Fonts are self-hosted through `@fontsource`.
- **Motion** shares one easing (`--ease`, `--fast`, `--slow`) and all of it stops under reduced motion.
- **Written pages describe what the code does.** Docs (`content/docs/*.md`, frontmatter `slug`, `title`,
  `section`, `summary`) render at build time with markdown-it, raw HTML off (`docs.ts`), not through
  `packages/markdown`. The four legal pages stay in `content.ts`. Pages marked `draft` say so. Thresholds
  in `/docs/good-first-issues` match `BeginnerCheck.tsx`, and `/docs/xp` matches `internal/xp`: change
  both together.
- **Prerendered:** `npm run build` renders `/`, the legal pages and every docs page
  (`scripts/prerender.mjs`, `src/entry-server.tsx`); the client hydrates only when `data-route` matches;
  Vercel serves `shell.html` for everything else. Every claim in `src/seo.ts` must be true of the site.
  The build also writes `sitemap.xml` and `robots.txt` (private app pages disallowed). In the app,
  `useDocumentTitle` keeps the tab title right: a new page sets one.
- **The CSP** (`vercel.json`) allows fetches only to the API host and GitHub's API, images only from
  GitHub. A theme stylesheet is someone else's code: widen it only deliberately, and say why in the PR.
- **Issue bodies are attacker-controlled:** fetch from GitHub in the browser (`lib/issueBody.ts`, the API
  fallback only on GitHub's rate limit) and render only through `packages/markdown`.
- **The server holds no user token:** the profile's pull-request counts come from GitHub search in the
  browser, with no token (`lib/github.ts`).
- **Anything that differs between web and desktop goes through the platform** (`app/platform.ts`,
  `setPlatform`), never a fork of a screen.
- **Anything that can't simply be undone** (give up a path, release a claim, delete a theme or
  workspace) asks first through `useConfirm` (`Confirm.tsx`).
- **`data-ui` hooks are an API:** user theme stylesheets select them (`content/docs/themes.md`). Don't
  rename one casually.
- **Themes apply to the app's root only** (`app/theme.tsx`, `data-app`). Every theme has `[light]` and
  `[dark]` halves; the mode is the person's, dark by default; nothing chosen means Midnight. Page changes
  run the theme's `[motion] transition` through View Transitions, never under reduced motion.
- **App navigation decisions:** the Profile menu and the header never hold the same item. Search (the
  palette) leads to the search page, never to Explore. Explore's state lives in the address
  (`explore/query.ts`: absent `difficulty`/`language` = the profile's, empty = all). A repository or owner
  link opens Explore sorted easiest (the For you feed shows one issue per repository). The wordmark opens
  Explore afresh; the Explore nav item returns to it as it was left (`explore/where.ts`).
- Live counts poll `GET /stats` every 10s (React Query, paused in background tabs). Catalog paging always
  sends `?cursor=`, even empty (it selects the For you feed). A short page with a non-null
  `next_cursor` is not the end.

## Interface design (the maintainer's direction, for every screen)

- **Hairlines, not boxes.** Space and 1px rules split sections. A box is only for what floats (menu,
  dialog, the search palette). No bordered cards, no red frames.
- **The accent is rare and means something:** the main button, where you are (nav), what's earned or
  live (level bars, XP, badges), and section headings (h2) and group labels. Page titles (h1) and body
  text stay in the text colour.
- **Rules and chips are neutral:** `--rule`, `--chip`, `--chip-strong` (`app/App.module.css`) mix from
  the text colour, not the accent.
- **Light mode is soft:** paper `#f1eee7`, ink `#26231f`. Never pure white or black.
- **Fewer words.** No paragraph explaining a control; a filter's meaning is the first line of its
  dropdown, a search box's hints are its placeholder and a `/` key.
- **One component per job:** the search bar is `app/explore/SearchBar.tsx` everywhere; a profile header
  is `app/profile/Header.tsx`. Reuse; never restyle a copy.
- **Whole rows are targets.** A card opens its thing anywhere you click; no extra "View" button. Standalone
  targets are at least 44px.
- **Red buttons use bold text** (JetBrains Mono 700): lighter weights break up on red.

## Gotchas

- A mutation that sends the session cookie must carry `Content-Type: application/json`, even a DELETE:
  the API's CSRF check answers 415 otherwise. In `packages/client` that means `body: {}` on a DELETE. A
  DELETE without one once broke "give up this path" and releasing a claim.
- A logo or brand change also touches `public/og.png`, the favicon PNGs, `manifest.webmanifest`, and the
  desktop's and installer's icons and bitmaps (`desktop/build/`). One was missed before.
