import type { MouseEvent } from "react";
import { Link, NavLink, useNavigate } from "react-router";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import { Footer } from "../landing/Footer";
import { SECTIONS, inSection, type DocPage as Doc } from "./docs";
import s from "./DocPage.module.css";

const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });

/** A documentation page: the sidebar of every page by section, and the rendered Markdown. */
export function DocPage({ doc }: { doc: Doc }) {
  const navigate = useNavigate();

  // Links inside the rendered Markdown to the site's own pages navigate in the
  // app rather than reloading it; anything else behaves as a normal link.
  function onClick(e: MouseEvent<HTMLDivElement>) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (!a || !href || !href.startsWith("/") || href.startsWith("//") || a.target) return;
    e.preventDefault();
    void navigate(href);
  }

  return (
    <div className={tokens.page}>
      <Header />
      <div className={s.layout}>
        <nav className={s.sidebar} aria-label="Documentation">
          {SECTIONS.map((section) => {
            const pages = inSection(section);
            if (!pages.length) return null;
            return (
              <div key={section} className={s.group}>
                <p className={s.groupTitle}>{section}</p>
                <ul>
                  {pages.map((p) => (
                    <li key={p.path}>
                      <NavLink to={p.path} end className={({ isActive }) => (isActive ? s.active : undefined)}>
                        {p.title}
                      </NavLink>
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
          <div className={s.group}>
            <p className={s.groupTitle}>Legal &amp; Trust</p>
            <ul>
              <li><Link to="/privacy">Privacy policy</Link></li>
              <li><Link to="/terms">Terms of service</Link></li>
              <li><Link to="/security">Security</Link></li>
              <li><Link to="/licenses">Licenses and attributions</Link></li>
            </ul>
          </div>
        </nav>
        <main className={s.main}>
          <article>
            <p className={s.section}>{doc.section}</p>
            <h1 className={s.title}>{doc.title}</h1>
            <p className={s.summary}>{doc.summary}</p>
            {/* First-party Markdown from content/docs, rendered with raw HTML off (docs.ts). */}
            <div className={s.prose} onClick={onClick} dangerouslySetInnerHTML={{ __html: doc.html }} />
            {doc.updated && <p className={s.updated}>Last updated {dateFormat.format(new Date(doc.updated))}.</p>}
          </article>
        </main>
      </div>
      <Footer />
    </div>
  );
}
