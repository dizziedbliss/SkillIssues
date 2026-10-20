import { Link } from "react-router";
import { Logo } from "./Logo";
import { external } from "../lib/links";
import s from "./Footer.module.css";

type Item = { label: string; to?: string; href?: string };

const columns: { title: string; items: Item[] }[] = [
  {
    title: "SkillIssues",
    items: [
      { label: "About us", to: "/about" },
      { label: "Browse Issues", to: "/app" },
      { label: "Desktop App", to: "/download" },
      { label: "Documentation", to: "/docs" },
      { label: "SkillIssues Live", href: external.live },
      { label: "Changelogs", href: external.releases },
    ],
  },
  {
    title: "Community",
    items: [
      { label: "Contribute", to: "/contribute" },
      { label: "Discussions", href: external.discussions },
      { label: "Suggest a Repository", to: "/maintainers" },
      { label: "Leaderboard", to: "/leaderboard" },
      { label: "Sponsor Us", to: "/sponsor" },
    ],
  },
  {
    title: "Resources",
    items: [
      { label: "Getting Started", to: "/docs/getting-started" },
      { label: "How It Works", to: "/how-it-works" },
      { label: "FAQ", to: "/faq" },
      { label: "Contribution Guide", to: "/docs/contributing" },
    ],
  },
  {
    title: "Support",
    items: [
      { label: "Contact Us", to: "/contact" },
      { label: "Report a Bug", href: external.reportBug },
      { label: "Request a Feature", href: external.requestFeature },
    ],
  },
  {
    title: "Legal & Trust",
    items: [
      { label: "Privacy Policy", to: "/privacy" },
      { label: "Terms of Service", to: "/terms" },
      { label: "Security", to: "/security" },
      { label: "Licenses & Attributions", to: "/licenses" },
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
