# Documentation content

These Markdown files are the public documentation pages. `web/src/pages/docs.ts` reads them at
build time, renders them with markdown-it (raw HTML off) and prerenders each at its `slug`;
`DocPage.tsx` shows them with a sidebar by `section`. The legal pages (`privacy`, `terms`,
`security`, `licenses`) are kept here for reference but the site serves `web/src/pages/content.ts`'s
versions of those. Adding a file here adds a page; no code changes are needed.

Every claim in these pages describes the code as it is now. When behaviour changes, change the
page in the same pull request.

## File format

One file per page. Each file starts with YAML frontmatter, then the page body in Markdown.

```yaml
---
title: Getting started
slug: /docs/getting-started
section: Guides
summary: Four steps from the catalog to your first pull request.
draft: false
updated: 2026-10-21
---
```

| Key | Type | Meaning |
|---|---|---|
| `title` | string | The page title, in sentence case. Render it as the page heading. The body does not repeat it as an `h1`. |
| `slug` | string | The URL path of the page, with a leading slash. Unique across all pages. It is the address the site already serves the page at, or `/docs/<name>` for a page that is new. |
| `section` | string | Where the page belongs in navigation. One of `SkillIssues`, `Community`, `Resources`, `Support`, `Legal & Trust`, `Guides`. |
| `summary` | string | One sentence describing the page. Use it for the page's meta description and for link cards on the index. |
| `draft` | boolean | `true` when the page is waiting for the owner's review. Show a visible draft notice. No page is a draft today. |
| `updated` | date | `YYYY-MM-DD`, the day the page last changed. |

A page keeps the address the site already uses for it (`about.md` is `/about`, `faq.md` is
`/faq`, `contribution-guide.md` is `/docs/contributing`). A page that has no existing address,
such as `themes.md`, lives under `/docs/<filename>`. The `slug` in the frontmatter is the
address; the filename is not, so read it from there. The full list is in *Sections and pages*.

## Body conventions

- Headings start at `##`. The page `title` is the only `h1`.
- Links between pages use the target page's `slug`, for example `[Difficulty ratings](/docs/difficulty-ratings)`.
- Links to GitHub and other sites are absolute URLs.
- Code blocks are fenced and carry a language tag (`toml`, `css`, `json`, `bash`).
- Tables are GitHub-flavoured Markdown.
- There is no raw HTML, no images and no embedded scripts.

## Sections and pages

| Section | Pages (file: address) |
|---|---|
| SkillIssues | about: `/about`, desktop-app: `/download`, documentation: `/docs`, skillissues-live: `/docs/skillissues-live`, changelog: `/docs/changelog` |
| Community | contribute: `/contribute`, discussions: `/docs/discussions`, suggest-a-repository: `/docs/suggest-a-repository`, leaderboard: `/leaderboard`, sponsor: `/sponsor` |
| Resources | getting-started: `/docs/getting-started`, how-it-works: `/how-it-works`, faq: `/faq`, contribution-guide: `/docs/contributing` |
| Support | contact: `/contact`, report-a-bug: `/docs/report-a-bug`, request-a-feature: `/docs/request-a-feature`, reporting: `/docs/reporting` |
| Legal & Trust | privacy: `/privacy`, terms: `/terms`, security: `/security`, licenses-and-attributions: `/licenses` |
| Guides | good-first-issues: `/docs/good-first-issues`, finding-issues: `/docs/finding-issues`, difficulty-ratings: `/docs/difficulty-ratings`, themes: `/docs/themes`, for-maintainers: `/maintainers`, public-api: `/docs/public-api` |

## Relationship to `web/src/pages/content.ts`

The web app currently has short versions of several of these pages in
`web/src/pages/content.ts`. These pages use the same addresses (`/about`, `/faq`, `/security`
and so on), so when the docs UI ships they replace those short pages in place and no redirects
are needed. The pages with no short version (the guides, `report-a-bug`, `request-a-feature`,
`discussions`, `suggest-a-repository`, `changelog` and `skillissues-live`) are new addresses
under `/docs/`.
