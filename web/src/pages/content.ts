import { external } from "../lib/links";

/*
 * The legal pages: privacy, terms, security and licenses. Every other written
 * page is Markdown in content/docs (docs.ts). Every claim here describes what
 * the code does today; when the code changes, the page changes with it. Pages
 * marked draft are waiting for the owner's review and say so on the page.
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

export const pages: Page[] = [
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
    body: [
      "Last updated 21 October 2026.",
      { h: "Who is responsible" },
      "SkillIssues is run by @dizziedbliss on GitHub, an individual based in Bengaluru, Karnataka, India. For the purposes of India’s Digital Personal Data Protection Act, 2023 and similar laws such as the GDPR, we are the data fiduciary (controller) of the personal data described here. Privacy contact and grievance contact: privacy@dizziedbliss.lol.",
      { h: "If you only browse" },
      "You don’t need an account to browse. We don’t ask for your name or email, we don’t set cookies for browsing, and we don’t use third-party analytics or advertising.",
      "Your browser keeps a few of your choices in its own storage: light or dark, your language, your theme and the themes you used recently, and for half an hour a copy of the public pull request numbers your profile shows. They stay on your device and never reach us; clearing your browser’s site data removes them.",
      "To deliver pages and to stop one address using up the service, our infrastructure sees your IP address:",
      { list: [
        "Vercel serves the site’s pages and handles your request for them first.",
        "Cloudflare, our network provider, sits in front of the API the pages call.",
        "Our API server keeps access logs that include IP addresses. They rotate by size, so each log is deleted as newer ones replace it, usually within a few days.",
        "Our rate limiter keeps a short-lived counter per address (or per signed-in account). The counter expires within the rate-limit window, which is an hour by default.",
      ] },
      "The issues and repositories you see are public GitHub data.",
      { h: "If you sign in" },
      "You sign in with GitHub through an app that asks for no permissions. From GitHub we keep:",
      { list: ["your numeric GitHub id;", "your GitHub username;", "the link to your avatar."] },
      "We do not keep your GitHub token. We use it once to read those three items and then discard it. From you we keep:",
      { list: [
        "the profile and preferences you set, such as your bio, experience level, target level, interests and preferred languages;",
        "the issues you save;",
        "the themes you create, and which one is applied to your account;",
        "notifications about saved issues that close, which the system records for you;",
        "reports you make: what you reported (an issue, a repository or a shared theme), the reason you chose and any note you add.",
      ] },
      "To keep you signed in we keep a session, stored as a hash, while your browser holds a secure, HTTP-only cookie. A second, short-lived cookie ties a sign-in to the browser that began it. A session lasts for 7 days by default, or until you sign out or sign out everywhere.",
      { h: "The desktop app" },
      "The desktop app uses a GitHub token that stays on your computer. It never reaches our servers. The desktop app sends no usage data or crash reports to us, and it doesn’t check for updates on its own.",
      { h: "How we use it, and why" },
      { list: [
        "Signing you in and keeping your session: your consent, and providing the service you asked for.",
        "Showing the catalog ordered by your languages; keeping your saved issues, preferences and themes; checking that a saved issue is still open: providing the service you asked for.",
        "Rate limiting, logs and abuse prevention: legitimate interests (keeping the service safe and available).",
        "Responding to your requests and reports: legitimate interests, and legal obligations.",
        "Reviewing reports of issues, repositories and themes, and leaving reported content out of lists until it’s reviewed: legitimate interests (keeping the catalog safe and useful). Only the people who run SkillIssues see reports; they’re never shown to the person or project you reported.",
      ] },
      "We don’t sell your data, show ads, build advertising profiles or share your data with third parties for their own use.",
      { h: "Who else handles your data" },
      { list: [
        "GitHub provides sign-in and the public issue data.",
        "Vercel delivers the site’s pages and sees visitor IP addresses. It operates globally.",
        "Cloudflare sits in front of the API and sees visitor IP addresses. It operates globally.",
        "OVH hosts the server and database, in Sydney, Australia.",
      ] },
      "These providers process data only to run the service for us. We may also disclose data if the law requires it.",
      { h: "Themes you share" },
      "A theme you mark public can be opened by anyone who has its link. It shows the theme file and the username you signed in with. A private theme is visible only to you.",
      { h: "SkillIssues Live" },
      "The public live visualizer shows system activity. Anything tied to a signed-in person is replaced with random symbols before it leaves our servers, and no event contains an address, token or user id.",
      { h: "Where data is kept and international transfers" },
      "Data is stored in a database on a server in Sydney, Australia. Because Vercel, Cloudflare and GitHub operate globally, and you may be anywhere in the world, your data can be processed in other countries. Production secrets and backups are encrypted, and backups are encrypted with a key that isn’t kept on the server.",
      { h: "How long we keep it" },
      { list: [
        "Account, profile, preferences, saved issues, themes, notifications and reports: until you delete your account.",
        "Sessions: 7 days by default, or until you sign out.",
        "Access logs, which include IP addresses: until newer logs replace them (they rotate by size), usually within a few days.",
        "Rate-limit counters: within the rate-limit window, an hour by default.",
        "Encrypted backups: 14 days, then deleted. Backups are kept on the server, and your data can remain in one for that time after you delete it from the live database.",
      ] },
      { h: "Your rights" },
      "Depending on where you live, you can:",
      { list: [
        "ask what data we hold about you and get a copy;",
        "correct it, or ask us to delete it;",
        "withdraw consent (for example, by deleting your account);",
        "object to or ask us to limit some processing;",
        "nominate someone to exercise your rights if you can’t (a right under India’s DPDP Act);",
        "make a complaint to us at privacy@dizziedbliss.lol, and, if you are not satisfied, to your local data protection authority (in India, the Data Protection Board).",
      ] },
      "We aim to reply within 30 days.",
      { h: "Deleting your data" },
      "Sign in, open the app and choose Delete account: it happens straight away. If you can’t sign in, email privacy@dizziedbliss.lol with your GitHub username and we will delete it; we may ask you to confirm the request from that GitHub account first, for example with a comment on GitHub. Please don’t post a deletion request in a public issue, since that publishes your username and request. Deleting your account removes your profile, preferences, saved issues, themes, sessions, notifications and reports. You can also remove the sign-in app from your authorised applications in GitHub’s settings.",
      { link: { label: "Open the app", to: "/app/settings/account" } },
      { h: "Security and breaches" },
      "We protect your data as described on the Security page. If a breach affects your personal data, we will tell the people affected and the authorities as the law requires.",
      { h: "Children" },
      "SkillIssues is for people aged 18 and over. We don’t knowingly collect data from anyone under 18. If you believe a child has an account, email privacy@dizziedbliss.lol and we will delete it.",
      { h: "Changes" },
      "We may update this policy. For material changes we will put a notice on the site before they take effect. The date at the top of the page shows when it last changed.",
      { h: "Contact" },
      "Privacy questions, requests and complaints: privacy@dizziedbliss.lol. For a security problem, see Security.",
      { link: { label: "Security", to: "/security" } },
      { link: { label: "Terms of service", to: "/terms" } },
    ],
  },
  {
    path: "/terms",
    title: "Terms of service",
    lede: "The short version: use SkillIssues to find and solve open-source issues, and be decent about it. It is a free hobby project run by one person, provided as it is.",
    body: [
      "Last updated 20 October 2026.",
      { h: "Who runs SkillIssues" },
      "SkillIssues is run by @dizziedbliss on GitHub, an individual based in Bengaluru, Karnataka, India (“we”, “us”). It is not run by a company. You can reach us at privacy@dizziedbliss.lol.",
      { h: "The service" },
      "SkillIssues is free. It is provided as it is and as available, with no promise that it will always be online, accurate or complete. We may change it, pause it or stop it at any time.",
      "The issue and repository data comes from GitHub and can be out of date, wrong or missing. Check an issue on GitHub before you work on it. We don’t promise that an issue is still open, that a project suits you, that a maintainer will reply, or that your contribution will be accepted.",
      { h: "Who can use it" },
      "You must be 18 or older. By using SkillIssues you confirm that you are, and that you will follow these terms and the Privacy policy.",
      { h: "Your account" },
      { list: [
        "You sign in with your GitHub account. Keep it secure.",
        "You are responsible for what is done through your account.",
        "You can delete your account at any time. See the Privacy policy for how.",
      ] },
      { h: "The desktop app" },
      "Actions that write to GitHub (forking, pushing, opening pull requests) happen in the desktop app, on your computer, with your own GitHub token. You are responsible for what you push and for following GitHub’s terms. We can’t act on your GitHub account from our servers and we are not responsible for what you do with it.",
      { h: "Other people’s work" },
      "Issues, repositories and their content belong to their authors on GitHub. SkillIssues only links to them. When you contribute, follow each project’s own license, contribution rules and code of conduct. SkillIssues has no say in whether a project accepts your pull request.",
      { h: "Acceptable use" },
      "Don’t:",
      { list: [
        "break into, overload or probe the service, or try to get around its limits;",
        "scrape beyond what the published API and its rate limits allow;",
        "use SkillIssues to spam maintainers, for example by commenting on or claiming many issues you don’t intend to work on;",
        "use themes to mislead people, hide information, or load or run code or content from elsewhere;",
        "post unlawful, abusive or infringing content, including in a theme’s name or description;",
        "use the service to collect other people’s personal data for unsolicited messages or resale.",
      ] },
      "We may limit or remove access, or delete content, if you break these terms or put the service or others at risk.",
      { h: "Themes and your content" },
      "A theme you create belongs to you. By making a theme public, you give us a free, worldwide permission to host and display it and to let anyone with its link view and apply it. A public theme shows the theme file and your GitHub username. You can make it private or delete it at any time, and the permission ends when you do (apart from copies in backups that are deleted on the normal schedule).",
      "You promise that you have the right to share what you put in a theme, and that it doesn’t break the law or anyone’s rights.",
      { h: "Reports and takedowns" },
      "If you think a theme or other content on SkillIssues infringes your rights or is abusive, email privacy@dizziedbliss.lol with a link and a short explanation. We will review it and may remove the content. For security problems, see the Security page and don’t use a public issue.",
      { h: "Sponsorship" },
      "If sponsorship is offered, it is a voluntary gift. It does not buy a contract, support, features, uptime or a refund, and it does not change these terms. Sponsors use SkillIssues on the same terms as everyone else.",
      { h: "Open source" },
      "The SkillIssues source code is released under the MIT license. These terms are about using the running service and don’t change the license of the code. See Licenses and attributions.",
      { h: "Not affiliated with GitHub" },
      "SkillIssues is an independent project. It is not affiliated with, endorsed by or sponsored by GitHub, Inc. or Microsoft. Your use of GitHub is covered by GitHub’s own terms.",
      { h: "No warranty, and limits on liability" },
      "To the extent the law allows, SkillIssues is provided without warranties of any kind, including accuracy, fitness for a particular purpose and availability. To the extent the law allows, we are not liable for indirect or consequential loss, lost data, lost opportunities or service interruptions arising from your use of SkillIssues, and our total liability to you is limited to the amount you paid us, which for a free service is zero.",
      "Nothing in these terms excludes or limits liability that cannot be excluded or limited by law, such as for fraud or intentional wrongdoing, or takes away rights you have under mandatory consumer or data protection laws where you live.",
      { h: "Ending your use" },
      "You can stop using SkillIssues and delete your account at any time. We may suspend or end your access if you break these terms, if we suspect abuse, or if we need to for legal or security reasons, or we may shut the service down.",
      { h: "Governing law and disputes" },
      "These terms are governed by the laws of India. Courts in Bengaluru, Karnataka, India have jurisdiction, except that nothing here takes away protections you have under mandatory laws of the country where you live. If something goes wrong, please email privacy@dizziedbliss.lol first so we can try to sort it out informally.",
      { h: "Changes" },
      "We may change these terms. For material changes we will put a notice on the site at least 14 days before they take effect, except where a change is needed sooner for legal or security reasons. The date at the top of the page shows when they last changed. Continuing to use SkillIssues after a change means you accept the new terms.",
      { h: "General" },
      "If any part of these terms is found invalid, the rest still applies. If we don’t enforce a term, that doesn’t mean we give it up. These terms and the Privacy policy are the whole agreement between you and us about the service.",
      { h: "Contact" },
      "Questions about these terms: privacy@dizziedbliss.lol. Security problems: see Security.",
      { link: { label: "Privacy policy", to: "/privacy" } },
      { link: { label: "Security", to: "/security" } },
      { link: { label: "Licenses and attributions", to: "/licenses" } },
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
];
