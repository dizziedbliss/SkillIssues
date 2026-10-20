import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { useT } from "../i18n";
import { docs } from "./docs";
import s from "./DocSearch.module.css";

/** Results listed at most. */
const SHOWN = 8;
/** Characters of context on each side of a match. */
const AROUND = 60;

// The docs as plain text, once: titles, summaries and bodies.
const index = docs.map((d) => ({
  doc: d,
  title: d.title.toLowerCase(),
  text: `${d.summary} ${d.html.replace(/<[^>]+>/g, " ").replace(/&[a-z#0-9]+;/g, " ")}`.replace(/\s+/g, " "),
}));

/**
 * Searches every docs page as you type: every word must appear, title
 * matches first. "/" or Ctrl/⌘K from anywhere on a docs page puts the
 * cursor here; Enter opens the first result.
 */
export function DocSearch() {
  const t = useT();
  const navigate = useNavigate();
  const input = useRef<HTMLInputElement>(null);
  const [q, setQ] = useState("");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName);
      if ((e.key === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing && !e.metaKey && !e.ctrlKey && !e.altKey)) {
        e.preventDefault();
        input.current?.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const results = useMemo(() => {
    const words = q.toLowerCase().split(/\s+/).filter(Boolean);
    if (!words.length) return [];
    return index
      .filter((e) => words.every((w) => e.title.includes(w) || e.text.toLowerCase().includes(w)))
      .map((e) => {
        const at = e.text.toLowerCase().indexOf(words[0]!);
        const snippet =
          at < 0 ? e.doc.summary : `${at > AROUND ? "…" : ""}${e.text.slice(Math.max(0, at - AROUND), at + AROUND)}…`;
        return { doc: e.doc, snippet, inTitle: words.some((w) => e.title.includes(w)) };
      })
      .sort((a, b) => Number(b.inTitle) - Number(a.inTitle))
      .slice(0, SHOWN);
  }, [q]);

  return (
    <div className={s.search} role="search">
      <label className={s.box}>
        <span className="visually-hidden">{t("docs.search")}</span>
        <input
          ref={input}
          type="search"
          value={q}
          placeholder={t("docs.search")}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && results[0]) {
              void navigate(results[0].doc.path);
              setQ("");
            }
            if (e.key === "Escape") setQ("");
          }}
        />
        <kbd className={s.kbd} aria-hidden="true">/</kbd>
      </label>
      {q.trim() && (
        <div className={s.results} aria-live="polite">
          {results.length === 0 && <p className={s.none}>{t("docs.searchNone")}</p>}
          <ul>
            {results.map((r) => (
              <li key={r.doc.path}>
                <Link to={r.doc.path} onClick={() => setQ("")}>
                  <span className={s.title}>{r.doc.title}</span>
                  <span className={s.snippet}>{r.snippet}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
