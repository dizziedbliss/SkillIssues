# Contributing to SkillIssues

Thanks for being here — typo fixes, bug reports, tests and features are all welcome, and nobody
expects you to understand the whole codebase on day one. Ask questions in [Discussions](https://github.com/dizziedbliss/SkillIssues/discussions) or on your PR.

This is production code: it runs against the real GitHub API and attributes real contributions
to real accounts. The non-negotiable rules are in [CLAUDE.md](CLAUDE.md) (written for AI
assistants, but it's the most complete statement of the project's conventions — worth a read).

## What you need

- Git, Docker with Compose v2
- Go 1.26, Node.js 22, npm 11 — if you run services natively or work on the clients
- A GitHub token for `GITHUB_TOKEN` (no scopes needed; the crawler reads public data). For
  login, forking and PR flows locally you also need your own GitHub OAuth App — see
  `.env.example`.

## Where things live

```text
backend/cmd/          api, ingestd, flowd, migrate, repoimport, healthcheck
backend/internal/     auth, issue (search, bodies), ingest (crawler), xp and progress (XP),
                      claim, learnpath, theme, settings, metrics, middleware, migrate/migrations, …
web/src/              React web app
desktop/src/          main/ (Electron main process), preload/, renderer/ (React)
packages/             client (typed API), theme, markdown — shared by web and desktop
monitoring/           Prometheus and Grafana config
docs/                 API reference, settings reference, ingestion design
```

[ARCHITECTURE.md](ARCHITECTURE.md) explains how the pieces fit.

## Running locally

### Everything in Docker

```bash
cp .env.example .env
docker compose up -d --build
```

Then load repositories once — see *Load repositories* in [README.md](README.md#2-load-repositories).

### Services natively, databases in Docker

```bash
docker compose up -d postgres redis

cd backend
export DATABASE_URL="postgresql://skillissues:skillissues@localhost:5432/skillissues"
go run ./cmd/migrate                       # schema first

export REDIS_URL="redis://localhost:6379/0"
export FRONTEND_URL="http://localhost:5173"
export GITHUB_TOKEN="<your token>"
go run ./cmd/api                           # :8000
go run ./cmd/ingestd                       # :8002, in another shell (optional)
```

### Web and desktop

`web`, `desktop` and `packages/*` are one npm workspace with one lockfile. Install **once, at
the repo root**, with `npm ci` (dependencies are pinned — don't `npm install`):

```bash
npm ci
cd web && npm run dev            # http://localhost:5173
cd desktop && npm run dev        # Electron with hot reload; API at http://localhost:8000
```

## Checks

These are what CI runs; please run the ones for the parts you touched.

```bash
cd backend && go vet ./... && gofmt -l . && go build ./... && go test -short ./...
cd web && npm run typecheck && npm run build
cd desktop && npm run check && npm run build
```

`-short` skips tests that need live GitHub. Tests must pass with Redis stopped.

## Branches and commits

- Branch from `main`: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`. (Use
  `chore/` for refactors — an existing branch named `refactor` blocks `refactor/<topic>`.)
- [Conventional Commits](https://www.conventionalcommits.org/): `feat(search): …`,
  `fix(desktop): …`, `perf(ingestd): …`, `docs: …`.
- One concern per PR. Say what changed, why, and how you tested it. `main` only changes through
  reviewed PRs, and every merge to `main` deploys to production.

## Rules that reviews will hold you to

**Go**
- Never discard an error. Log it, return it, or count it — a failure nobody can see is a bug.
- Thread `ctx` into every database, Redis and HTTP call; use the shared `pgxpool`.
- No operational literals (limits, intervals, sizes, worker counts). Add a live setting
  instead — [docs/settings.md](docs/settings.md#adding-a-setting) shows how.
- Never log a token, or any part or length of one.
- Writes to GitHub (fork, push, PR) use the user's own token, never the server's.

**Database**
- Migrations live in `backend/internal/migrate/migrations/` and are append-only: never edit one
  that has been applied anywhere. Add a new numbered file with `-- +goose Up` and
  `-- +goose Down`, idempotent (`IF NOT EXISTS`).
- Issue identity is `(repository_id, number)`. Unlabelled issues are `UNCLASSIFIED`, never
  `BEGINNER`.

**Web and desktop**
- No `any`; declare types for API payloads (they belong in `packages/client`).
- Render issue bodies only through `@skillissues/markdown` — they're attacker-controlled.
- Desktop: keep `contextIsolation`, `sandbox` and the typed bridge (`src/shared/contract.ts`); a new
  boundary rule goes in `src/main/security.ts` with a test. Run git asynchronously so the UI never blocks.

## Using AI tools

SkillIssues itself is built with AI coding tools, mostly Claude (through Claude Code). A person
directs every change, reads it, tests it and reviews it before it merges; [CLAUDE.md](CLAUDE.md) is
the brief those tools work from. You're welcome to use AI tools too, on the same terms:

1. **Say so.** Tick the box in the pull request template and name the tool. It changes nothing
   about whether we merge; it tells the reviewer where to look harder.
2. **You own every line.** Read all of it before you open the PR, and be ready to explain any line
   in review. "The AI wrote it" is not an answer to a review comment.
3. **Run it.** Run the checks above, and use the change: a screenshot for anything visible, the
   command and its output for anything else. Unrun code gets closed.
4. **Keep it to the task.** Assistants like to rewrite nearby code, reformat whole files, rename
   things and add dependencies. Undo everything you didn't mean to change.
5. **Check every name it gives you.** Functions, flags, settings, API fields and packages that
   don't exist are the classic AI mistake, and an invented package name is how attackers get code
   installed ("slopsquatting"). Every new dependency is verified by hand.
6. **Give it the rules.** Point your tool at [CLAUDE.md](CLAUDE.md) before it writes anything.
7. **Never paste secrets** (`.env`, tokens, keys) or anyone's private data into an AI tool.
8. **Tests must test.** A generated test that asserts nothing, or asserts whatever the code does
   today, is worse than none.

Pull requests, issues and comments that are clearly generated and unreviewed (mass-produced, never
run, not about this project) are closed without review. Doing it again gets you blocked.

## What gets a pull request closed, even when CI passes

CI shows that the code builds and the tests pass. It doesn't show that the code is safe. This
project holds real people's GitHub identities, and every merge deploys to production, so reviews
look for these on purpose. Any of them, without a clear reason stated in the PR, means it's closed:

- **A new place data goes.** A host the server, the site or the desktop app didn't talk to before;
  a wider CSP (`vercel.json`, the desktop's CSP), CORS rule, or list of links the desktop may open
  (`desktop/src/main/security.ts`); analytics, telemetry, tracking pixels or third-party scripts.
- **Data leaving.** Sending tokens, cookies, session ids, email addresses, IP addresses or anything
  else about a person anywhere new, or writing them to a log. The server holds no user GitHub
  token, ever (see *Credentials and identity* in [CLAUDE.md](CLAUDE.md)).
- **Code that hides what it does.** Minified or encoded blobs, long base64 or hex strings, `eval`,
  `new Function`, importing code from a URL, `dangerouslySetInnerHTML` outside
  `packages/markdown`, a shell built from strings (`sh -c`, `shell: true`), or behaviour that only
  happens on some date, for some user, in some environment or behind an unexplained variable.
- **Weakened guards.** A test skipped, deleted or made to pass by changing what it checks; a
  security rule loosened; a rate limit, size limit or timeout removed or raised without a reason;
  an error caught and ignored; an applied migration edited.
- **Dangerous database changes.** `DELETE`, `DROP` or `TRUNCATE` without an obviously right
  condition, new grants to a database role, or anything that reads or exposes credential columns.
- **Dependency surprises.** A new dependency without a reason in the PR, one installed from a git
  URL or tarball, a name one letter off a popular package, or unrelated version bumps (Dependabot
  does those).
- **Pipeline and infrastructure changes** without an issue agreed first: `.github/workflows/`,
  `Dockerfile`, the compose files, `Caddyfile`, `scripts/`, `secrets/`, `.sops.yaml`. Never add
  `pull_request_target`, widen a workflow's `permissions`, or replace a pinned action SHA with a tag.
- **Bulk diffs** that mix reformatting with logic, so the change that matters is hard to see.

Pull requests that touch any of these get a slower, line-by-line review and may be asked to split.
Anything that looks deliberately malicious is closed, reported to GitHub, and its author blocked.
If you spot something like this in code that's already merged, report it privately, as in
[SECURITY.md](SECURITY.md).

## Security issues

Don't report a vulnerability in a public issue or PR — see [SECURITY.md](SECURITY.md).

## Code of Conduct

Please read the [Code of Conduct](CODE_OF_CONDUCT.md). Be kind and assume good intent.
