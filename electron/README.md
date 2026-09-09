# SkillIssues Desktop

The Electron client currently has the home view, issue-details view, and submissions view. It loads live issue recommendations, supports search and saved state, shows the signed-in profile data when available, opens issue details first, and only reveals local clone controls after the user explicitly presses `Start challenge`. Clones are restricted to `Documents/SkillIssues`; no filesystem chooser is used. It does not receive backend secrets, execute cloned code, or expose unrestricted filesystem access.

Install dependencies with `npm install` inside this directory, then run `npm start` while the API is available.

The desktop flow lets a user inspect an issue before starting, then clone a public GitHub repository into `Documents/SkillIssues/<repository>` and inspect Git status. There is no destination selector or folder picker. The submissions tab reads contribution state from `/contributions`. Clone and Git inspection commands are appended to `.skillissues/commands.log` inside the cloned repository. After cloning, the user can explicitly create a validated branch, push it to the repository remote, and open the workspace. The preload bridge exposes only the narrow workspace actions; backend credentials never enter the renderer.

```bash
cd electron
npm install
npm start
```
