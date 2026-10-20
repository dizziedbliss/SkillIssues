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

  // ---- the app: the catalog (until its designed version) ----
  "catalog.notice": "You’re seeing a simple version of the catalog while the full app is being designed.",
  "catalog.searchLabel": "Search issues",
  "catalog.searchPlaceholder": "Search by repository, title, language or author",
  "catalog.search": "Search",
  "catalog.clear": "Clear search",
  "catalog.sort": "Sort",
  "catalog.sort.shuffle": "Shuffled",
  "catalog.sort.easiest": "Easiest first",
  "catalog.sort.newest": "Newest",
  "catalog.difficulty": "Difficulty",
  "catalog.difficulty.any": "Any difficulty",
  "catalog.difficulty.BEGINNER": "Beginner",
  "catalog.difficulty.INTERMEDIATE": "Intermediate",
  "catalog.difficulty.ADVANCED": "Advanced",
  "catalog.difficulty.UNCLASSIFIED": "Unranked",
  "catalog.language": "Language",
  "catalog.language.any": "Any language",
  "catalog.loading": "Loading issues…",
  "catalog.empty": "No open issues match. Try another language or difficulty.",
  "catalog.comments": (p: { count: number }) => (p.count === 1 ? "1 comment" : `${p.count} comments`),
  "catalog.more": "Show more issues",
  "catalog.loadingMore": "Loading",

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
