# Documentation content

These Markdown files are the text of the public documentation pages. They hold content only.
The docs UI that renders them is built separately, from the owner's design.

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
updated: 2026-10-20
---
```

| Key | Type | Meaning |
|---|---|---|
| `title` | string | The page title, in sentence case. Render it as the page heading. The body does not repeat it as an `h1`. |
| `slug` | string | The URL path of the page, with a leading slash. Unique across all pages. |
| `section` | string | Where the page belongs in navigation. One of `SkillIssues`, `Community`, `Resources`, `Support`, `Legal & Trust`, `Guides`. |
| `summary` | string | One sentence describing the page. Use it for the page's meta description and for link cards on the index. |
| `draft` | boolean | `true` when the page is waiting for the owner's review. Show a visible draft notice. Only `privacy` and `terms` are drafts today. Those two also carry a note at the top of the body. |
| `updated` | date | `YYYY-MM-DD`, the day the page last changed. |

The filename is the last part of the slug, so `getting-started.md` has the slug
`/docs/getting-started`. The one exception is `documentation.md`, whose slug is `/docs`.

## Body conventions

- Headings start at `##`. The page `title` is the only `h1`.
- Links between pages use the target page's `slug`, for example `[Difficulty ratings](/docs/difficulty-ratings)`.
- Links to GitHub and other sites are absolute URLs.
- Code blocks are fenced and carry a language tag (`toml`, `css`, `json`, `bash`).
- Tables are GitHub-flavoured Markdown.
- There is no raw HTML, no images and no embedded scripts.

## Sections and pages

| Section | Pages |
|---|---|
| SkillIssues | about, desktop-app, documentation, skillissues-live, changelog |
| Community | contribute, discussions, suggest-a-repository, leaderboard, sponsor |
| Resources | getting-started, how-it-works, faq, contribution-guide |
| Support | contact, report-a-bug, request-a-feature |
| Legal & Trust | privacy, terms, security, licenses-and-attributions |
| Guides | finding-issues, difficulty-ratings, themes, for-maintainers, public-api |

## Relationship to `web/src/pages/content.ts`

The web app currently has short versions of several of these pages in
`web/src/pages/content.ts`, at paths such as `/about`, `/faq` and `/security`. The pages here
expand on them and say the same things. When the docs UI ships, those short pages can be
replaced by these, or the old paths can redirect to the matching `/docs/...` slug:

| Old path | New slug |
|---|---|
| `/about` | `/docs/about` |
| `/download` | `/docs/desktop-app` |
| `/docs` | `/docs` |
| `/docs/getting-started` | `/docs/getting-started` |
| `/docs/contributing` | `/docs/contribution-guide` |
| `/how-it-works` | `/docs/how-it-works` |
| `/contribute` | `/docs/contribute` |
| `/maintainers` | `/docs/for-maintainers` |
| `/faq` | `/docs/faq` |
| `/contact` | `/docs/contact` |
| `/security` | `/docs/security` |
| `/privacy` | `/docs/privacy` |
| `/terms` | `/docs/terms` |
| `/licenses` | `/docs/licenses-and-attributions` |
| `/leaderboard` | `/docs/leaderboard` |
| `/sponsor` | `/docs/sponsor` |
