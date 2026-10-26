import { useEffect, useRef, useState, type FormEvent } from "react";
import { SiteLink } from "../SiteLink";
import { useLocation, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { Difficulty } from "@skillissues/client";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import { useSession } from "../session";
import { useSearchOverlay } from "../search/context";
import { IssueCard } from "../IssueCard";
import { CrossIcon } from "../icons";
import { SearchBar } from "./SearchBar";
import { FilterPanel } from "./FilterPanel";
import { Pager } from "./Pager";
import { DIFFICULTIES, readQuery, toFilters, writeQuery, type CatalogQuery } from "./query";
import { useCatalog } from "./useCatalog";
import s from "./Explore.module.css";
import { rememberExplore } from "./where";
import { useDocumentTitle } from "../../lib/title";

/**
 * Explore: the heading, one search bar (clear, the filter panel, Search),
 * one toolbar (the levels, then what narrows the list, each removable), the
 * issues and the pager. All of it is kept in the address (query.ts).
 */
export function Explore() {
  const t = useT();
  useDocumentTitle(t("app.nav.explore"));
  const [params, setParams] = useSearchParams();
  const query = readQuery(params);
  const [draft, setDraft] = useState(query.search);
  // Arriving from the search overlay's "filter" opens the panel, whether or
  // not Explore was already showing.
  const location = useLocation();
  const [filtersOpen, setFiltersOpen] = useState(false);
  const wantsFilters = !!(location.state as { filters?: boolean } | null)?.filters;
  useEffect(() => {
    if (wantsFilters) setFiltersOpen(true);
  }, [wantsFilters, location.key]);
  const overlay = useSearchOverlay();
  const top = useRef<HTMLDivElement>(null);

  // The search box follows the address (back, forward, the search overlay).
  useEffect(() => setDraft(query.search), [query.search]);
  // Kept, so links back to Explore return to these filters (where.ts).
  const search = params.toString();
  useEffect(() => rememberExplore(search ? `?${search}` : ""), [search]);

  // Difficulty not chosen yet: your profile's level, or every level signed out.
  const { user, isPending: sessionPending } = useSession();
  // Languages not chosen yet: your profile's preferred ones, or every language signed out.
  const profile = useQuery({
    queryKey: ["me"],
    queryFn: ({ signal }) => api.me({ signal }),
    enabled: !!user && (query.difficulty === null || !query.languageChosen),
  });
  function fromProfile<T>(pick: (p: NonNullable<typeof profile.data>) => T, none: T): T | null {
    if (sessionPending) return null;
    if (!user) return none;
    if (profile.isPending) return null;
    return profile.data ? pick(profile.data) : none;
  }
  const difficulty: Difficulty[] | null = query.difficulty ?? fromProfile((p) => [p.experience_level], []);
  const language: string[] | null = query.languageChosen ? query.language : fromProfile((p) => p.preferred_languages ?? [], []);

  const settled = difficulty && language ? { ...query, language } : null;
  const filters = settled && toFilters(settled, difficulty!);
  const key = JSON.stringify([query.search, filters]);
  const catalog = useCatalog(key, query.search, filters);

  function update(next: Partial<CatalogQuery>) {
    setParams(writeQuery({ ...query, difficulty, ...next }));
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    update({ search: draft.trim() });
  }

  function toggleLevel(d: Difficulty) {
    const now = difficulty ?? [];
    update({ difficulty: now.includes(d) ? now.filter((x) => x !== d) : DIFFICULTIES.filter((x) => x === d || now.includes(x)) });
  }

  function go(i: number) {
    catalog.go(i);
    top.current?.scrollIntoView({ block: "start" });
  }

  const issues = catalog.data?.issues ?? [];

  return (
    <div className={s.explore}>
      <h1 className={s.title}>{t("explore.title")}</h1>

      <div className={s.search}>
        {/* Shares its name with the overlay's box, so one glides into the other; never both at once. */}
        <SearchBar
          id="explore-search"
          value={draft}
          onChange={setDraft}
          onSubmit={submit}
          onClear={() => {
            setDraft("");
            update({ search: "" });
          }}
          onFilters={() => setFiltersOpen((o) => !o)}
          filtersOpen={filtersOpen}
          filtersId="explore-filters"
          transitionName={overlay.isOpen ? "none" : "search-box"}
          hint={{ key: "/", title: t("explore.searchHintLink") }}
        />
      </div>

      {filtersOpen && settled && (
        <div id="explore-filters">
          <FilterPanel
            key={key}
            value={settled}
            difficulty={difficulty!}
            onApply={(panel, levels) => {
              update({ ...panel, difficulty: levels, languageChosen: true });
              setFiltersOpen(false);
            }}
          />
        </div>
      )}

      {/* One row under the bar: the level, then what narrows the list, each removable. */}
      <div ref={top} className={s.toolbar}>
        <div className={s.segments} role="group" aria-label={t("explore.difficulties")} data-ui="difficulty">
          {DIFFICULTIES.map((d) => (
            <button
              key={d}
              type="button"
              className={s.segment}
              aria-pressed={difficulty?.includes(d) ?? false}
              disabled={!difficulty}
              onClick={() => toggleLevel(d)}
            >
              {t(`difficulty.${d}`)}
            </button>
          ))}
        </div>

        {[...query.repo, ...query.owner].map((v) => (
          <span key={v} className={s.scope}>
            {v}
            <button type="button" aria-label={t("explore.scopeClear")} onClick={() => update({ repo: [], owner: [] })}>
              <CrossIcon />
            </button>
          </span>
        ))}

        {!query.languageChosen && language && language.length > 0 && (
          <span className={s.scope} data-ui="your-languages" title={t("explore.yourLanguages")}>
            {language.join(", ")}
            <button type="button" aria-label={t("explore.allLanguages")} onClick={() => update({ language: [], languageChosen: true })}>
              <CrossIcon />
            </button>
          </span>
        )}

        <SiteLink to="/docs/good-first-issues" className={s.newHere}>{t("explore.newHere")}</SiteLink>
      </div>

      {query.search && <h2 className={s.resultsFor}>{t("explore.results", { term: query.search })}</h2>}

      <div className={s.results} aria-busy={catalog.isFetching}>
        {catalog.error && (
          <p className={s.error} role="alert">
            {catalog.error.message}{" "}
            <button type="button" onClick={() => void catalog.refetch()}>{t("explore.retry")}</button>
          </p>
        )}
        {!catalog.error && (catalog.isPending || !settled) && <p className={s.quiet}>{t("explore.loading")}</p>}
        {!catalog.error && catalog.isSuccess && issues.length === 0 && (
          <p className={s.quiet}>{query.search ? t("explore.emptySearch") : t("explore.empty")}</p>
        )}
        <div className={catalog.isPlaceholderData ? s.stale : undefined}>
          {issues.map((it, i) => (
            <IssueCard key={it.id} issue={it} index={i} />
          ))}
        </div>
      </div>

      {(catalog.pages > 1 || catalog.index > 0) && (
        <Pager index={catalog.index} pages={catalog.pages} hasNext={catalog.hasNext} busy={catalog.isFetching} onGo={go} />
      )}
    </div>
  );
}
