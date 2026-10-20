import { external } from "../lib/links";

/*
 * The site's written pages. Every claim here describes what the code does
 * today; when the code changes, the page changes with it. Pages marked draft
 * are waiting for the owner's review and say so on the page.
 */

export type Block =
  | string
  | { h: string }
  | { list: string[] }
  | { link: { label: string; to?: string; href?: string } };

export interface Page {
  path: string;
  title: string;
  lede: string;
  draft?: boolean;
  body: Block[];
}

const releases = { label: "Desktop releases on GitHub", href: external.releases };

export const pages: Page[] = [
  {
    path: "/how-it-works",
    title: "How SkillIssues works",
    lede: "Open issues from active GitHub projects, gathered in one place and kept current, so you can find one you can solve.",
    body: [
      { h: "Where the issues come from" },
      "We list repositories that are public, active and have a minimum number of stars (25 today). The star floor isn’t a measure of quality: it keeps anyone from flooding the catalog with throwaway projects.",
      "New repositories are found two ways: a monthly sweep of GitHub’s repository search, and the public GitHub event stream, where any repository with fresh issue activity is checked and listed if it qualifies.",
      { h: "How it stays current" },
      "Every hour we read GH Archive, the public record of GitHub events. An issue opened, labelled or closed on GitHub shows the change here within about two hours. A slower pass re-reads every repository directly from GitHub each month, to repair anything the event stream missed.",
      "We only record an issue as closed when GitHub says it closed. Issues you saved are checked directly, more often.",
      { h: "How issues are ordered" },
      "By default the catalog is shuffled: every repository gets one slot, showing its easiest open issue, in an order redrawn each day. A project with ten thousand issues gets the same single slot as one with three. The goal is the whole backlog, not the biggest projects.",
      "You can sort by easiest or newest instead, and filter by labels, languages, the technologies a repository uses, its topics, difficulty and how much discussion an issue has.",
      { h: "Difficulty" },
      "Difficulty comes from the labels maintainers put on issues, like “good first issue” or “difficulty/hard”. An issue with no such label is unrated. We never guess that an unlabelled issue is easy.",
      { link: { label: "Explore open issues", to: "/app" } },
    ],
  },
  {
    path: "/download",
    title: "The desktop app",
    lede: "Pull the project, make your change, push it and follow your pull request, without juggling terminals and tabs.",
    body: [
      "The desktop app runs on Windows, macOS and Linux. It forks and clones the project for you, opens it in your editor, pushes your branch and opens the pull request.",
      "It signs in to GitHub with a token that stays on your computer. Our servers never see it.",
      "The app is being rebuilt alongside this website. Builds are published on GitHub as they are released.",
      { link: releases },
    ],
  },
  {
    path: "/docs",
    title: "Documentation",
    lede: "Everything you need to go from finding an issue to a merged pull request.",
    body: [
      { link: { label: "Getting started", to: "/docs/getting-started" } },
      { link: { label: "How SkillIssues works", to: "/how-it-works" } },
      { link: { label: "Contribution guide", to: "/docs/contributing" } },
      { link: { label: "For maintainers", to: "/maintainers" } },
      { link: { label: "Security", to: "/security" } },
      { link: { label: "FAQ", to: "/faq" } },
    ],
  },
  {
    path: "/docs/getting-started",
    title: "Getting started",
    lede: "Four steps from here to your first pull request.",
    body: [
      { h: "1. Find an issue" },
      "Browse the catalog, or filter it by the languages you know and a difficulty you’re comfortable with. Beginner issues are ones maintainers marked as a good place to start. Read the issue and its comments on GitHub before you begin: someone may already be on it.",
      { h: "2. Say you’re taking it" },
      "Leave a short comment on the issue saying you’d like to work on it. Maintainers often reply with pointers, and it saves two people doing the same work.",
      { h: "3. Fork, clone and fix" },
      "Fork the repository on GitHub, clone your fork, and create a branch. Read the project’s CONTRIBUTING file: it says how to run the tests and how the maintainers like changes made. The desktop app does the fork, clone and branch for you.",
      { h: "4. Open a pull request" },
      "Push your branch and open a pull request that links the issue (write “Fixes #123” in the description). Answer review comments, and don’t worry about needing a few rounds: that’s normal.",
      { link: { label: "Find your first issue", to: "/app" } },
    ],
  },
  {
    path: "/docs/contributing",
    title: "Contribution guide",
    lede: "How to be the contributor maintainers are glad to see.",
    body: [
      { list: [
        "Pick an issue that’s still open and unassigned, and comment before you start.",
        "Keep the change to what the issue asks. A small, focused pull request is reviewed faster.",
        "Follow the project’s own rules: its CONTRIBUTING file, code style and commit format.",
        "Run the tests before you push, and add one for the bug you fixed.",
        "Describe what you changed and why, and link the issue.",
        "Be patient with reviews. Maintainers are often volunteers.",
      ] },
      { h: "Contributing to SkillIssues itself" },
      "SkillIssues is open source too. The repository’s CONTRIBUTING.md explains how to run it locally and how changes are made.",
      { link: { label: "SkillIssues on GitHub", href: external.repo } },
    ],
  },
  {
    path: "/contribute",
    title: "Contribute",
    lede: "Two ways to help: solve issues across open source, or help build SkillIssues.",
    body: [
      { h: "Solve issues" },
      "Find an issue in a language you know and send a fix. Every closed issue makes the pile smaller.",
      { link: { label: "Explore open issues", to: "/app" } },
      { h: "Build SkillIssues" },
      "The code is MIT licensed. Bug reports, feature ideas and pull requests are all welcome.",
      { link: { label: "SkillIssues on GitHub", href: external.repo } },
      { link: { label: "Report a bug", href: external.reportBug } },
    ],
  },
  {
    path: "/maintainers",
    title: "For maintainers",
    lede: "How your project gets listed, and how to help contributors find the right issues.",
    body: [
      { h: "Getting listed" },
      "There’s nothing to sign up for. A public repository is listed once it has at least 25 stars, has been pushed to in the last two years, has a main language and isn’t archived. We find new repositories from GitHub’s public event stream, so issue activity on a qualifying repository is enough. Check your repository on the home page.",
      { h: "Help contributors pick well" },
      { list: [
        "Label easy issues “good first issue” or “difficulty/easy”, and harder ones “difficulty/medium” or “difficulty/hard”. We read these labels to rate difficulty; unlabelled issues show as unrated.",
        "Write the issue so a newcomer can start: what’s wrong, where the code lives, and how you’d check a fix.",
        "Keep a CONTRIBUTING file with how to run the project and its tests.",
      ] },
      { h: "Not listed, or want out?" },
      "If your repository should be listed and isn’t, or you’d rather it weren’t listed at all, open an issue and we’ll sort it out.",
      { link: { label: "Open an issue", href: `${external.repo}/issues/new` } },
    ],
  },
  {
    path: "/about",
    title: "About SkillIssues",
    lede: "A global initiative to help tackle open source’s growing backlog.",
    body: [
      "Open source has millions of open issues: bugs, improvements and questions that keep piling up faster than any group of maintainers can work through them. Plenty of developers want to help but don’t know where to start.",
      "SkillIssues puts those issues in one place, keeps them current, and helps you find one that fits what you know. Then you go to GitHub and fix it.",
      "It’s built in the open, by and for the open-source community.",
      { link: { label: "SkillIssues on GitHub", href: external.repo } },
    ],
  },
  {
    path: "/faq",
    title: "Frequently asked questions",
    lede: "Short answers to what people ask most.",
    body: [
      { h: "Is it free?" },
      "Yes.",
      { h: "Do I need an account?" },
      "No, to browse. Signing in with GitHub lets you save issues and set the languages and level you’re looking for.",
      { h: "What can SkillIssues do with my GitHub account?" },
      "Nothing. Sign-in only reads who you are (your GitHub id, username and avatar), and we throw the token away. We can’t see private repositories or act on your behalf.",
      { h: "Why isn’t a repository I know listed?" },
      "It may be under the star floor, inactive for two years, archived or without a main language. The lookup on the home page tells you which.",
      { h: "Why does an issue say “unrated”?" },
      "Its maintainers haven’t labelled its difficulty. We don’t guess.",
      { h: "How fresh is the data?" },
      "Changes on GitHub usually show here within two hours.",
    ],
  },
  {
    path: "/contact",
    title: "Contact us",
    lede: "The quickest way to reach us is on GitHub.",
    body: [
      { list: [
        "Something broken? Report a bug.",
        "An idea? Request a feature, or talk it over in Discussions.",
        "A question? Ask it in Discussions.",
        "A security problem? Don’t post it publicly: report it privately.",
      ] },
      { link: { label: "Report a bug", href: external.reportBug } },
      { link: { label: "Request a feature", href: external.requestFeature } },
      { link: { label: "Ask a question in Discussions", href: external.askQuestion } },
      { link: { label: "Report a vulnerability privately", href: external.reportVulnerability } },
      { link: { label: "Security", to: "/security" } },
    ],
  },
  {
    path: "/security",
    title: "Security",
    lede: "How SkillIssues handles your account, your code and the contribution process.",
    body: [
      { h: "Your account" },
      "You sign in with GitHub through an app that asks for no permissions. We read your numeric GitHub id, username and avatar, then discard the token. Your account is tied to that id, never to your username, which GitHub lets someone else take.",
      "A session lasts until you sign out, it expires, or you sign out everywhere. It lives in a secure, HTTP-only cookie.",
      { h: "Your code" },
      "Anything that writes to GitHub (forking, pushing, opening pull requests) happens in the desktop app, with a token that stays on your computer. It never reaches our servers.",
      { h: "Our servers" },
      "Every release is built by our public CI, signed, and verified on the server before it runs. Secrets are stored encrypted, and backups are encrypted with a key that isn’t on the server.",
      { h: "Themes" },
      "The app can be restyled with themes, including ones other people share. A theme’s stylesheet is checked against a strict allowlist and can’t load anything from elsewhere, and the browser blocks it from trying.",
      { h: "Reporting a vulnerability" },
      "Please don’t open a public issue. Use GitHub’s private vulnerability reporting, which only the maintainers can see, and tell us what’s affected, how to reproduce it, and whether you’ve seen it exploited.",
      { link: { label: "Report a vulnerability privately", href: external.reportVulnerability } },
    ],
  },
  {
    path: "/privacy",
    title: "Privacy policy",
    lede: "What we collect, why, and what we don’t.",
    draft: true,
    body: [
      { h: "If you only browse" },
      "We don’t set cookies or track you. Our network provider (Cloudflare) and our server see your IP address to deliver pages and to limit how many requests one address can make. The rate-limit counter expires within minutes; our web server’s access logs, which include IP addresses, rotate out as they fill.",
      { h: "If you sign in" },
      { list: [
        "From GitHub: your numeric GitHub id, username and avatar. We do not keep your GitHub token.",
        "From you: the preferences you set (languages, level, interests), the issues you save, and any themes you create.",
        "A session cookie, so you stay signed in.",
      ] },
      { h: "What we don’t do" },
      "We don’t sell data, show ads, or use third-party analytics. Issue data shown on SkillIssues is public GitHub data.",
      { h: "Deleting your data" },
      "Ask us on GitHub and we’ll delete your account and everything tied to it.",
    ],
  },
  {
    path: "/terms",
    title: "Terms of service",
    lede: "The short version: use SkillIssues to find and solve open-source issues, and be decent.",
    draft: true,
    body: [
      { list: [
        "SkillIssues is provided as is, for free, with no guarantee it will always be available or accurate.",
        "Issues, repositories and their content belong to their authors on GitHub. Follow each project’s own license and code of conduct when you contribute.",
        "Don’t abuse the service: no scraping beyond the published API limits, no attempts to break in, no using themes to mislead other users.",
        "Themes you share are yours; by sharing one you let others apply it.",
        "We may change these terms; the page will show when.",
      ] },
    ],
  },
  {
    path: "/licenses",
    title: "Licenses and attributions",
    lede: "What SkillIssues is built from.",
    body: [
      { h: "SkillIssues" },
      "MIT License.",
      { link: { label: "Source code", href: external.repo } },
      { h: "Data" },
      { list: [
        "Issue and repository data from GitHub’s public API.",
        "Hourly public events from GH Archive (gharchive.org).",
        "Initial repository metadata from the Kaggle dataset “GitHub repository metadata with 5 stars”.",
      ] },
      { h: "Typefaces" },
      { list: [
        "Instrument Serif, by Rodrigo Fuenzalida and Jordan Egstad, SIL Open Font License 1.1.",
        "JetBrains Mono, by JetBrains, SIL Open Font License 1.1.",
        "Krona One, by Yvonne Schüttler, SIL Open Font License 1.1.",
      ] },
    ],
  },
  {
    path: "/leaderboard",
    title: "Leaderboard",
    lede: "Coming soon: levels and a community leaderboard for the people closing issues across open source.",
    body: [
      "We’re building it so that a merged pull request counts the moment GitHub shows it, without us ever holding your GitHub token.",
      { link: { label: "Find an issue meanwhile", to: "/app" } },
    ],
  },
  {
    path: "/sponsor",
    title: "Sponsor SkillIssues",
    lede: "SkillIssues is free and runs on one small server.",
    body: [
      "Sponsorship isn’t set up yet. Until it is, the best support is solving an issue, or telling a friend who’s looking for one.",
      { link: { label: "Explore open issues", to: "/app" } },
    ],
  },
];
