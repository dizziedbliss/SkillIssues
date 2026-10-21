---
title: Finding issues
slug: /docs/finding-issues
section: Guides
summary: How the catalog is ordered, how to filter it, and how search works.
draft: false
updated: 2026-10-21
---

The catalog lists open issues from every repository SkillIssues tracks. You can browse it in one of three orders, narrow it with filters, or search it.

## Sorts

| Sort | What you get |
|---|---|
| Shuffle | The default. Every repository gets one slot, showing its best open issue. The order of repositories is redrawn once a day and is the same for everyone in between. |
| Easiest | Rated issues first, easiest to hardest, then the most recently updated. Unranked issues come last. |
| Newest | The most recently opened issues first. |

The shuffled feed is meant to spread attention across the whole backlog rather than the biggest projects. A repository with ten thousand open issues gets the same single slot as one with three. Because it surfaces each repository's best issue, you see variety instead of one project's whole queue. If you want a specific order, pick easiest or newest.

If you're signed in, your preferred languages come first.

Choose the sort under **filter**, beside the search box.

## Filters

Press **filter** beside the search box to open the filters. Each has a **?** that says what it matches. Pick what you want, then press **Apply**. **Clear** removes them all.

You can combine any of these:

| Filter | Matches |
|---|---|
| Labels | The labels on the issue itself. Not case sensitive. |
| Languages | The repository's main language. |
| Tech | Any language GitHub reports for the repository, not just the main one. |
| Topics | The repository's GitHub topics. |
| Difficulty | Beginner, Intermediate, Advanced or Unranked. See [Difficulty ratings](/docs/difficulty-ratings). |
| Comments | A minimum and a maximum number of comments. A maximum of 0 means "no comments yet". |

Issues someone is assigned to on GitHub, or has claimed on SkillIssues, are left out, since someone is already on them. Tick **Include issues someone is assigned to or has claimed** in the filters to see them too; they're marked **Assigned** or **Claimed**. The catalog learns who's assigned as it re-reads each repository, so for a while some assigned issues may still show.

The difficulty buttons under the search box are the same filter, one press away. Press one to add or remove it. With none pressed, you see every difficulty. If you're signed in, they start at the experience level in your profile.

### How filters combine

Within one filter, an issue matches if it has any of the values you picked. Choose the languages Go and Rust, and you see issues from Go repositories and from Rust repositories.

Across filters, an issue must match every one. Choose the languages Go and Rust, and the label "bug", and you see issues that are in a Go or Rust repository and are labelled "bug".

The values in each filter's list come from the catalog itself, most common first. The list and its counts are refreshed once a day, so a new label can take a day to show up in the list.

## Search

There are two ways to search.

**Search** in the menu (or `/`, or Ctrl+K, from anywhere in the app) opens a search over the page. Results come as you type, sorted into three kinds, and each kind has **View more**, which opens that kind on its own on the search page, with pages of its own. **See all results** opens every kind at once on the search page. Search never takes you back to Explore.

The search box on Explore searches the catalog you're browsing and shows every kind of match mixed together in one list. **x clear** empties it.

Search looks in three places:

| Kind | Matches |
|---|---|
| Repositories | The repository's name contains your term. |
| Titles and labels | The issue's title or labels match your term, as words. |
| Languages and tech | The repository's main language is your term, or it uses that language at all. Main language first. |

A few details:

- In a mixed list the kinds take turns, one result each, so a common word can't fill the page with one kind of result. An issue appears once, under the first kind to reach it. On its own, a kind lists every issue it matches.
- A mixed list goes as deep as you page, up to 2,000 results per kind.
- On Explore, choosing **Newest** or **Easiest** sorts the search's results that way. Otherwise they stay in match order.
- Type a GitHub username to find a person: if they've made their profile public, it shows first, under **People**.
- For repositories and languages, a single repository contributes only a few issues, so a large project can't fill the list.
- Terms under three characters only match titles and languages.
- Characters like `%` and `_` are searched literally.
- Your filters still apply to a search.
- Search ignores the sort. Results are ordered by how they matched.

## When a page looks short or empty

When you browse the shuffled feed, and especially when you combine filters that few repositories match, the catalog may stop looking after a couple of seconds and give you a short page, or even an empty one, along with a way to carry on from where it stopped. This doesn't mean you've reached the end. Only a response with no way to continue means the end. The website keeps reading a few more pages to fill each page before it tells you nothing matches.

## Pages

The numbers under the list are the pages you've already seen, so you can go back to any of them, plus the next one when there is one. The shuffled feed has no fixed page 7 to jump to, so the numbers grow as you go. If you're using the API directly, see [Public API](/docs/public-api) for what to do.

## Reading an issue

SkillIssues shows you what GitHub has about an issue, such as its title, labels, comment count and repository. The full text and discussion live on GitHub. Read them there before you start, and say on the issue that you're taking it. See [Getting started](/docs/getting-started).

Cards also say how the repository treats contributions, from its last 20 merged pull requests, once it has at least 5:

- **Merges outside contributions**: at least 3 of them came from people outside the project (not its owners, organisation members or collaborators). A project that merges outsiders' work is a good place to start.
- **Slow to merge**: its pull requests usually take 60 days or more to merge. Expect to wait for a review.

The issue page gives the numbers. They're read again each time the catalog re-reads the repository.
