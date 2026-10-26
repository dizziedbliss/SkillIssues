import { useEffect, useState, type MouseEvent } from "react";
import { Link, NavLink, useNavigate } from "react-router";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import { Footer } from "../landing/Footer";
import { SECTIONS, inSection, type DocPage as Doc } from "./docs";
import { useI18n } from "../i18n";
import { DocSearch } from "./DocSearch";
import { Downloads } from "./Downloads";
import s from "./DocPage.module.css";
import { useDocumentTitle } from "../lib/title";

/** A documentation page: the sidebar of every page by section, the rendered Markdown, and the pages either side. */
export function DocPage({ doc }: { doc: Doc }) {
  useDocumentTitle(doc.title);
  const navigate = useNavigate();
  const { t, locale } = useI18n();
  const dateFormat = new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
  // The sidebar's order, read straight through: the pages either side of this one.
  const order = SECTIONS.flatMap((section) => inSection(section));
  const at = order.findIndex((p) => p.path === doc.path);
  const prev = at > 0 ? order[at - 1] : undefined;
  const next = at >= 0 && at < order.length - 1 ? order[at + 1] : undefined;
  const current = useCurrentSection(doc.toc.map((x) => x.id));

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
        <div className={s.searchArea}>
          <DocSearch />
        </div>
        <nav className={s.sidebar} aria-label={t("docs.nav")}>
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
            <p className={s.groupTitle}>{t("docs.legal")}</p>
            <ul>
              <li><Link to="/privacy">{t("footer.privacy")}</Link></li>
              <li><Link to="/terms">{t("footer.terms")}</Link></li>
              <li><Link to="/security">{t("footer.security")}</Link></li>
              <li><Link to="/licenses">{t("footer.licenses")}</Link></li>
            </ul>
          </div>
        </nav>
        <main className={s.main}>
          <article>
            <p className={s.section}>{doc.section}</p>
            <h1 className={s.title}>{doc.title}</h1>
            <p className={s.summary}>{doc.summary}</p>
            {doc.toc.length >= TOC_MIN && (
              <details className={s.tocInline}>
                <summary>{t("docs.onThisPage")}</summary>
                <Toc items={doc.toc} current={current} />
              </details>
            )}
            {doc.path === "/download" && <Downloads />}
            {/* First-party Markdown from content/docs, rendered with raw HTML off (docs.ts). */}
            <div className={s.prose} onClick={onClick} dangerouslySetInnerHTML={{ __html: doc.html }} />
            {doc.updated && <p className={s.updated}>{t("docs.updated", { date: dateFormat.format(new Date(doc.updated)) })}</p>}
            {(prev || next) && (
              <nav className={s.pager} aria-label={t("docs.pager")}>
                {prev ? (
                  <Link to={prev.path} className={s.prev}>
                    <span className={s.pagerLabel}>{t("docs.prev")}</span>
                    {prev.title}
                  </Link>
                ) : (
                  <span />
                )}
                {next && (
                  <Link to={next.path} className={s.next}>
                    <span className={s.pagerLabel}>{t("docs.next")}</span>
                    {next.title}
                  </Link>
                )}
              </nav>
            )}
          </article>
        </main>
        {doc.toc.length >= TOC_MIN && (
          <aside className={s.toc} aria-labelledby="toc-title">
            <p id="toc-title" className={s.groupTitle}>{t("docs.onThisPage")}</p>
            <Toc items={doc.toc} current={current} />
          </aside>
        )}
      </div>
      <Footer />
    </div>
  );
}

/** Fewer sections than this and a page needs no index. */
const TOC_MIN = 3;

/** The page's sections as links, the one being read marked. */
function Toc({ items, current }: { items: { id: string; text: string }[]; current: string }) {
  return (
    <ul className={s.tocList}>
      {items.map((x) => (
        <li key={x.id}>
          <a href={`#${x.id}`} aria-current={current === x.id ? "location" : undefined}>{x.text}</a>
        </li>
      ))}
    </ul>
  );
}

/**
 * Which section is being read: the last heading that has scrolled past the
 * top third of the window. "" until the page has run in a browser, so the
 * prerendered index marks nothing.
 */
function useCurrentSection(ids: string[]): string {
  const [current, setCurrent] = useState("");
  const key = ids.join(" ");
  useEffect(() => {
    if (!key) return;
    const headings = key.split(" ").map((id) => document.getElementById(id)).filter((h): h is HTMLElement => !!h);
    const update = () => {
      const line = window.innerHeight / 3;
      let at = "";
      for (const h of headings) if (h.getBoundingClientRect().top <= line) at = h.id;
      setCurrent(at || headings[0]?.id || "");
    };
    update();
    window.addEventListener("scroll", update, { passive: true });
    window.addEventListener("resize", update);
    return () => {
      window.removeEventListener("scroll", update);
      window.removeEventListener("resize", update);
    };
  }, [key]);
  return current;
}
