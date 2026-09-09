# SkillIssues Desktop

The Electron client currently has the Figma-based home view, issue-details view, and submissions view. It loads live issue recommendations, supports search and saved state, shows the profile rail, opens issue details first, and only reveals local clone controls after the user explicitly presses `Start challenge`. It does not receive backend secrets, execute cloned code, or expose unrestricted filesystem access.

Install dependencies with `npm install` inside this directory, then run `npm start` while the API is available.

The desktop flow lets a user inspect an issue before starting, choose an existing local destination only after starting, clone a public GitHub repository, and inspect Git status. The submissions tab reads contribution state from `/contributions`. Clone and Git inspection commands are appended to `.skillissues/commands.log` inside the cloned repository. After cloning, the user can explicitly create a validated branch and push it to the repository remote. The preload bridge exposes only the narrow workspace actions; backend credentials never enter the renderer.

```bash
cd electron
npm install
npm start
```
