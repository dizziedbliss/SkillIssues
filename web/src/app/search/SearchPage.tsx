import { useEffect, useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import { IssueCard } from "../IssueCard";
import { FilterPanel } from "../explore/FilterPanel";
import { Pager } from "../explore/Pager";
import { readQuery, toFilters, writeQuery, type CatalogQuery } from "../explore/query";
import { CATEGORIES, searchURL } from "./categories";
import { useSearchOverlay } from "./context";
import es from "../explore/Explore.module.css";
import s from "./SearchPage.module.css";

/** Issues on a page of one kind. */
const PAGE_SIZE = 20;
/** Each kind's issues shown when every kind is listed. */
const PER_KIND = 4;

/**
 * The search page (/app/search): every kind of match, each with View more,
 * or one kind on its own (?in=), paged. Filters apply as on Explore; with
 * none chosen, every difficulty. This is where search leads, never Explore.
 */
export function SearchPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const navigate = useNavigate();
  const query = readQuery(params);
  const overlay = useSearchOverlay();
  const kind = CATEGORIES.find((c) => c.param === params.get("in"));
  const page = Math.max(1, Number(params.get("page")) || 1);
  const difficulty = query.difficulty ?? [];
  const filters = toFilters(query, difficulty);

  const [draft, setDraft] = useState(query.search);
  useEffect(() => setDraft(query.search), [query.search]);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const wantsFilters = !!(location.state as { filters?: boolean } | null)?.filters;
  useEffect(() => {
    if (wantsFilters) setFiltersOpen(true);
  }, [wantsFilters, location.key]);

  const results = useQuery({
    queryKey: ["searchPage", query.search, kind?.key ?? "", filters, page],
    queryFn: ({ signal }) =>
      api.search(
        query.search,
        kind ? { ...filters, sort: undefined, category: kind.key, limit: PAGE_SIZE, page } : { ...filters, sort: undefined, limit: PER_KIND * CATEGORIES.length },
        { signal },
      ),
    enabled: query.search.length > 0,
    placeholderData: keepPreviousData,
  });

  function update(next: Partial<CatalogQuery>, extra: Record<string, string> = {}) {
    const p = writeQuery({ ...query, ...next });
    if (kind) p.set("in", kind.param);
    for (const [k, v] of Object.entries(extra)) p.set(k, v);
    setParams(p);
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    update({ search: draft.trim() });
  }

  const data = results.data ?? [];

  return (
    <div className={es.explore}>
      <h1 className={es.title}>{t("search.page.title")}</h1>

      <form className={es.search} role="search" onSubmit={submit}>
        <label htmlFor="page-search" className={es.searchLabel}>{t("explore.searchLabel")}</label>
        {/* Shares its name with the overlay's box, so one glides into the other. */}
        <div className={es.box} style={{ viewTransitionName: overlay.isOpen ? "none" : "search-box" }}>
          <input
            id="page-search"
            type="search"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={t("explore.searchPlaceholder")}
            autoComplete="off"
          />
          {draft && (
            <button type="button" className={es.inBox} aria-label={t("explore.clearLabel")} onClick={() => setDraft("")}>
              <span aria-hidden="true">x</span> <span className={es.clearWord}>{t("explore.clear")}</span>
            </button>
          )}
          <button
            type="button"
            className={`${es.inBox} ${es.filterToggle}`}
            aria-expanded={filtersOpen}
            aria-controls="search-filters"
            onClick={() => setFiltersOpen((o) => !o)}
          >
            {t("explore.filter")}
          </button>
        </div>
        <button type="submit" className={es.submit}>{t("explore.search")}</button>
      </form>
      <p className={es.hint}>{t("search.page.sub")}</p>

      {filtersOpen && (
        <div id="search-filters">
          <FilterPanel
            key={JSON.stringify(filters)}
            value={query}
            difficulty={difficulty}
            onApply={(panel, levels) => {
              setFiltersOpen(false);
              // Filters with nothing to search are browsing: Explore, filtered.
              if (!query.search) {
                void navigate(`/app?${writeQuery({ ...query, ...panel, difficulty: levels }).toString()}`);
                return;
              }
              update({ ...panel, difficulty: levels });
            }}
          />
        </div>
      )}

      {!query.search && <p className={s.note}>{t("search.page.empty")}</p>}
      {results.error && (
        <p className={s.error} role="alert">
          {results.error.message}{" "}
          <button type="button" onClick={() => void results.refetch()}>{t("explore.retry")}</button>
        </p>
      )}
      {query.search && results.isPending && <p className={s.note}>{t("explore.loading")}</p>}

      {query.search && kind && (
        <section className={s.kind} aria-labelledby="kind-title">
          <div className={s.kindHead}>
            <h2 id="kind-title" className={es.next}>{t("search.page.in", { kind: t(kind.label), term: query.search })}</h2>
            <Link to={searchURL(query.search)} className={s.back}>{t("search.page.all")}</Link>
          </div>
          {results.isSuccess && data.length === 0 && <p className={s.note}>{t("search.page.none")}</p>}
          <div className={results.isPlaceholderData ? es.stale : undefined}>
            {data.map((i) => (
              <IssueCard key={i.id} issue={i} />
            ))}
          </div>
          {(page > 1 || data.length === PAGE_SIZE) && (
            <Pager
              index={page - 1}
              pages={page + (data.length === PAGE_SIZE ? 1 : 0)}
              hasNext={data.length === PAGE_SIZE && !results.isPlaceholderData}
              busy={results.isFetching}
              onGo={(i) => {
                update({}, { page: String(i + 1) });
                // The page scrolls, or (in the desktop app) the app's own root does.
                window.scrollTo({ top: 0 });
                document.querySelector("[data-app]")?.scrollTo({ top: 0 });
              }}
            />
          )}
        </section>
      )}

      {query.search && !kind && results.isSuccess && (
        <>
          <h2 className={`${es.next} ${s.resultsTitle}`}>{t("search.page.results", { term: query.search })}</h2>
          {data.length === 0 && <p className={s.note}>{t("explore.emptySearch")}</p>}
          {CATEGORIES.map((c) => {
            const issues = data.filter((i) => i.match_category === c.key).slice(0, PER_KIND);
            if (!issues.length) return null;
            return (
              <section key={c.key} className={s.kind} aria-label={t(c.label)}>
                <div className={s.kindHead}>
                  <h3 className={s.kindTitle}>{t(c.label)}</h3>
                  <Link to={searchURL(query.search, c.key)} className={s.more}>{t("search.viewMore")}</Link>
                </div>
                {issues.map((i) => (
                  <IssueCard key={i.id} issue={i} />
                ))}
              </section>
            );
          })}
        </>
      )}
    </div>
  );
}
