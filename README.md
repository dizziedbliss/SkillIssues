<h1 align="center">
    <a href="https://skillissues.dizziedbliss.lol">
        <picture>
            <source media="(prefers-color-scheme: dark)" srcset=".github/assets/wordmark-dark.png">
            <img width="420" alt="SkillIssues" src=".github/assets/wordmark-light.png">
        </picture>
    </a>
</h1>

<p align="center">
    <a href="https://github.com/dizziedbliss/SkillIssues/releases/latest"><img src="https://img.shields.io/github/v/release/dizziedbliss/SkillIssues?label=desktop" alt="Latest desktop release"></a>
    <a href="https://github.com/dizziedbliss/SkillIssues/actions/workflows/deploy.yml"><img src="https://github.com/dizziedbliss/SkillIssues/actions/workflows/deploy.yml/badge.svg" alt="Deploy status"></a>
    <a href="https://github.com/dizziedbliss/SkillIssues/blob/main/backend/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/dizziedbliss/SkillIssues?filename=backend%2Fgo.mod" alt="Go version"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/dizziedbliss/SkillIssues" alt="MIT license"></a>
    <a href="https://skillissues.dizziedbliss.lol/docs/changelog"><img src="https://img.shields.io/badge/website-2.8.0-c8190b" alt="Website version"></a>
</p>

<p align="center"><strong>Find an issue you can solve, in ten seconds.</strong><br />
Open issues from active GitHub projects, matched to your languages, skills and level.<br />
Pick one, fix it, get it merged, level up. Free and open source, every feature.</p>

<p align="center"><img width="800" alt="The home page: a real issue that has been waiting 260 days, and the promise to find one in ten seconds" src=".github/assets/landing.png" /></p>

## Why

Tutorials teach you to code. Nothing teaches you to *develop*: read a stranger's codebase, find
your way round an issue tracker, get a pull request merged by someone who doesn't know you.
Meanwhile millions of open issues sit on GitHub waiting for exactly that person.

SkillIssues connects the two. It keeps every open issue of hundreds of thousands of active
repositories fresh, works out which ones fit you, helps you set up a workspace, and pays you XP
when GitHub says your fix was merged.

<p align="center"><img width="800" alt="Explore: beginner TypeScript issues from real repositories, with their labels and languages" src=".github/assets/explore.png" /></p>

## Features

- **Every open issue, fresh:** ~8 million issues from every public repository with 25+ stars
  pushed in the last two years, kept current from GitHub's own event stream
- **For you:** a feed that weighs your languages, level and interests, how doable an issue is,
  and how long it has waited. Every repository gets one slot, and stars never count
- **Honest difficulty:** read from the repository's own labels (`good first issue`,
  `difficulty/easy`, …). No label means *unknown*, never *easy*
- **A beginner's checklist** on every issue: labelled, described, unassigned, no open pull
  request already, recent, short discussion
- **Search and filters:** by repository, title, language or tech, labels, topics and comment
  count, with results balanced so one kind of match can't drown the rest
- **XP for real merges only:** base XP by difficulty, bonuses for old issues and clean reviews,
  levels, streaks, badges, quests and learning paths. Opening pull requests earns almost nothing,
  so spamming maintainers gets you nowhere
- **"I'm working on this":** claim an issue so other SkillIssues users leave it to you
- **A desktop app** that forks, clones, branches, commits, pushes and opens the pull request
  with your own GitHub account, then opens VS Code
- **Themes:** twelve built-ins (Catppuccin, Dracula, Nord, Gruvbox, Tokyo Night, Rosé Pine…),
  or write your own in TOML and share it
- **Private by design:** the server never holds your GitHub token, not even encrypted. Public
  profiles are opt-in
- **Free, all of it:** no plans, no ads, no paywalled features. If you have money for open source,
  the site points you at the maintainers to sponsor instead
- **Easy on the eyes:** dark by default, and a light mode with warm, slightly dimmed paper for
  long reading
- **Watch it run:** [SkillIssues Live](https://flow.dizziedbliss.lol) draws every request, query
  and crawled page moving through the system, in real time, with anything of yours turned into
  symbols

## Get it

**On the web:** [skillissues.dizziedbliss.lol](https://skillissues.dizziedbliss.lol). Browse
without an account; sign in with GitHub to save issues, claim them and earn XP.

**On the desktop:** download from [the download page](https://skillissues.dizziedbliss.lol/download)
or [the latest release](https://github.com/dizziedbliss/SkillIssues/releases/latest). The app asks
before it downloads an update, every time.

<details>
<summary><strong>Windows</strong></summary>

Run `SkillIssues-Installer.exe`. It downloads the newest version from this repository's releases,
checks it against the release's SHA-512, and installs it, for you or for everyone on the PC. Run it
again later to update, repair or remove SkillIssues. No internet while installing? The release also
has `SkillIssues-Setup-<version>.exe`, the full offline installer.

Neither is code-signed yet, so SmartScreen will warn about an unrecognised app. Choose
**More info → Run anyway**.

</details>

<details>
<summary><strong>macOS</strong></summary>

Open the `.dmg` (Apple silicon) and drag SkillIssues to Applications.

The app isn't signed, so the first time, right-click it and choose **Open**. Updates open the
release page instead of installing themselves.

</details>

<details>
<summary><strong>Linux</strong></summary>

```bash
chmod +x SkillIssues-*.AppImage
./SkillIssues-*.AppImage
```

Where your system has no keyring, the app keeps your sign-in in memory only and asks again each
launch. It never writes a token to disk unencrypted.

</details>

> [!WARNING]
> Side effects may include merged pull requests, maintainers who know your name, and an
> uncontrollable urge to fix "just one more" `good first issue`. If symptoms persist, open
> [a discussion](https://github.com/dizziedbliss/SkillIssues/discussions) and tell the rest of us.

## How it works

```mermaid
flowchart LR
    Kaggle[/"repository dataset"/] -->|repoimport| PG[("Postgres")]
    Search[/"GitHub search"/] -->|discovery| Ingestd
    Archive[/"GH Archive<br/>hourly events"/] --> Ingestd
    GH["GitHub GraphQL"] <-->|crawl| Ingestd["ingestd"]
    Ingestd --> PG
    PG --> API["go-api"]
    API --> Web["web · Vercel"]
    API --> Desktop["desktop · Electron"]
    Desktop -->|"fork, push, PR<br/>with your token"| GH
```

- **`ingestd`** keeps the catalog. Repositories come from a public dataset and from GitHub
  search. Their open issues come from a crawl that commits each page together with its bookmark,
  so a crash never skips one. Changes arrive from GH Archive's hourly event files, which cost no
  API budget. Every eligible repository gets the same treatment: there's no priority queue for
  big projects.
- **`go-api`** serves the catalog in index order, so page 500 costs the same as page 1. It holds
  no GitHub credential at all.
- **The web app and the desktop app** run the same screens. Issue descriptions are read from
  GitHub in your browser and rendered through a sanitiser, because anyone can write one.
- **Every number** (page sizes, intervals, thresholds, XP amounts) is a live setting, changeable
  on the running system.

The full picture: [ARCHITECTURE.md](ARCHITECTURE.md) and
[the ingestion design](docs/ingestion-architecture.md).

## Run it yourself

You need Docker with Compose v2, and a GitHub token for `GITHUB_TOKEN`. The crawler reads only
public data, so a token with no scopes is enough.

```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env          # set GITHUB_TOKEN; everything else has a dev default
docker compose up -d --build
docker compose ps             # migrate and metrics-role exit 0; everything else running
```

The `migrate` service applies the schema; nothing needs setting up by hand.

<details>
<summary><strong>Loading repositories</strong></summary>

The catalog starts empty. Repositories come from the Kaggle dataset
*github-repository-metadata-with-5-stars* (`repo_metadata.json`, ~3 GB). Put it at
`data/repo_metadata.json`, then:

```bash
cd backend
DATABASE_URL=postgresql://skillissues:skillissues@localhost:5432/skillissues \
  go run ./cmd/repoimport --file ../data/repo_metadata.json
```

Only eligible repositories are stored. A full pass takes about 16 minutes and is safe to re-run;
`ingestd` starts crawling their issues on its own.

</details>

<details>
<summary><strong>What runs where</strong></summary>

| What | URL |
|---|---|
| Web app | http://localhost:5173 |
| API | http://localhost:8000 (`/health`, `/metrics`) |
| ingestd | http://localhost:8002/sync/status |
| flowd (live flow stream) | `curl -N 'http://localhost:8003/stream?format=ndjson'` |
| Grafana | http://localhost:3000 (`admin` / `admin` unless `GRAFANA_ADMIN_PASSWORD` is set) |
| Prometheus | http://localhost:9090 |

```bash
docker compose logs -f go-api ingestd
```

</details>

<details>
<summary><strong>The desktop app, from source</strong></summary>

You'll need Node.js 22 and npm 11. `web`, `desktop` and `packages/*` are one npm workspace with
one lockfile, so install once, at the root:

```bash
npm ci
cd desktop
npm run dev         # expects the API at http://localhost:8000
```

</details>

<details>
<summary><strong>The checks CI runs</strong></summary>

Go 1.27 (what CI and the images use; `go.mod` needs 1.26 or later):

```bash
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build && npm test
```

The database tests run against a migrated Postgres when `TEST_DATABASE_URL` is set, and pass
with Redis stopped.

</details>

## Configuration

Three layers: bootstrap values and secrets come from the environment (`.env`), and **everything
operational lives in the `settings` table**, validated by the database and changeable while the
system runs:

```bash
# read everything
curl localhost:8000/admin/config -H "Authorization: Bearer $ADMIN_SYNC_TOKEN"

# give the crawler four workers, right now, no restart
curl -XPUT localhost:8000/admin/config/ingest.routine_workers \
     -H "Authorization: Bearer $ADMIN_SYNC_TOKEN" -d '{"value": 4}'
```

Every change is recorded in `settings_audit`. Every key, with its default and bounds:
[docs/settings.md](docs/settings.md).

> [!TIP]
> `psql` works too, and hits the same validation trigger. That's handy when a bad value has made
> the API itself unhealthy.

## Themes

A theme is a TOML file of design tokens, plus an optional stylesheet. Start from any built-in and
change only what you name:

```toml
[meta]
name = "Night shift"
base = "tokyo-night"

[colors]
accent = "#ff7a59"

[dark]
background = "#0b0d14"
```

Write one in **Settings → Themes**: the editor checks it as you type and shows a live preview.
Share it, and it appears in the Community tab. Stylesheets are checked against an allowlist on
the server *and* in your browser, because a theme somebody shares is code running on your page.
The full reference: [the themes guide](https://skillissues.dizziedbliss.lol/docs/themes).

## Documentation

| | |
|---|---|
| [Changelog](https://skillissues.dizziedbliss.lol/docs/changelog) | What changed in each website and desktop version |
| [Getting started](https://skillissues.dizziedbliss.lol/docs/getting-started) | Your first issue, from sign-in to merge |
| [How XP works](https://skillissues.dizziedbliss.lol/docs/xp) | What earns XP, levels, streaks, badges, quests |
| [For maintainers](https://skillissues.dizziedbliss.lol/maintainers) | How your repository gets listed, and how to ask for it not to be |
| [Public API](https://skillissues.dizziedbliss.lol/docs/public-api) | Reading the catalog from your own code |
| [docs/api.md](docs/api.md) | Every route, with its auth and responses |
| [docs/database.md](docs/database.md) | Every table and column, and where its values come from |
| [ARCHITECTURE.md](ARCHITECTURE.md) | How the pieces fit |

## Contributing

Bug reports, fixes, docs, tests, translations and accessibility improvements are all welcome.
Read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md), then open
an issue (there's a form for each kind, including "my repository isn't indexed" and "exclude my
repository") or a pull request.

SkillIssues is built with AI coding tools, mostly Claude, with every change directed, read and
tested by a person before it merges. You're welcome to use them too: *Using AI tools* in
CONTRIBUTING.md has the rules, and [AGENTS.md](AGENTS.md) is the brief for coding agents
([CLAUDE.md](CLAUDE.md) is the full one).

Found a security problem? Please don't open an issue: use
[private vulnerability reporting](https://github.com/dizziedbliss/SkillIssues/security/advisories/new).
See [SECURITY.md](SECURITY.md).

## Whatcha think?

We'd love to hear from you. Ideas, questions, a theme you're proud of, your first merged pull
request: bring them to [Discussions](https://github.com/dizziedbliss/SkillIssues/discussions).

## License

[MIT](LICENSE)

---

<p align="center">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset=".github/assets/wordmark-dark.png">
        <img width="160" alt="SkillIssues" src=".github/assets/wordmark-light.png">
    </picture><br />
    Made by <a href="https://github.com/dizziedbliss">dizziedbliss</a>. Go fix something.
</p>
