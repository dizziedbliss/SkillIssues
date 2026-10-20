import { external } from "../../lib/links";
import s from "./Filler.module.css";

/*
 * Next to "Built with trust in mind": the claims, each one true of the code
 * as it stands (CLAUDE.md, Credentials and identity; Deployment).
 */
const facts = [
  {
    title: "We keep no GitHub tokens.",
    text: "Signing in reads your public profile, then the token is thrown away. Nothing on our servers can act as you.",
  },
  {
    title: "Your code stays with you.",
    text: "Forks, pushes and pull requests go from the desktop app, with a token that never leaves your machine.",
  },
  {
    title: "Every release is signed.",
    text: "The server only runs images that are verified against the build that made them.",
  },
  {
    title: "The code is open.",
    text: "Read exactly what runs, and tell us when something looks wrong.",
    link: external.repo,
  },
];

export function TrustFacts() {
  return (
    <ul className={s.facts}>
      {facts.map((f) => (
        <li key={f.title} className={s.fact}>
          <span className={s.marker} aria-hidden="true" />
          <p className={s.factTitle}>{f.link ? <a href={f.link} style={{ color: "inherit" }}>{f.title}</a> : f.title}</p>
          <p className={s.factText}>{f.text}</p>
        </li>
      ))}
    </ul>
  );
}
