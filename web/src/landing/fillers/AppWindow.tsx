import s from "./Filler.module.css";

/*
 * Next to "Keep the contribution moving": the desktop app, drawn in the
 * page's own mono as a schematic rather than a screenshot. It shows the flow
 * the copy describes: pull, change, push, follow the pull request.
 */
const art = String.raw`┌─ SkillIssues ────────────────────────────────── □ ✕ ┐
│                                                     │
│  working on                                         │
│  ─────────────────────────────────────────────────  │
│  acme/widgets #482                                  │
│  Fix hydration warning on nested <Link>             │
│                                                     │
│  [✓] pull      cloned to ~/code/widgets             │
│  [✓] change    3 files, +41 −12                     │
│  [✓] push      fix/nested-link-hydration            │
│  [ ] review    PR #491 open, 2 checks running       │
│                                                     │
│  ─────────────────────────────────────────────────  │
│  [ open in editor ]   [ view PR ]   [ next issue ]  │
└─────────────────────────────────────────────────────┘`;

export function AppWindow() {
  // The red marks are the step in progress.
  const [before, after] = art.split("[ ] review");
  return (
    <figure>
      <pre className={s.window} aria-hidden="true">
        {before}
        <em>[ ] review</em>
        {after}
      </pre>
      <figcaption className={s.caption}>
        <span className="visually-hidden">
          A sketch of the desktop app: an issue being worked on, with the project pulled, changes made and pushed,
          and its pull request under review.
        </span>
        A sketch of the desktop app. Your code and your GitHub token stay on your machine.
      </figcaption>
    </figure>
  );
}
