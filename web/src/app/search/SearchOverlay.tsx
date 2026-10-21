import { useEffect, useRef, useState, type FormEvent, type MouseEvent } from "react";
import { flushSync } from "react-dom";
import { useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import { withTransition } from "../transition";
import { CATEGORIES, searchURL } from "./categories";
import { PersonResult } from "./PersonResult";
import s from "./SearchOverlay.module.css";

/** Results read while typing, across the kinds; each kind shows a few and "View more". */
const QUICK = 12;
const PER_KIND = 4;
/** Typing pause before searching, so each word doesn't cost a request. */
const PAUSE_MS = 300;

/**
 * Search over whatever page you're on, the page blurred behind it. Results
 * come as you type, grouped by what matched, each kind with View more. Going
 * anywhere (a result, a kind, every result, the filters) opens the search
 * page or the issue inside the theme's page transition; search never sends
 * you back to Explore. A click outside the panel closes it.
 */
export function SearchOverlay({ onClose }: { onClose: () => void }) {
  const t = useT();
  const navigate = useNavigate();
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
  function leave(to: string, state?: unknown) {
    withTransition(() => {
      flushSync(onClose);
      return navigate(to, { state });
    });
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (draft.trim()) leave(searchURL(draft.trim()));
  }

  const groups = CATEGORIES.map((c) => ({
    ...c,
    issues: (results.data ?? []).filter((i) => i.match_category === c.key),
  })).filter((g) => g.issues.length > 0);

  return (
    <dialog
      ref={dialog}
      className={s.dialog}
      data-ui="search-dialog"
      aria-labelledby="search-title"
      aria-describedby="search-sub"
      onClose={onClose}
      // The panel fills the dialog, so a click on the dialog itself is on the backdrop.
      onClick={(e) => {
        if (e.target === dialog.current) dialog.current?.close();
      }}
    >
      <div className={s.panel}>
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
              // Nothing typed: filters alone are browsing, so Explore with its filters open.
              onClick={() => leave(draft.trim() ? searchURL(draft.trim()) : "/app", { filters: true })}
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
          {term.length >= 2 && <PersonResult term={term} onOpen={(to) => leave(to)} />}
          <div className={s.groups}>
            {groups.map((g) => (
              <section key={g.key} className={s.group} aria-label={t(g.label)}>
                <h3 className={s.groupTitle}>{t(g.label)}</h3>
                <ul>
                  {g.issues.slice(0, PER_KIND).map((i) => (
                    <li key={i.id}>
                      <a href={`/app/issues/${i.id}`} className={s.result} onClick={(e) => follow(e, `/app/issues/${i.id}`)}>
                        <span className={s.resultTitle}>
                          {i.title} <span className={s.number}>#{i.number}</span>
                        </span>
                        <span className={s.resultRepo}>{i.repository}</span>
                      </a>
                    </li>
                  ))}
                </ul>
                <a href={searchURL(term, g.key)} className={s.more} onClick={(e) => follow(e, searchURL(term, g.key))}>
                  {t("search.viewMore")}
                </a>
              </section>
            ))}
          </div>
          {results.isSuccess && results.data.length > 0 && (
            <button type="button" className={s.all} onClick={() => leave(searchURL(term))}>
              {t("search.all", { term })}
            </button>
          )}
        </div>
      </div>
    </dialog>
  );

  /** A plain click leaves through the transition; with a modifier, the browser does as asked. */
  function follow(e: MouseEvent, to: string) {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    leave(to);
  }
}
