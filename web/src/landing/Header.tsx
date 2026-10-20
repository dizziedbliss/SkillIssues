import { useState } from "react";
import { Link } from "react-router";
import { Logo } from "./Logo";
import { external } from "../lib/links";
import s from "./Header.module.css";

const nav = [
  { label: "How it works", to: "/how-it-works" },
  { label: "Install desktop app", to: "/download" },
  { label: "Docs", to: "/docs" },
  { label: "SkillIssues Live", href: external.live },
  { label: "Contribute", to: "/contribute" },
];

export function Header() {
  const [open, setOpen] = useState(false);
  return (
    <header className={s.header}>
      <Link to="/" className={s.home} aria-label="SkillIssues home">
        <Logo size="header" />
      </Link>
      <button
        type="button"
        className={s.toggle}
        aria-expanded={open}
        aria-controls="site-nav"
        onClick={() => setOpen((o) => !o)}
      >
        {open ? "Close" : "Menu"}
      </button>
      <nav id="site-nav" className={`${s.nav} ${open ? s.open : ""}`} aria-label="Main">
        {nav.map((n) =>
          n.to ? (
            <Link key={n.label} to={n.to} onClick={() => setOpen(false)}>
              {n.label}
            </Link>
          ) : (
            <a key={n.label} href={n.href}>
              {n.label}
            </a>
          ),
        )}
      </nav>
    </header>
  );
}
