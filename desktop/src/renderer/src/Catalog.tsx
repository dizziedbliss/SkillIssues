import { useEffect, useState, type FormEvent } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { Difficulty, Issue, Sort } from "@skillissues/client";
import { api } from "./bridge";

/** Empty pages to read on its own before saying nothing matches (a short page with a cursor is not the end). */
const AUTO_PAGES = 5;

const difficulties: { value: Difficulty | ""; label: string }[] = [
  { value: "", label: "Any difficulty" },
  { value: "BEGINNER", label: "Beginner" },
  { value: "INTERMEDIATE", label: "Intermediate" },
  { value: "ADVANCED", label: "Advanced" },
  { value: "UNCLASSIFIED", label: "Unrated" },
];

export function Catalog({ selected, onSelect }: { selected: number | null; onSelect: (issue: Issue) => void }) {
  const [sort, setSort] = useState<Sort>("shuffle");
  const [difficulty, setDifficulty] = useState<Difficulty | "">("");
  const [language, setLanguage] = useState("");
  const [draft, setDraft] = useState("");
  const [term, setTerm] = useState("");

  const facets = useQuery({ queryKey: ["facets"], queryFn: () => api.facets(), staleTime: 30 * 60_000 });
  const filters = {
    sort,
    difficulty: difficulty ? [difficulty] : undefined,
    language: language ? [language] : undefined,
    limit: 30,
  };
  const browse = useInfiniteQuery({
    queryKey: ["browse", filters],
    queryFn: ({ pageParam }) => api.browse(filters, pageParam),
    initialPageParam: "",
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    enabled: term === "",
  });
  const search = useQuery({
    queryKey: ["search", term, filters],
    queryFn: () => api.search(term, { ...filters, sort: undefined }),
    enabled: term !== "",
  });

  const issues: Issue[] = term ? search.data ?? [] : browse.data?.pages.flatMap((p) => p.issues) ?? [];
  const error = term ? search.error : browse.error;
  const pages = browse.data?.pages.length ?? 0;
  const stillLooking = !term && !browse.error && issues.length === 0 && browse.hasNextPage && pages < AUTO_PAGES;
  useEffect(() => {
    if (stillLooking && !browse.isFetchingNextPage) void browse.fetchNextPage();
  }, [stillLooking, browse]);
  const loading = term ? search.isPending : browse.isPending || stillLooking;

  function submit(e: FormEvent) {
    e.preventDefault();
    setTerm(draft.trim());
  }

  return (
    <section className="catalog" aria-label="Open issues">
      <form className="filters" onSubmit={submit} role="search">
        <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="Search repository, title, language or author" aria-label="Search" />
        <div className="row">
          <select value={sort} onChange={(e) => setSort(e.target.value as Sort)} aria-label="Sort" disabled={term !== ""}>
            <option value="shuffle">Shuffled</option>
            <option value="easiest">Easiest first</option>
            <option value="newest">Newest</option>
          </select>
          <select value={difficulty} onChange={(e) => setDifficulty(e.target.value as Difficulty | "")} aria-label="Difficulty">
            {difficulties.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
          </select>
          <select value={language} onChange={(e) => setLanguage(e.target.value)} aria-label="Language">
            <option value="">Any language</option>
            {(facets.data?.languages ?? []).map((f) => <option key={f.value} value={f.value}>{f.value}</option>)}
          </select>
          {term && <button type="button" className="link" onClick={() => (setTerm(""), setDraft(""))}>Clear search</button>}
        </div>
      </form>

      {error && <p className="error" role="alert">{error.message}</p>}
      {loading && <p className="quiet">Loading issues…</p>}
      {!loading && !error && issues.length === 0 && <p className="quiet">No open issues match. Try another language or difficulty.</p>}

      <ol className="issues">
        {issues.map((it) => (
          <li key={it.id}>
            <button type="button" className={`issue${selected === it.id ? " active" : ""}`} onClick={() => onSelect(it)} aria-pressed={selected === it.id}>
              <span className="repo">{it.repository} #{it.number}</span>
              <span className="title">{it.title}</span>
              <span className="meta">
                {[it.language, it.difficulty === "UNCLASSIFIED" ? "unrated" : it.difficulty.toLowerCase(), `${it.comments} comments`]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
            </button>
          </li>
        ))}
      </ol>
      {!term && browse.hasNextPage && issues.length > 0 && (
        <button type="button" className="button" onClick={() => void browse.fetchNextPage()} disabled={browse.isFetchingNextPage}>
          {browse.isFetchingNextPage ? "Loading…" : "Show more"}
        </button>
      )}
    </section>
  );
}
