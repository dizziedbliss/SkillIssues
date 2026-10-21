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
| For you | The default. Issues that suit you, look likely to get merged and have waited, one per repository, drawn differently for each person. |
| Easiest | Rated issues first, easiest to hardest, then the most recently updated. Unranked issues come last. |
| Newest | The most recently opened issues first. |

**For you** is how SkillIssues tries to get the whole backlog done, not just the most popular corners of it. It works in three steps:

1. **Candidates.** It walks the catalog's repositories in a random order, taking each one's best open issue that passes your filters, so a project with ten thousand issues gets the same one slot as one with three. Each person's walk starts somewhere different, and it starts somewhere new each day.
2. **A score for each issue: Fit × Doable × Need.**
   - **Fit** (signed in): the repository's language is one of yours, or it uses one; the difficulty is your level or the one you're aiming for; it touches your interests; you've merged in that language, or in that repository, before.
   - **Doable:** it has a difficulty label, the discussion is short, it's been active recently, the repository has a contributing guide, and it merges outsiders' pull requests without long waits.
   - **Need:** it has been waiting a while, and few people have been shown it lately.
3. **A weighted draw.** Each page is drawn at random, weighted by score: a better match is more likely to come first, but two people don't get the same list, so nobody piles onto the same issue. The draw is fixed for the day, so going back a page shows what you saw.

Each card says why it's there, in a line under the repository ("Why: your language, merges outsiders' pull requests"). Signed out, Fit doesn't apply and everyone shares one draw a day.

If you're signed in, Explore starts with your profile's languages and level as filters, shown under the search box. **Show every language** removes the languages; the difficulty buttons change the level. With Easiest, your preferred languages also come first.

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

When you browse For you, and especially when you combine filters that few repositories match, the catalog may stop looking after a couple of seconds and give you a short page, or even an empty one, along with a way to carry on from where it stopped. This doesn't mean you've reached the end. Only a response with no way to continue means the end. The website keeps reading a few more pages to fill each page before it tells you nothing matches.

## Pages

The numbers under the list are the pages you've already seen, so you can go back to any of them, plus the next one when there is one. For you has no fixed page 7 to jump to, so the numbers grow as you go. If you're using the API directly, see [Public API](/docs/public-api) for what to do.

## Reading an issue

SkillIssues shows you what GitHub has about an issue, such as its title, labels, comment count and repository. The full text and discussion live on GitHub. Read them there before you start, and say on the issue that you're taking it. See [Getting started](/docs/getting-started).

Cards also say how the repository treats contributions, from its last 20 merged pull requests, once it has at least 5:

- **Merges outside contributions**: at least 3 of them came from people outside the project (not its owners, organisation members or collaborators). A project that merges outsiders' work is a good place to start.
- **Slow to merge**: its pull requests usually take 60 days or more to merge. Expect to wait for a review.

The issue page gives the numbers. They're read again each time the catalog re-reads the repository.
