---
title: SkillIssues Live
slug: /docs/skillissues-live
section: SkillIssues
summary: A public, real-time view of data moving through SkillIssues, and what it does and doesn't show.
draft: false
updated: 2026-10-20
---

SkillIssues Live is a public visualizer at [flow.dizziedbliss.lol](https://flow.dizziedbliss.lol). It shows data moving through the system as it happens: requests arriving, the database answering, the crawler reading GitHub, and the cache doing its work.

It is a separate site from SkillIssues itself. Opening it doesn't sign you in or affect your account.

## What you see

Each event is one hop between two parts of the system, for example a request reaching the API, a database query, or a page of issues read from GitHub. An event can carry:

- when it happened and how long it took;
- the route that was called, such as `GET /issues/{id}`;
- the kind of database operation and the table it touched, but never the query or its values;
- the name of the GitHub operation and the status it returned;
- for a crawled page of issues, the repository and how many issues arrived, plus the title of the first one. Issue titles are public GitHub data.

For parts of the system the visualizer can't instrument directly, it shows ambient network traffic instead.

## What you never see

- **Anything about signed-in people.** When a hop belongs to a signed-in user, its label is replaced before it leaves our servers with random symbols of random length. The symbols have no relationship to the original text or its size.
- Addresses, tokens, request ids, user ids, URL paths or database values. None of these exist in an event.

## Limits

- The stream is capped at a number of labelled events per second. Events beyond the cap still count toward each link's activity line, they just don't get a label.
- If nothing is listening, or our message queue is down, events are dropped. This never slows the real system down.
- There is a maximum number of simultaneous viewers. Past it, a new viewer is turned away until someone leaves.

## The data behind it

The visualizer reads a public stream of these events from SkillIssues. Issue titles in it come from strangers on GitHub, so anything that displays them should show them as plain text.

The same live counts, such as open issues and issues arriving today, are available from the [public API](/docs/public-api) at `GET /api/stats`.
