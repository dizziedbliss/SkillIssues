# SkillIssues desktop: release notes

Each release's notes are the section under its version. The release workflow
(.github/workflows/release.yml) puts that section on the GitHub release and
refuses to build a tag that has none; the app shows the same notes when an
update is available, and once after it installs.

## 1.5.0

**A calmer, clearer app.** Lines and chips are quiet now, and red is kept for what matters: the main button, where you are, what you've earned, and section headings. Light mode uses warmer, slightly dimmed paper and softer ink, easier on the eyes over a long session.

**Your profile, rebuilt.** Your level sits large at the top with its bar, your merges, languages and streak beside it. **Next up** leads with what you're doing (pull requests in review with where they stand, issues you've claimed, your learning path), then the quests you're closest to finishing. Your pull requests are one list, each once, with the XP it earned. Hover a badge to see how you earned it.

**Your contributions** show your numbers, your year, every pull request with All, Merged and Open, where your XP came from, and every quest.

**Finding issues is quicker.** One search bar everywhere, with Filters beside it; filters are a row of pills that say what they mean when you open them; the levels and your languages sit in one row you can clear. Search (`/`) opens as a palette with results as you type.

**Tidier everywhere else:** issue pages show six labels before "+ more", the learning path is a list you can start with one click, Settings is a row per setting with a shorter language list, themes appear as miniatures of themselves, and the menu holds what's about you while the top bar holds where you work.

## 1.3.0

**Easier on the eyes.** The Default and Midnight themes moved from pure white and pure black to warm paper and ink, with softer lines. The red is still fully saturated, now set for each mode so text on it and in it is easy to read: every pairing passes WCAG AA. Your own themes are unchanged; ones built on Default or Midnight pick up the new colours for anything they don't set.

**The installer shows the wordmark alone.** The mark stays the app's icon, nothing more.

## 1.2.1

**When your fork is deleted on GitHub,** the app now says so, and **Fork again** makes a new one and points your folder at it. Your commits stay, ready to push.

**Pull requests show where they stand on GitHub:** open, merged, or closed. A closed one offers **Open a new pull request**, and Working on shows each state.

**A new look for the Windows installer:** dark, like the app, with the SkillIssues mark.

## 1.2.0

**Updates ask first.** When a new version is out, the title bar says **Update available**. Open it to read what's new, then download it when you choose. Nothing downloads or restarts on its own; after the download you restart into it, or it installs when you close the app. And after an update, the app shows you what changed.

**A proper installer on Windows.** It now opens with what SkillIssues does, shows the license and terms, and lets you choose where to install it, for just you or everyone on the computer.

**Working on.** A new page in the menu lists every issue you've started in the app and what's next for each. Commit, push and the pull request are now separate steps:

- **Commit** takes a title and, if you like, a description.
- **Push** sends only your commits, never uncommitted changes.
- **Pull request** is yours to write. Tick **Close #number when it's merged** if your change fixes the issue.

**For you.** Explore's default list now picks issues that suit you (your languages, your level, what you've merged before), look likely to get merged, and have waited. Each card says why it's there.

**Fixes**

- Copying the sign-in code works, and says so.
- Connecting GitHub for pushing no longer says you're signed in as "nobody".
- Open in VS Code finds the folder on Windows.
- Giving up a path, releasing a claim and deleting a theme ask first.
- Explore keeps its filters and page when you open an issue and come back.
- Issue descriptions show attached images and colour their code.
- The app is smaller to download.
