import { useEffect, useState, type FormEvent } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { Difficulty, Issue, Sort } from "@skillissues/client";
import { api } from "../lib/api";
import { useT, type MessageKey } from "../i18n";
import s from "./Explore.module.css";

/** Empty pages to read on its own before saying nothing matches. */
const AUTO_PAGES = 5;

/*
 * A plain, working catalog while Explore's design is built: browse the
 * shuffled feed, sort, filter by difficulty and language, search when you
 * press Search.
 */

const sorts: { value: Sort; label: MessageKey }[] = [
  { value: "shuffle", label: "catalog.sort.shuffle" },
  { value: "easiest", label: "catalog.sort.easiest" },
  { value: "newest", label: "catalog.sort.newest" },
];

const difficulties: Difficulty[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED", "UNCLASSIFIED"];

export function Explore() {
  const t = useT();
  const [sort, setSort] = useState<Sort>("shuffle");
  const [difficulty, setDifficulty] = useState<Difficulty | "">("");
  const [language, setLanguage] = useState("");
  const [draft, setDraft] = useState("");
  const [term, setTerm] = useState("");

  const facets = useQuery({ queryKey: ["facets"], queryFn: ({ signal }) => api.facets({ signal }), staleTime: 30 * 60_000 });
  const filters = {
    sort,
    difficulty: difficulty ? [difficulty] : undefined,
    language: language ? [language] : undefined,
    limit: 30,
  };

  const browse = useInfiniteQuery({
    queryKey: ["browse", filters],
    queryFn: ({ pageParam, signal }) => api.browse(filters, pageParam, { signal }),
    initialPageParam: "",
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    enabled: term === "",
  });
  const search = useQuery({
    queryKey: ["search", term, filters],
    queryFn: ({ signal }) => api.search(term, { ...filters, sort: undefined }, { signal }),
    enabled: term !== "",
  });

  const issues: Issue[] = term ? search.data ?? [] : browse.data?.pages.flatMap((p) => p.issues) ?? [];
  const error = term ? search.error : browse.error;

  // A page can come back short, even empty, with a cursor: the server
  // stopped looking to answer quickly, not because it ran out. Keep looking
  // a few pages before saying nothing matches.
  const pagesFetched = browse.data?.pages.length ?? 0;
  const stillLooking = !term && !browse.error && issues.length === 0 && browse.hasNextPage && pagesFetched < AUTO_PAGES;
  useEffect(() => {
    if (stillLooking && !browse.isFetchingNextPage) void browse.fetchNextPage();
  }, [stillLooking, browse]);
  const loading = term ? search.isPending : browse.isPending || stillLooking;

  function submit(e: FormEvent) {
    e.preventDefault();
    setTerm(draft.trim());
  }

  return (
    <>
      <p className={s.notice} role="note">{t("catalog.notice")}</p>
      <form className={s.controls} onSubmit={submit} role="search">
        <label className={s.searchField}>
          <span className="visually-hidden">{t("catalog.searchLabel")}</span>
          <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={t("catalog.searchPlaceholder")} />
        </label>
        <button type="submit" className={s.button}>{t("catalog.search")}</button>
        {term && (
          <button type="button" className={s.textButton} onClick={() => { setDraft(""); setTerm(""); }}>
            {t("catalog.clear")}
          </button>
        )}
      </form>
      <div className={s.filters}>
        <label>
          <span>{t("catalog.sort")}</span>
          <select value={sort} onChange={(e) => setSort(e.target.value as Sort)} disabled={term !== ""}>
            {sorts.map((o) => <option key={o.value} value={o.value}>{t(o.label)}</option>)}
          </select>
        </label>
        <label>
          <span>{t("catalog.difficulty")}</span>
          <select value={difficulty} onChange={(e) => setDifficulty(e.target.value as Difficulty | "")}>
            <option value="">{t("catalog.difficulty.any")}</option>
            {difficulties.map((d) => <option key={d} value={d}>{t(`catalog.difficulty.${d}`)}</option>)}
          </select>
        </label>
        <label>
          <span>{t("catalog.language")}</span>
          <select value={language} onChange={(e) => setLanguage(e.target.value)}>
            <option value="">{t("catalog.language.any")}</option>
            {(facets.data?.languages ?? []).map((f) => <option key={f.value} value={f.value}>{f.value}</option>)}
          </select>
        </label>
      </div>

      {error && <p className={s.error} role="alert">{error.message}</p>}
      {loading && <p className={s.quiet}>{t("catalog.loading")}</p>}
      {!loading && !error && issues.length === 0 && <p className={s.quiet}>{t("catalog.empty")}</p>}

      <ol className={s.list}>
        {issues.map((it) => (
          <li key={it.id} className={s.item}>
            <a href={it.url} target="_blank" rel="noreferrer" className={s.itemLink}>
              <span className={s.repo}>{it.repository} #{it.number}</span>
              <span className={s.title}>{it.title}</span>
              <span className={s.meta}>
                {[it.language, t(`catalog.difficulty.${it.difficulty}`), t("catalog.comments", { count: it.comments })]
                  .filter(Boolean)
                  .join(", ")}
              </span>
            </a>
          </li>
        ))}
      </ol>

      {!term && browse.hasNextPage && (
        <button type="button" className={s.button} onClick={() => browse.fetchNextPage()} disabled={browse.isFetchingNextPage}>
          {browse.isFetchingNextPage ? t("catalog.loadingMore") : t("catalog.more")}
        </button>
      )}
    </>
  );
}
