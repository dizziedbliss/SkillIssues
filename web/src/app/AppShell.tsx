import { useEffect, useState, type FormEvent } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { Difficulty, Issue, Sort } from "@skillissues/client";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import { api } from "../lib/api";
import { Account } from "./Account";
import s from "./AppShell.module.css";

/** Empty pages to read on its own before saying nothing matches. */
const AUTO_PAGES = 5;

/*
 * A plain, working catalog for /app while the signed-in app's designs are on
 * their way: browse the shuffled feed, sort, filter by difficulty and
 * language, search when you press Search. It is deliberately simple and
 * will be replaced by the designed app.
 */

const sorts: { value: Sort; label: string }[] = [
  { value: "shuffle", label: "Shuffled" },
  { value: "easiest", label: "Easiest first" },
  { value: "newest", label: "Newest" },
];

const difficulties: { value: Difficulty | ""; label: string }[] = [
  { value: "", label: "Any difficulty" },
  { value: "BEGINNER", label: "Beginner" },
  { value: "INTERMEDIATE", label: "Intermediate" },
  { value: "ADVANCED", label: "Advanced" },
  { value: "UNCLASSIFIED", label: "Unrated" },
];

export default function AppShell() {
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
    <div className={tokens.page}>
      <Header />
      <main className={s.main}>
        <Account />
        <p className={s.notice} role="note">
          You’re seeing a simple version of the catalog while the full app is being designed.
        </p>
        <form className={s.controls} onSubmit={submit} role="search">
          <label className={s.searchField}>
            <span className="visually-hidden">Search issues</span>
            <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="Search by repository, title, language or author" />
          </label>
          <button type="submit" className={s.button}>Search</button>
          {term && (
            <button type="button" className={s.textButton} onClick={() => { setDraft(""); setTerm(""); }}>
              Clear search
            </button>
          )}
        </form>
        <div className={s.filters}>
          <label>
            <span>Sort</span>
            <select value={sort} onChange={(e) => setSort(e.target.value as Sort)} disabled={term !== ""}>
              {sorts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <label>
            <span>Difficulty</span>
            <select value={difficulty} onChange={(e) => setDifficulty(e.target.value as Difficulty | "")}>
              {difficulties.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <label>
            <span>Language</span>
            <select value={language} onChange={(e) => setLanguage(e.target.value)}>
              <option value="">Any language</option>
              {(facets.data?.languages ?? []).map((f) => <option key={f.value} value={f.value}>{f.value}</option>)}
            </select>
          </label>
        </div>

        {error && <p className={s.error} role="alert">{error.message}</p>}
        {loading && <p className={s.quiet}>Loading issues…</p>}
        {!loading && !error && issues.length === 0 && (
          <p className={s.quiet}>No open issues match. Try another language or difficulty.</p>
        )}

        <ol className={s.list}>
          {issues.map((it) => (
            <li key={it.id} className={s.item}>
              <a href={it.url} target="_blank" rel="noreferrer" className={s.itemLink}>
                <span className={s.repo}>{it.repository} #{it.number}</span>
                <span className={s.title}>{it.title}</span>
                <span className={s.meta}>
                  {[it.language, it.difficulty === "UNCLASSIFIED" ? "unrated" : it.difficulty.toLowerCase(), `${it.comments} comments`]
                    .filter(Boolean)
                    .join(", ")}
                </span>
              </a>
            </li>
          ))}
        </ol>

        {!term && browse.hasNextPage && (
          <button type="button" className={s.button} onClick={() => browse.fetchNextPage()} disabled={browse.isFetchingNextPage}>
            {browse.isFetchingNextPage ? "Loading" : "Show more issues"}
          </button>
        )}
      </main>
    </div>
  );
}
