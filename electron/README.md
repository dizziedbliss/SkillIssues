# SkillIssues Desktop

The Electron boundary is intentionally small: it opens the local workspace, validates that it is an existing directory, and reads basic Git status. It does not receive backend secrets, execute cloned code, or expose unrestricted filesystem access.

Install dependencies with `npm install` inside this directory, then run `npm start` while the Compose frontend is available.
