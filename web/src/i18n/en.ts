/*
 * Every interface string on the site, in English: the landing page, the
 * header and footer, and the signed-in app. Another language is a file of
 * the same shape (Translation) listing the keys it translates; a key it
 * leaves out shows in English. The docs and the legal pages are not here:
 * they stay in English, and legal text changes only with a reviewed
 * translation.
 *
 * A message that takes values is a function of them, so a translation can
 * reorder the words around them.
 */
/** The same choice all day, a different one tomorrow. */
function dayPick(n: number): number {
  const d = new Date();
  return (d.getFullYear() * 372 + d.getMonth() * 31 + d.getDate()) % n;
}

export const en = {
  // ---- the site's header and footer ----
  "site.home": "SkillIssues home",
  "site.menu": "Menu",
  "site.close": "Close",
  "site.nav": "Main",
  "site.nav.howItWorks": "How it works",
  "site.nav.download": "Install desktop app",
  "site.nav.docs": "Docs",
  "site.nav.live": "SkillIssues Live",
  "site.nav.contribute": "Contribute",
  "logo.line": "You Have the Skills. We Have the Issues.",

  "footer.col.skillissues": "SkillIssues",
  "footer.col.guides": "Guides",
  "footer.col.community": "Community",
  "footer.col.support": "Support",
  "footer.col.legal": "Legal & Trust",
  "footer.about": "About us",
  "footer.browse": "Browse issues",
  "footer.desktop": "Desktop app",
  "footer.docs": "Documentation",
  "footer.live": "SkillIssues Live",
  "footer.changelog": "Changelog",
  "footer.gettingStarted": "Getting started",
  "footer.howItWorks": "How it works",
  "footer.findingIssues": "Finding issues",
  "footer.difficulty": "Difficulty ratings",
  "footer.themes": "Themes",
  "footer.publicApi": "Public API",
  "footer.contribute": "Contribute",
  "footer.contributionGuide": "Contribution guide",
  "footer.maintainers": "For maintainers",
  "footer.suggest": "Suggest a repository",
  "footer.discussions": "Discussions",
  "footer.leaderboard": "Leaderboard",
  "footer.sponsor": "Sponsor us",
  "footer.faq": "FAQ",
  "footer.contact": "Contact us",
  "footer.reportBug": "Report a bug",
  "footer.requestFeature": "Request a feature",
  "footer.privacy": "Privacy policy",
  "footer.terms": "Terms of service",
  "footer.security": "Security",
  "footer.licenses": "Licenses and attributions",
  "footer.sign": "There will always be more work to do. Start with one issue.",
  "footer.copyright": (p: { year: number }) => `© ${p.year} SkillIssues`,
  "footer.madeBy": "Made by and for the open-source community.",
  "footer.tools": "Display and source",
  "footer.source": "Source code on GitHub",

  // ---- light and dark, language ----
  "mode.toDark": "Switch to dark mode",
  "mode.toLight": "Switch to light mode",
  "mode.useDefault": "Use the default theme",
  "mode.useMine": (p: { name: string }) => `Use your theme, ${p.name}`,
  "language.choose": "Language",
  "language.more": "More languages are on the way.",

  // ---- the landing page ----
  "landing.hero.title": "Better software doesn’t happen on its own.",
  "landing.hero.body":
    "Across open source, millions of issues are waiting: bugs to fix, improvements to make, ideas to bring to life. That’s unfinished work in the software we all rely on, and the pile keeps growing.",
  "landing.us.title": "It’s a lot of issues. And there are a lot of us.",
  "landing.us.body1":
    "Every project has work waiting to be done: bugs to fix, improvements to make, and questions to answer. Across thousands of projects, those issues add up faster than any one person can tackle them.",
  "landing.us.body2":
    "But it doesn’t have to be one person. Big challenges move when people take useful steps together. Here, that can start with one issue you know how to solve.",
  "landing.made.lead": "So we made",
  "landing.made.title": "A global initiative to help tackle open source’s growing backlog.",
  "landing.made.body":
    "SkillIssues brings open issues from active GitHub projects into one searchable place. Find one that fits your language, skills, and experience—then head to GitHub and help make it better.",
  "landing.made.cta": "Explore open issues",
  "landing.dev.kicker": "For developers",
  "landing.dev.title": "Find an issue that feels like yours.",
  "landing.dev.body":
    "Maybe you’re learning a new language, looking for your first contribution, or ready to take on something more challenging. Choose what you’re interested in, explore issues by skill and difficulty, and find a place to start.",
  "landing.dev.cta": "Solve issues",
  "landing.docs.title": "A good place to start—and help along the way.",
  "landing.docs.body":
    "Every project does things a little differently. Our docs will help you get set up, understand the work, and find your way from “I could help with this” to opening a pull request.",
  "landing.maint.kicker": "For maintainers",
  "landing.maint.title": "Your project’s next contributor could be out there looking for you.",
  "landing.maint.body":
    "SkillIssues helps connect open issues with developers interested in your project’s languages and skills. So more people can find the work that needs doing—and your project can keep moving forward.",
  "landing.maint.cta": "Learn how to include your project",
  "landing.desk.title": "Keep the contribution moving.",
  "landing.desk.body1":
    "Found your issue? The desktop app helps you get into the work: pull the project, make your changes, push them, and keep track of your pull requests.",
  "landing.desk.body2": "Less time juggling steps. More time making something better.",
  "landing.desk.cta": "Download the desktop app",
  "landing.desk.aside": "Available for Windows, macOS and Linux.",
  "landing.effort.title": "Every contribution adds to the bigger effort.",
  "landing.effort.body":
    "We’re working on levels and a community leaderboard to celebrate the people helping move open source forward—and show what we can accomplish together.",
  "landing.trust.kicker": "Built with trust in mind",
  "landing.trust.title": "Know what’s happening with your account and code.",
  "landing.trust.body":
    "Contributing should feel good—and that includes knowing how your tools work. Our security docs explain how SkillIssues handles your account, your code, and the contribution process.",
  "landing.trust.cta": "Read the security docs",
  "landing.live.title": "And you can see how SkillIssues works, too.",
  "landing.live.body":
    "We wanted the product to be open about what’s happening behind the scenes. SkillIssues Live lets you watch activity move through the system and see the different parts working together.",
  "landing.live.cta": "Explore SkillIssues Live",
  "landing.end.title": "There will always be more work to do. Start with one issue.",
  "landing.end.body": "Find something you care about. Make a contribution. Help a project take its next step.",
  "landing.end.cta": "[Find an issue]",

  "live.label": (p: { closed: string; open: string }) =>
    `${p.closed} issues closed since we started counting; ${p.open} open issues.`,
  "live.loading": "Live count of closed and open issues, loading.",
  "live.closed": "issues closed",
  "live.open": "open issues and counting",

  // ---- written pages ----
  "page.draft": "Draft: this page is being reviewed and may change before it is final.",
  "page.notFound.title": "There’s no page here.",
  "page.notFound.body": "The link may be old, or mistyped. There are plenty of open issues, though.",
  "page.notFound.cta": "Explore open issues",
  "docs.nav": "Documentation",
  "docs.legal": "Legal & Trust",
  "docs.search": "Search the docs",
  "docs.searchNone": "No page mentions that.",
  "docs.updated": (p: { date: string }) => `Last updated ${p.date}.`,

  // ---- the app: header and menu ----
  "app.nav": "App",
  "app.nav.explore": "Explore",
  "app.nav.profile": "Profile",
  "app.nav.signIn": "Sign in",
  "app.menu.settings": "Settings",
  "app.menu.appearance": "Appearance",
  "app.menu.dark": "Dark",
  "app.menu.language": "Language",
  "app.menu.signOut": "Sign out",
  "app.menu.signingOut": "Signing out",
  "app.notFound.title": "There’s nothing here.",
  "app.notFound.body": "This part of the app doesn’t exist, or not yet.",
  "app.notFound.cta": "Back to Explore",

  // ---- the app: signing in ----
  "auth.signIn": "Sign in with GitHub",
  "auth.opening": "Opening GitHub",
  "auth.unavailable": "Sign-in isn’t available right now.",
  "auth.dismiss": "Dismiss",
  "auth.error.access_denied": "Sign-in was cancelled on GitHub.",
  "auth.error.invalid_callback": "That sign-in link was expired or already used. Start again.",
  "auth.error.oauth_failed": "GitHub didn’t confirm the sign-in. Start again.",
  "auth.error.sign_in_failed": "Signing you in failed on our side. Try again in a minute.",
  "auth.error.other": "Sign-in didn’t complete. Start again.",
  "theme.failed": (p: { reason: string }) => `Your theme couldn’t be applied, so you’re seeing the default: ${p.reason}`,

  // ---- the app: Explore ----
  "explore.title": "Find issues that you can solve",
  "explore.searchLabel": "Search:",
  "explore.searchPlaceholder": "Issue title, repository name, language, labels",
  "explore.clear": "clear",
  "explore.clearLabel": "Clear the search",
  "explore.filter": "filter",
  "explore.search": "Search",
  "explore.next": "Find your next issue",
  "explore.searchHint": "This search mixes every kind of match (repositories, titles and labels, languages, authors) in one list.",
  "explore.searchHintLink": "Search with / to see them by kind.",
  "explore.scope": "Showing issues from",
  "explore.scopeClear": "Show all repositories",
  "explore.difficulties": "Difficulty",
  "explore.results": (p: { term: string }) => `Results for “${p.term}”`,
  "explore.loading": "Loading issues…",
  "explore.empty": "No open issues match. Try fewer filters or another difficulty.",
  "explore.emptySearch": "Nothing matches that search. Try a repository name, a language or other words.",
  "explore.retry": "Try again",
  "explore.pager": "Pages",
  "explore.prev": "Previous page",
  "explore.nextPage": "Next page",
  "explore.page": (p: { n: number }) => `Page ${p.n}`,

  "difficulty.BEGINNER": "Beginner",
  "difficulty.INTERMEDIATE": "Intermediate",
  "difficulty.ADVANCED": "Advanced",
  "difficulty.UNCLASSIFIED": "Unranked",

  "filters.title": "Filters",
  "filters.labels": "labels",
  "filters.tech": "tech",
  "filters.topics": "topics",
  "filters.languages": "languages",
  "filters.comments": "comments",
  "filters.difficulty": "difficulty",
  "filters.sort": "sort by",
  "filters.help.labels": "GitHub labels on the issue, like “good first issue”. An issue with any one of your picks matches.",
  "filters.help.tech": "Languages the repository is written in, not only its main one.",
  "filters.help.topics": "Topics the repository lists on GitHub.",
  "filters.help.languages": "The repository’s main language.",
  "filters.help.comments": "How many comments the issue has. Few comments often means nobody has started on it.",
  "filters.help.difficulty": "Rated from the issue’s labels. Unranked issues have no difficulty label: their difficulty is unknown, not easy.",
  "filters.help.sort": "Shuffle gives every repository a fair turn. Easiest puts beginner issues first. Newest shows the most recently opened.",
  "filters.whatIs": (p: { name: string }) => `What is ${p.name}?`,
  "filters.any": "any",
  "filters.chosen": (p: { count: number }) => `${p.count} chosen`,
  "filters.find": (p: { name: string }) => `Find ${p.name}`,
  "filters.none": "Nothing matches.",
  "filters.loading": "Loading the choices…",
  "filters.min": "At least",
  "filters.max": "At most",
  "filters.clear": "Clear",
  "filters.apply": "Apply",
  "sort.shuffle": "Shuffle",
  "sort.easiest": "Easiest",
  "sort.newest": "Newest",

  // ---- the app: the search overlay ----
  "search.title": "Search",
  "search.close": "Close",
  "search.sub":
    "Looks for your words in four places at once: repository names, issue titles and labels, languages, and the people who opened issues. Results come as you type, each kind on its own. Open with / or Ctrl+K from anywhere.",
  "search.short": "Type at least two characters.",
  "search.searching": "Searching…",
  "search.all": (p: { term: string }) => `See all results for “${p.term}”`,
  "search.cat.Repository": "Repositories",
  "search.cat.Title": "Titles and labels",
  "search.cat.Language": "Languages",
  "search.cat.Author": "Authors",
  "app.nav.search": "Search",
  "app.nav.contribute": "Contributions",
  "app.nav.unread": (p: { count: number }) => (p.count === 1 ? "1 unread notification" : `${p.count} unread notifications`),
  "app.menu.profile": "Profile",
  "app.menu.saved": "Saved issues",
  "app.menu.contributions": "My contributions",

  // ---- the app: an issue ----
  "issue.updated": (p: { ago: string }) => `updated ${p.ago}`,
  "issue.comments": (p: { count: number }) => (p.count === 1 ? "1 comment" : `${p.count} comments`),
  "issue.moreLabels": (p: { count: number }) => `+${p.count}`,
  "issue.view": "View Issue",
  "issue.moreRepo": (p: { repo: string }) => `More issues from ${p.repo}`,
  "issue.moreOwner": (p: { owner: string }) => `More issues from ${p.owner}`,
  "issue.onGitHub": "On GitHub",
  "issue.save": "Save issue",
  "issue.unsave": "Remove from saved",
  "issue.saveSignIn": "Sign in to save issues",
  "issue.saveFailed": (p: { reason: string }) => `Couldn’t change saved issues: ${p.reason}`,
  "issue.back": "Back to Explore",
  "issue.openGitHub": "Open on GitHub",
  "issue.desktop": "Work on it in the desktop app",
  "desktop.notOpened": "The desktop app didn’t open. Don’t have it yet?",
  "desktop.download": "Download the desktop app",
  "desktop.github": "Or carry on, on GitHub",
  "issue.saved": "Saved",
  "issue.saveLong": "Save",
  "issue.closed": "This issue is closed on GitHub.",
  "issue.closedNow": (p: { ago: string }) =>
    `This issue was closed on GitHub ${p.ago}. It leaves SkillIssues at the next check, so pick another to work on.`,
  "issue.loading": "Loading the issue…",
  "issue.notFound": "This issue isn’t in the catalog. It may have been closed for a while and removed.",
  "issue.body": "Description",
  "issue.bodyLoading": "Loading the description from GitHub…",
  "issue.bodyEmpty": "The issue has no description.",
  "issue.bodyQueued": "GitHub isn’t answering us for this one right now. Open it on GitHub to read it.",
  "issue.bodyFailed": (p: { reason: string }) => `The description couldn’t be loaded: ${p.reason}`,
  "issue.fromCache": "GitHub limited this browser’s requests, so this copy comes from our cache and may be a little behind.",
  "issue.repository": "Repository",
  "issue.stars": (p: { count: string }) => `${p.count} stars`,
  "issue.license": (p: { license: string }) => `Licence: ${p.license}`,
  "issue.author": (p: { login: string }) => `Opened by ${p.login}`,
  "issue.opened": (p: { date: string }) => `Opened ${p.date}`,

  // ---- the app: signed-in pages ----
  "signin.title": "Sign in to see this",
  "signin.body": "Your profile, contributions and saved issues need a GitHub sign-in. SkillIssues reads only your public profile.",

  // ---- the app: profile ----
  // Greetings by the reader's own clock; one is picked per day.
  "profile.greet.morning": (p: { name: string }) =>
    [`Good morning, ${p.name}`, `Morning, ${p.name}. Coffee first?`, `Rise and ship, ${p.name}`][dayPick(3)]!,
  "profile.greet.afternoon": (p: { name: string }) =>
    [`Good afternoon, ${p.name}`, `Afternoon, ${p.name}. Got an issue in you?`, `Hey ${p.name}, back at it`][dayPick(3)]!,
  "profile.greet.evening": (p: { name: string }) =>
    [`Good evening, ${p.name}`, `Evening, ${p.name}. One more fix?`, `Winding down, ${p.name}?`][dayPick(3)]!,
  "profile.greet.night": (p: { name: string }) =>
    [`Burning the midnight oil, ${p.name}?`, `Up late, ${p.name}?`, `Hello, night owl`][dayPick(3)]!,
  "profile.joined": (p: { date: string }) => `joined on ${p.date}`,
  "profile.noBio": "No bio yet.",
  "profile.editBio": "Add one in Settings",
  "profile.noLanguages": "Choose the languages you like in Settings.",
  "profile.languages": "Your languages",
  "profile.activity": "Activity",
  "profile.prs": "Pull Requests",
  "profile.resolved": "Resolved issues",
  "profile.saved": "Saved issues",
  "profile.allSaved": "All saved issues",
  "profile.allContributions": "All contributions",
  "profile.noPRs": "No public pull requests yet. Find an issue on Explore and open your first.",
  "profile.noResolved": "Nothing merged yet. Your first merged pull request will show here.",
  "profile.noSaved": "Nothing saved yet. Press the bookmark on any issue to keep it here.",
  "profile.loading": "Loading…",

  // ---- the app: contributions ----
  "contrib.title": "Your contributions",
  "contrib.lede": "Counted from your public pull requests on GitHub, read by your browser.",
  "contrib.stat.total": "Contributions",
  "contrib.stat.total.help": "Every public pull request you’ve opened on GitHub.",
  "contrib.stat.resolved": "issues resolved",
  "contrib.stat.resolved.help": "Your merged pull requests: each one landed a fix or change.",
  "contrib.stat.repos": "repositories worked on",
  "contrib.stat.repos.help": "Different repositories among your merged pull requests.",
  "contrib.stat.reposPartial": (p: { count: number }) => `Counted from your latest ${p.count} merged pull requests.`,
  "contrib.graph": "Contributions",
  "contrib.graph.label": (p: { count: number }) =>
    p.count === 1 ? "1 pull request opened in the last year" : `${p.count} pull requests opened in the last year`,
  "contrib.graph.partial": "Showing your latest 300 pull requests of the year.",
  "contrib.graph.day": (p: { date: string; count: number }) =>
    p.count === 1 ? `1 pull request on ${p.date}` : `${p.count} pull requests on ${p.date}`,
  "contrib.graph.less": "Less",
  "contrib.graph.more": "More",
  "contrib.updated": (p: { ago: string }) => `Read from GitHub ${p.ago}.`,
  "contrib.refresh": "Read again",
  "contrib.limited": (p: { when: string }) => `GitHub limits how often a browser can search without signing in. Try again ${p.when}.`,
  "contrib.limitedSoon": "GitHub limits how often a browser can search without signing in. Try again in a minute.",
  "contrib.failed": (p: { reason: string }) => `Your contributions couldn’t be read from GitHub: ${p.reason}`,
  "contrib.merged": "merged",
  "contrib.open": "open",
  "contrib.closed": "closed",

  "achieve.title": "Achievements",
  "achieve.firstPR": "First pull request",
  "achieve.firstMerge": "First merge",
  "achieve.tenMerged": "Ten merged",
  "achieve.fiftyMerged": "Fifty merged",
  "achieve.fiveRepos": "Five repositories",
  "achieve.busyMonth": "Ten in a month",
  "achieve.done": "Unlocked",
  "achieve.progress": (p: { have: number; need: number }) => `${p.have} of ${p.need}`,

  // ---- the app: saved ----
  "saved.title": "Saved issues",
  "saved.updates": "Updates",
  "saved.closedOn": (p: { title: string }) => `“${p.title}” was closed on GitHub.`,
  "saved.markRead": "Mark as read",
  "saved.markAll": "Mark all as read",
  "saved.empty": "Nothing saved yet. Press the bookmark on any issue to keep it here.",
  "saved.explore": "Explore issues",
  "issue.closedChip": "Closed",

  // ---- the app: settings ----
  "settings.title": "Settings",
  "settings.account": "Account",
  "settings.signedInAs": (p: { name: string }) => `Signed in as ${p.name}`,
  "settings.signedOut": "Sign in to save issues, keep a profile and use themes.",
  "settings.everywhere": "Sign out everywhere",
  "settings.everywhere.body": "Ends every session on every device, this one included.",
  "settings.everywhere.pending": "Signing out",
  "settings.delete": "Delete account",
  "settings.delete.body":
    "This deletes your profile, preferences, saved issues, themes, notifications and every session, straight away. It can’t be undone.",
  "settings.delete.confirm": "Delete my account",
  "settings.delete.pending": "Deleting",
  "settings.delete.keep": "Keep it",
  "settings.deleted.before": "Your account and everything tied to it is deleted. You can also remove the sign-in app on",
  "settings.deleted.link": "GitHub’s authorised apps page",
  "settings.support": "Support",
  "settings.support.docs": "Documentation",
  "settings.support.faq": "FAQ",
  "settings.support.contact": "Contact us",
  "settings.support.bug": "Report a bug",
  "settings.support.privacy": "Privacy policy",
  "settings.support.terms": "Terms of service",
  "settings.support.security": "Security",
};

export type Messages = typeof en;

/** Another language: any of the English keys, each with the same shape. */
export type Translation = Partial<{ [K in keyof Messages]: Messages[K] }>;
