# Changelog

All notable changes to the SkillIssues website and API are documented in this file. The desktop app's
are in [desktop/release-notes.md](desktop/release-notes.md), in the same format.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The website and API share one version, since they deploy together on every merge to `main`: MAJOR
when something people or the API relied on works differently, MINOR for new features, PATCH for fixes.

## [Unreleased]

## [2.7.0] - 2026-10-26

### Added

- The download page offers the new Windows setup (`SkillIssues-Installer.exe`), with the full offline
  installer beside it.

## [2.6.0] - 2026-10-26

### Added
- Home page: a real open issue that has waited, from today's For you picks (never a repository
  flagged slow, never over a year old, welcoming repositories first), with language and level chips
  to swap it, and four live numbers in place of the live bar (#149).
- Profile header shared with public profiles; Next up (pull requests in review, claims, the learning
  path, the closest quests); one merged pull request list with XP; earned badges as chips with their
  criteria on hover (#150).
- Docs: an On this page index and previous/next pages (#150).
- `CONTRIBUTING.md`: *Using AI tools* and *What gets a pull request closed, even when CI passes*; a
  pull request template; `AGENTS.md` for coding agents (#150).

### Changed
- One shared search bar (Explore, the search page, the palette); filters as self-explaining pills;
  quieter issue cards; the learning path as rows; Settings as rows; themes drawn as miniatures;
  reports as rows; the nav holds where you work, the menu what's about you (#150).
- Rules and chips mix from the text colour, the accent marks headings and what matters; light mode
  warmer and dimmer (ink 13.5:1); square corners follow the theme radius (#150).
- The footer rebuilt (#150).

### Fixed
- `GET /stats` no longer counts against the signed-out rate limit and is cached in memory, so an
  open landing tab can't lock a visitor out of the site (#148).

### Removed
- `benchmark/` (the early Python import prototype) and an unused image (#151).
- Git history rewritten to drop committed build binaries and personal email addresses from commit
  metadata; commit ids changed, code unchanged.

## [2.5.0] - 2026-10-24

### Added

- Backlog clock home page, repo finder, AA brand colours, readable docs (#143)
- Persistent logs in Loki, a parked-jobs gauge, and pruning of old images on deploy (#144)

### Changed

- Keep local backups for 3 days (#145)

### Fixed

- Failed discovery sweeps reopen instead of waiting for the next full sweep (#144)
- Accept older log lines in Loki, restart the log pipeline on deploy (#146)

### Security

- Strip IPs from stored logs, put Docker behind a socket proxy (#147)

## [2.4.1] - 2026-10-22

### Changed

- Describe what is true at migration 65 (#138)
- A front page like open-source projects have (#139)
- The wordmark at the top and foot, in light and dark (#141)

### Fixed

- Classify GitHub errors by type, not by their text (#136)
- Stars no longer raise a recommendation (#137)

## [2.4.0] - 2026-10-21

### Added

- Reporting, a beginner's checklist, community themes, and assigned issues left out (#106)
- Link the repository's contributing guide on every issue page (#111)
- XP, claims, quests, learning paths, public profiles and repository health (#112)
- The desktop app runs the web app's screens (#113)
- Working on, separate commit/push/PR steps, sign-in fixes, smaller app (#118)
- For you, the default feed — personal for everyone, fair to the whole backlog (#120)
- Updates ask first, What's new after an update, an assisted Windows installer (#121)
- The maintainer's favicon and wordmark (#122)
- Reports for moderators, in the app (#123)
- The maintainer's mark as logo, favicon and app icon; the new wordmark (#125)
- Keep the GitHub profile name for the greeting (#129)
- SEO and metadata; the mark only as the icon (#130)
- A dark, branded Windows installer (#131)
- Fork again when the fork is gone; pull requests show where they stand (#132)

### Changed

- Refactor/<topic> branches work again (#107)
- Flow tap leak check ignores timestamps (#126)
- Gofmt github.go (#133)
- Wordmark only in the installer and app; docs cross-checked (#134)
- Google Search Console site verification tag (#135)
- Dependency updates (#53, #110, #114, #115)

### Fixed

- Markdown-it 15.0.3 and vite 8.2.4 for the open security alerts (#108)
- Recommendations' default size is a setting; webhook failures counted and retried (#109)
- Name the Windows installer without spaces, so updates find it (#116)
- Give up a path and release a claim with a JSON body; Learning path in the nav (#117)
- Search depth and sort, Newest by difficulty, path swaps, one pull request list, and more (#119)
- Newest by difficulty reads the rated index (#124)
- State a rated difficulty once in Newest's reads (#127)
- A difficulty filter is the score alone (#128)

## [2.3.0] - 2026-10-20

### Added

- Filters, a fair shuffled feed, newest sort and facets (#72)
- Live catalog counts behind a public GET /stats (#75)
- User themes as validated TOML tokens plus an allowlisted stylesheet (#77)
- Rebuild the web app — landing page, content pages, theme engine, backdrop drafts (#79)
- Site on Vercel, API on its own host (#85)
- Show the backdrop drafts on the landing page, with a switcher (#91)
- Sign in, sign out and delete your account in the app (#93)
- Serve the Login app's client id for the desktop device flow (#96)
- One centred column over a faint glyph noise (#97)
- Rebuild on a typed IPC contract, with device-flow sign-in (#98)
- Back to the zig-zag, responsive, with smoother motion (#99)
- App shell from the designs, translations, dark mode (#100)
- Explore and the issue page, from the designs (#101)
- Search, transitions, profile, contributions, saved, and two review rounds (#102)
- Settings, themes for authors, and docs (#103)
- Light and dark in every theme; deleted themes leave Appearance (#104)

### Changed

- One indexed query per match category, run concurrently (#71)
- Score a bounded pool off the shuffled feed (#73)
- Bound filtered browsing on the full catalog (#76)
- Index labels per repository and fix the difficulty estimate (#78)
- Match a label for a batch of repositories at once; bound a shuffle page in time (#81)
- The documentation pages, served on the site (#82)
- Issue forms, discussion templates, private security reporting (#84)
- Publish the Terms of service and Privacy policy (#89)
- Statistics for label and tech filters (#92)

### Removed

- Remove the nginx frontend now that the site is on Vercel (#87)

### Fixed

- Send no-cache on the HTML shell (#74)
- One activity row per repository and commit (#80)
- The privacy page and SECURITY.md say what is true (#83)
- A missing asset is a 404 on Vercel, not the app shell (#86)
- Fetch /stats without credentials, so the live counts load (#90)
- Never inline a font, so the CSP doesn't block it (#94)
- Run one release at a time (#95)
- Stale themes leave the recents; light/dark switch beside the theme in use (#105)

### Security

- Enable GitHub sign-in on production (#88)

## [2.2.0] - 2026-10-19

### Added

- Reserve a share of the GitHub budget for interactive work (#65)
- Read GH Archive's hourly events for issue freshness (#66)
- Serve issue bodies from a bounded cache as a fallback (#67)
- Take the visitor's address from Cloudflare when proxied (#70)

### Changed

- Speed plan phases 1–3: budget reserve, GH Archive reader, issue body fallback (#68)

### Fixed

- Read the catalog in index order instead of sorting it (#64)
- Repair the panels that query removed tables or time out (#69)

## [2.1.1] - 2026-10-18

### Changed

- Exclude type-challenges, fe-interview and JARVIS (#63)

## [2.1.0] - 2026-10-17

### Added

- Discovery sweeper — find repositories the dataset lacks (#59)
- Live flow tap and flowd relay (visualizer backend) (#60)

### Changed

- Docs/security plan complete (#58)

### Fixed

- Don't redact the per-cycle tracked-issue select (#62)

### Security

- Add FLOW_DB_PASSWORD and STREAM_DOMAIN for flowd (#61)

## [2.0.0] - 2026-10-16

The major version: signing in and writing to GitHub work differently.

### Added

- Ingestion job tables on Grafana (#46)

### Changed

- **Breaking:** the server holds no user GitHub tokens. Sign-in reads your identity and discards the token; forking, pushing and pull requests moved to the desktop app, and the first XP system, which relied on stored tokens, was removed (#35)
- Low CPU/IO priority for the nightly dump; sizing note (#41)
- Reflect the finished security plan (#57)
- Dependency updates (#47, #50, #52, #54)

### Fixed

- Block /admin at Caddy, cap request bodies, validate deploy tag (#34)
- Exclude repositories by name; prune excluded and archived ones (#36)
- Remove stale registry keys resurrected by the #34/#35 merge (#37)
- Exclude OhMyGuus/I-Still-Dont-Care-About-Cookies (#42)
- Sign images in the legacy cosign format (#55)
- Cosign legacy signing flags (#56)

### Security

- Least-privilege database roles for api and ingestd (#38)
- SOPS-encrypted production secrets; age-encrypted R2 backups (#39)
- Set backup destination (#40)
- Supply-chain hardening (pins, signing, govulncheck, dependabot) (#44)
- Audit lows (rate-limit identity, CORS, timeouts, caps, stale webhooks, image pins) (#45)

## [1.2.0] - 2026-10-15

### Added

- Drop unused issue columns and indexes; chart catalog growth (#27)
- Incremental freshness that detects closed issues (#29)
- Prune closed issues; refresh repository stats from the crawl (#30)
- Notify users when an issue they saved or work on closes (#31)

### Removed

- Drop dead repository columns and the sync_state table (#28)

### Fixed

- Fail open on a down Redis without waiting 20s per request (#33)

## [1.1.0] - 2026-10-14

### Added

- Truthful metrics and a self-hosted Grafana dashboard (#15)
- Record when each issue was first seen and last synced (#24)

### Changed

- Fix deploy workflow failure when monitoring secrets are missing on host (#16)
- Rewrite every doc to match the code (#18)
- Run only the jobs a change touches, and only once (#25)
- Pin Ubuntu 24.04 and move actions to their Node 24 majors (#26)

### Fixed

- Fail fast on missing secrets instead of reusing others (#17)
- Reload Caddy and Prometheus so config changes take effect (#20)
- Drop secrets it never uses; feat(ui): Link PR for hand-opened pull requests (#21)
- Grafana secret key; docs: SECURITY.md (#22)
- Stop Grafana's OOM restart loop (#23)

### Security

- Audit findings — swallowed errors, lost XP, lease fencing, rate limiter, ingestd auth (#19)

## [1.0.0] - 2026-10-13

**Production deployment and a unified client architecture.** 1.0.0 is the release where SkillIssues actually went live, and where the web and desktop
clients stopped drifting apart. The app now runs on a real VPS behind CI-gated deploys, and
both clients share one typed API layer and one set of UI components instead of reimplementing
the product twice.

### Added

#### Production deployment (first real deploy of this project)
- **CI/CD pipeline**: `.github/workflows/ci.yml` gates every push — backend (`vet`, `gofmt`,
  build, test), web/desktop (typecheck **and** a real production build, not just a type check),
  and a `buildx` build of the `api`, `ingestd`, `migrate` and `web` Docker targets plus
  `docker compose config` on both compose files. `.github/workflows/deploy.yml` builds and pushes
  `ghcr.io/dizziedbliss/skillissues-{api,ingestd,migrate,web}` on every push to `main`, gated on
  CI passing, then deploys to the VPS over SSH. Rollback is a `workflow_dispatch` with an older
  immutable `sha-` tag.
- **One root `Dockerfile`** with named targets (`api`, `ingestd`, `migrate`, `web`,
  `repoimport`) built from the repo root, replacing four scattered Dockerfiles, so the `web`
  target can actually see the shared `packages/*` it imports. New root `.dockerignore` keeps
  the build context to tens of MB instead of shipping the multi-GB dataset.
- **`compose.prod.yaml`**: a from-scratch production compose file — Caddy as the only
  internet-facing service (automatic TLS, same-origin `/api/*` routing so there's no CORS and
  no cross-site cookie problem), Postgres/Redis sized for the 4GB/2vCPU VPS, explicit memory and
  CPU limits and log rotation on every service, `DB_MAX_CONNS` made configurable.
- **Disk guardrails**: `ingest.max_pages_per_repo` caps how deep the crawler paginates into any
  one repository; `ingest.max_db_size_mb` pauses enqueuing/claiming once the database crosses a
  configured size. Both live settings, both surfaced on `GET /health/detail` alongside current
  DB size, so the ceiling is visible before it's hit.
- **Automated backups**: a systemd timer running daily `pg_dump` with 14-day retention on the
  host.
- The production repository catalog was populated for the first time: **287,998 eligible
  repositories** imported from the Kaggle metadata snapshot; `ingestd` has been crawling live
  since.

#### Shared client architecture
- **`packages/api-client`**: one typed client for every API endpoint, used by both clients —
  `Issue`/`Contribution`/`Profile` types, `ApiError` that keeps `status` and `detail` so a 401
  is never confused with a 429 or a network failure, `AbortController`/cancellation support,
  pluggable transport (`fetchTransport` for web, an IPC-routed transport for desktop), and the
  recommendations ETag/stale-cache fallback that used to be duplicated and drifting in both
  clients.
- **`packages/ui`**: shared React components — `Modal` (body scroll lock, Escape-to-close,
  focus trap, backdrop click — none of which either client had before), `ConfirmDialog`,
  `Toast`, `IssueCard`/`IssueList`, `WorkingList`/`SubmissionList`, `DifficultyTabs`,
  `SectionTabs`, `Badge`, `ProgressBar`, `Skeleton`.
- **Desktop renderer rewritten in React + TypeScript**, replacing 1,780 lines of plain-JS
  `innerHTML` rebuilds. The old renderer blanked and refetched up to six endpoints from fifteen
  call sites on nearly every interaction (including a single bookmark click) — the rewrite
  moves to per-resource state with per-resource refetch, request sequencing so a fast
  filter→page→search sequence can't let a stale response win, and real React reconciliation, so
  scroll position now survives a list update.
- **Web's `App.tsx` rebuilt** onto the same shared packages, cut from 1,691 to roughly 1,100
  lines of routing and web-specific glue — its hand-rolled issue card, login form, working/
  submission lists, and detail modal are now the shared `packages/ui` components.
- npm workspaces at the repo root (`web`, `desktop`, `packages/*`) so `@skillissues/*` resolves
  as a real `node_modules` symlink, with one lockfile for the whole tree.

#### Issue lifecycle
- **Closed-issue detection**, on demand only (no background sweep): the existing fork/clone
  freshness check and a new GitHub `issues` webhook handler persist a closed/reopened state
  using GitHub's own `updatedAt`. A new batched `POST /issues/freshness` lets a client check
  only the handful of issues it's actually displaying.
- **Sticky saved issues carry their lifecycle** (`none`/`working`/`submitted`/`merged`) as a
  badge; saved issues never disappear regardless of issue or contribution state.
- **Re-solve**: starting a fresh attempt on an already-submitted or already-merged issue works
  end to end in both clients, backed by the existing idempotent fork and partial-unique-index
  contribution model.
- **Real `submitted_at`**, separate from the fork/start time (`created_at`) both clients were
  previously mislabeling as the submission time.
- A "View issue" action was added to working/submission cards on both clients (desktop's
  working cards were missing it entirely).

### Changed
- OAuth is now the only *supported* login path in both clients' UI; the legacy PAT-login
  endpoint stays registered server-side but is disabled by default behind a live setting.
- `FRONTEND_URL` is parsed once as a proper multi-value list instead of being treated as a
  single string everywhere, fixing cookie `Secure` detection, the OAuth redirect target, and the
  CORS `Access-Control-Allow-Origin` header.
- Issue-body "not ready yet" responses now carry a structured `{"status":"queued","reason":...}`
  instead of an ambiguous error shape, and the web client retries with real exponential backoff.
- A failed `/saved`, `/working`, or `/contributions` fetch now keeps the previously loaded list
  and shows a retry affordance instead of silently rendering empty — this previously made
  real data look like it had disappeared whenever the client hit the unauthenticated rate limit.
- `GET /recommendations` now accepts `difficulty`/`page`/`limit` filters through the shared
  client, matching what the backend already supported.

### Removed
- The co-contribution commit-trailer feature (`GET /github/co-contribution-trailer` and both
  clients' UI for it) — removed per product decision, not a regression.
- The PAT login form from both clients' UI.
- `database/migrations/` and `database/schema.sql` as a second, never-applied, out-of-sync
  schema source — the embedded goose migrations under `backend/internal/migrate/migrations/`
  are the only schema source now.
- The stale `k8s/` manifests, which referenced images nothing built and committed a plaintext
  database password.

### Fixed
- The web production Docker image's `HEALTHCHECK` always failed: it resolved `localhost` to
  `::1` while nginx only listens on IPv4, so the check failed even while the container correctly
  served real traffic.
- CI never installed the shared `packages/markdown` module's own dependencies, so every build
  that exercised it was silently relying on a dev machine's leftover `node_modules`.
- The deploy workflow could run and attempt an SSH deploy even when its own CI job had failed,
  caught only by an unrelated empty-tag guard; the condition now requires either an actual CI
  success or an explicit manual rollback dispatch.
- `cmd/ingestd` logging a prefix and length of the GitHub token at startup — removed outright;
  credentials are never logged, not even partially.
- A swallowed `QueryRow` error in `GET /health/detail` and a discarded `UPDATE` error in
  `submitContribution`, both now surfaced.
- Account deletion (`authPurge`) previously discarded its own delete errors and always reported
  success; it is now a single cascading `DELETE` whose error is returned to the caller.

[unreleased]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.7.0...HEAD
[2.7.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.6.0...web-v2.7.0
[2.6.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.5.0...web-v2.6.0
[2.5.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.4.1...web-v2.5.0
[2.4.1]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.4.0...web-v2.4.1
[2.4.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.3.0...web-v2.4.0
[2.3.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.2.0...web-v2.3.0
[2.2.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.1.1...web-v2.2.0
[2.1.1]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.1.0...web-v2.1.1
[2.1.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v2.0.0...web-v2.1.0
[2.0.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v1.2.0...web-v2.0.0
[1.2.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v1.1.0...web-v1.2.0
[1.1.0]: https://github.com/dizziedbliss/SkillIssues/compare/web-v1.0.0...web-v1.1.0
[1.0.0]: https://github.com/dizziedbliss/SkillIssues/releases/tag/web-v1.0.0
