import { useState } from "react";
import { Link } from "react-router";
import { Logo } from "./Logo";
import { external } from "../lib/links";
import { useT, type MessageKey } from "../i18n";
import s from "./Header.module.css";

const nav: { label: MessageKey; to?: string; href?: string }[] = [
  { label: "site.nav.howItWorks", to: "/how-it-works" },
  { label: "site.nav.download", to: "/download" },
  { label: "site.nav.docs", to: "/docs" },
  { label: "site.nav.live", href: external.live },
  { label: "site.nav.contribute", to: "/contribute" },
];

export function Header() {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <header className={s.header}>
      <Link to="/" className={s.home} aria-label={t("site.home")}>
        <Logo size="header" />
      </Link>
      <button
        type="button"
        className={s.toggle}
        aria-expanded={open}
        aria-controls="site-nav"
        onClick={() => setOpen((o) => !o)}
      >
        {open ? t("site.close") : t("site.menu")}
      </button>
      <nav id="site-nav" className={`${s.nav} ${open ? s.open : ""}`} aria-label={t("site.nav")}>
        {nav.map((n) =>
          n.to ? (
            <Link key={n.label} to={n.to} onClick={() => setOpen(false)}>
              {t(n.label)}
            </Link>
          ) : (
            <a key={n.label} href={n.href}>
              {t(n.label)}
            </a>
          ),
        )}
      </nav>
    </header>
  );
}
