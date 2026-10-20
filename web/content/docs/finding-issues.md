---
title: Finding issues
slug: /docs/finding-issues
section: Guides
summary: How the catalog is ordered, how to filter it, and how search works.
draft: false
updated: 2026-10-20
---

The catalog lists open issues from every repository SkillIssues tracks. You can browse it in one of three orders, narrow it with filters, or search it.

## Sorts

| Sort | What you get |
|---|---|
| Shuffled | The default. Every repository gets one slot, showing its best open issue. The order of repositories is redrawn once a day and is the same for everyone in between. |
| Easiest first | Rated issues first, easiest to hardest, then the most recently updated. Unrated issues come last. |
| Newest | The most recently opened issues first. |

The shuffled feed is meant to spread attention across the whole backlog rather than the biggest projects. A repository with ten thousand open issues gets the same single slot as one with three. Because it surfaces each repository's best issue, you see variety instead of one project's whole queue. If you want a specific order, pick easiest or newest.

If you're signed in, your preferred languages come first.

## Filters

You can combine any of these:

| Filter | Matches |
|---|---|
| Labels | The labels on the issue itself. Not case sensitive. |
| Languages | The repository's main language. |
| Tech | Any language GitHub reports for the repository, not just the main one. |
| Topics | The repository's GitHub topics. |
| Difficulty | Beginner, Intermediate, Advanced or Unrated. See [Difficulty ratings](/docs/difficulty-ratings). |
| Comments | A minimum and a maximum number of comments. A maximum of 0 means "no comments yet". |

### How filters combine

Within one filter, an issue matches if it has any of the values you picked. Choose the languages Go and Rust, and you see issues from Go repositories and from Rust repositories.

Across filters, an issue must match every one. Choose the languages Go and Rust, and the label "bug", and you see issues that are in a Go or Rust repository and are labelled "bug".

The values in each filter's list come from the catalog itself, most common first. The list and its counts are refreshed once a day, so a new label can take a day to show up in the list.

## Search

Type a term and press Search. Nothing is searched while you type, and the catalog doesn't change until you press Search.

Search looks in four places at once and shows a mix of all of them:

| Category | Matches |
|---|---|
| Repository | The repository's name contains your term. |
| Title | The issue's title or labels match your term, as words. |
| Language | The repository's language is exactly your term. |
| Author | The login of the person who opened the issue contains your term. |

A few details:

- An issue appears once, in the first category it matches.
- Each category is limited in size, and the categories are interleaved, so a common word can't fill the page with one kind of result.
- For the Repository and Language categories, a single repository contributes only a few issues, so a large project can't fill the category.
- Terms under three characters only match titles and languages.
- Characters like `%` and `_` are searched literally.
- Your filters still apply to a search.
- Search ignores the sort. Results are ordered by how they matched, with the categories mixed together.

## When a page looks short or empty

When you browse the shuffled feed, and especially when you combine filters that few repositories match, the catalog may stop looking after a couple of seconds and give you a short page, or even an empty one, along with a way to carry on from where it stopped. This doesn't mean you've reached the end. Only a response with no way to continue means the end. The website keeps reading a few more pages before it tells you nothing matches. If you're using the API directly, see [Public API](/docs/public-api) for what to do.

## Reading an issue

SkillIssues shows you what GitHub has about an issue, such as its title, labels, comment count and repository. The full text and discussion live on GitHub. Read them there before you start, and say on the issue that you're taking it. See [Getting started](/docs/getting-started).
