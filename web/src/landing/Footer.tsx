import { Link } from "react-router";
import { Logo } from "./Logo";
import s from "./Footer.module.css";

type Item = { label: string; to?: string; href?: string };

/** Every footer link goes to its page; only Discussions and the live site leave this site. */
const columns: { title: string; items: Item[] }[] = [
  {
    title: "SkillIssues",
    items: [
      { label: "About us", to: "/about" },
      { label: "Browse issues", to: "/app" },
      { label: "Desktop app", to: "/download" },
      { label: "Documentation", to: "/docs" },
      { label: "SkillIssues Live", to: "/docs/skillissues-live" },
      { label: "Changelog", to: "/docs/changelog" },
    ],
  },
  {
    title: "Guides",
    items: [
      { label: "Getting started", to: "/docs/getting-started" },
      { label: "How it works", to: "/how-it-works" },
      { label: "Finding issues", to: "/docs/finding-issues" },
      { label: "Difficulty ratings", to: "/docs/difficulty-ratings" },
      { label: "Themes", to: "/docs/themes" },
      { label: "Public API", to: "/docs/public-api" },
    ],
  },
  {
    title: "Community",
    items: [
      { label: "Contribute", to: "/contribute" },
      { label: "Contribution guide", to: "/docs/contributing" },
      { label: "For maintainers", to: "/maintainers" },
      { label: "Suggest a repository", to: "/docs/suggest-a-repository" },
      { label: "Discussions", to: "/docs/discussions" },
      { label: "Leaderboard", to: "/leaderboard" },
      { label: "Sponsor us", to: "/sponsor" },
    ],
  },
  {
    title: "Support",
    items: [
      { label: "FAQ", to: "/faq" },
      { label: "Contact us", to: "/contact" },
      { label: "Report a bug", to: "/docs/report-a-bug" },
      { label: "Request a feature", to: "/docs/request-a-feature" },
    ],
  },
  {
    title: "Legal & Trust",
    items: [
      { label: "Privacy policy", to: "/privacy" },
      { label: "Terms of service", to: "/terms" },
      { label: "Security", to: "/security" },
      { label: "Licenses and attributions", to: "/licenses" },
    ],
  },
];

export function Footer() {
  return (
    <footer className={s.footer}>
      <div className={s.columns}>
        {columns.map((c) => (
          <nav key={c.title} aria-label={c.title}>
            <h2 className={s.title}>{c.title}</h2>
            <ul>
              {c.items.map((i) => (
                <li key={i.label}>
                  {i.to ? <Link to={i.to}>{i.label}</Link> : <a href={i.href}>{i.label}</a>}
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
      <div className={s.sign}>
        <Logo size="footer" />
        <p>There will always be more work to do. Start with one issue.</p>
      </div>
      <div className={s.base}>
        <p>© {new Date().getFullYear()} SkillIssues</p>
        <p>Made by and for the open-source community.</p>
      </div>
    </footer>
  );
}
