import { useEffect, useRef, useState, type FormEvent } from "react";
import { flushSync } from "react-dom";
import { useLocation, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { api } from "../../lib/api";
import { useT, type MessageKey } from "../../i18n";
import { withTransition } from "../transition";
import s from "./SearchOverlay.module.css";

/** Results shown while typing; "See all" opens the rest in Explore. */
const QUICK = 8;
/** Typing pause before searching, so each word doesn't cost a request. */
const PAUSE_MS = 300;

/** Search's categories, in the order the API ranks them (match_category). */
const CATEGORIES: { key: string; label: MessageKey }[] = [
  { key: "Repository", label: "search.cat.Repository" },
  { key: "Title", label: "search.cat.Title" },
  { key: "Language", label: "search.cat.Language" },
  { key: "Author", label: "search.cat.Author" },
];

/**
 * Search over whatever page you're on, the page blurred behind it. Results
 * come as you type, grouped by what matched. Going to a result, or to all
 * of them in Explore, closes it inside the theme's page transition, and its
 * search box glides into Explore's (they share a view-transition-name).
 */
export function SearchOverlay({ onClose }: { onClose: () => void }) {
  const t = useT();
  const navigate = useNavigate();
  const location = useLocation();
  const dialog = useRef<HTMLDialogElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState("");
  const [term, setTerm] = useState("");

  useEffect(() => {
    const d = dialog.current;
    if (d && !d.open) d.showModal();
    // showModal focuses the first button; the search box is where typing goes.
    input.current?.focus();
  }, []);

  useEffect(() => {
    const id = setTimeout(() => setTerm(draft.trim()), PAUSE_MS);
    return () => clearTimeout(id);
  }, [draft]);

  const results = useQuery({
    queryKey: ["quickSearch", term],
    queryFn: ({ signal }) => api.search(term, { limit: QUICK }, { signal }),
    enabled: term.length >= 2,
    staleTime: 60_000,
  });

  /** Leaves for `to`, closing the overlay in the same transition. */
  function leave(to: string) {
    withTransition(() => {
      flushSync(onClose);
      return navigate(to);
    });
  }

  /** Explore with this search; on Explore already, its filters stay. */
  function exploreURL(withFilters = false) {
    const params = new URLSearchParams(location.pathname === "/app" ? location.search : "");
    params.set("search", draft.trim());
    if (!draft.trim()) params.delete("search");
    const q = params.toString();
    return { to: `/app${q ? `?${q}` : ""}`, state: withFilters ? { filters: true } : undefined };
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    leave(exploreURL().to);
  }

  const groups = CATEGORIES.map((c) => ({
    ...c,
    issues: (results.data ?? []).filter((i: Issue) => (i.match_category ?? "Title") === c.key),
  })).filter((g) => g.issues.length > 0);

  return (
    <dialog ref={dialog} className={s.dialog} aria-labelledby="search-title" aria-describedby="search-sub" onClose={onClose}>
      <div className={s.head}>
        <h2 id="search-title" className={s.title}>{t("search.title")}</h2>
        <button type="button" className={s.close} onClick={() => dialog.current?.close()}>
          {t("search.close")}
        </button>
      </div>
      <p className={s.sub} id="search-sub">{t("search.sub")}</p>
      <form className={s.form} role="search" onSubmit={submit}>
        <label htmlFor="overlay-search" className={s.label}>{t("explore.searchLabel")}</label>
        <div className={s.box} style={{ viewTransitionName: "search-box" }}>
          <input
            ref={input}
            id="overlay-search"
            type="search"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={t("explore.searchPlaceholder")}
            autoComplete="off"
          />
          {draft && (
            <button type="button" className={s.inBox} aria-label={t("explore.clearLabel")} onClick={() => setDraft("")}>
              <span aria-hidden="true">x</span> {t("explore.clear")}
            </button>
          )}
          <button
            type="button"
            className={`${s.inBox} ${s.filter}`}
            onClick={() => {
              const { to, state } = exploreURL(true);
              withTransition(() => {
                flushSync(onClose);
                return navigate(to, { state });
              });
            }}
          >
            {t("explore.filter")}
          </button>
        </div>
        <button type="submit" className={s.submit}>{t("explore.search")}</button>
      </form>

      <div className={s.results} aria-live="polite" aria-busy={results.isFetching}>
        {term.length > 0 && term.length < 2 && <p className={s.note}>{t("search.short")}</p>}
        {results.isFetching && !results.data && <p className={s.note}>{t("search.searching")}</p>}
        {results.error && (
          <p className={s.error} role="alert">
            {results.error.message}{" "}
            <button type="button" onClick={() => void results.refetch()}>{t("explore.retry")}</button>
          </p>
        )}
        {results.isSuccess && term.length >= 2 && results.data.length === 0 && (
          <p className={s.note}>{t("explore.emptySearch")}</p>
        )}
        {groups.map((g) => (
          <section key={g.key} className={s.group} aria-label={t(g.label)}>
            <h3 className={s.groupTitle}>{t(g.label)}</h3>
            <ul>
              {g.issues.map((i) => (
                <li key={i.id}>
                  <a
                    href={`/app/issues/${i.id}`}
                    className={s.result}
                    onClick={(e) => {
                      if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                      e.preventDefault();
                      leave(`/app/issues/${i.id}`);
                    }}
                  >
                    <span className={s.resultTitle}>
                      {i.title} <span className={s.number}>#{i.number}</span>
                    </span>
                    <span className={s.resultRepo}>{i.repository}</span>
                  </a>
                </li>
              ))}
            </ul>
          </section>
        ))}
        {results.isSuccess && results.data.length > 0 && (
          <button type="button" className={s.all} onClick={() => leave(exploreURL().to)}>
            {t("search.all", { term })}
          </button>
        )}
      </div>
    </dialog>
  );
}
