import { useEffect, useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { Issue, IssueFilters } from "@skillissues/client";
import { api } from "../../lib/api";

/** Issues on one page of Explore. */
export const PAGE_SIZE = 20;
/** Requests to make for one page before showing what was found. */
const MAX_REQUESTS = 5;

interface Page {
  issues: Issue[];
  /** Whether there is a page after this one. */
  more: boolean;
  /** Browsing: the cursor of the next page. */
  next: string | null;
}

/**
 * One browse page. A short page with a cursor isn't the end: the server
 * stopped looking to answer quickly. Read on until the page is full, the
 * cursor runs out, or MAX_REQUESTS have gone.
 */
async function browsePage(filters: IssueFilters, cursor: string, signal: AbortSignal): Promise<Page> {
  const issues: Issue[] = [];
  let next: string | null = cursor;
  for (let i = 0; i < MAX_REQUESTS && next !== null && issues.length < PAGE_SIZE; i++) {
    const page = await api.browse({ ...filters, limit: PAGE_SIZE }, next, { signal });
    issues.push(...page.issues);
    next = page.next_cursor;
  }
  return { issues, more: next !== null, next };
}

/**
 * Explore's pages. Browsing pages forward on cursors and remembers the ones
 * seen, so the pager can go back to any of them (a shuffled feed has no
 * page 7 to jump to). A search pages by number. Changing `key` (the search
 * and filters) starts again from page one.
 */
export function useCatalog(key: string, search: string, filters: IssueFilters | null) {
  // furthest: the deepest page reached, so the pager keeps offering it.
  const [state, setState] = useState({ key, cursors: [""], index: 0, furthest: 0 });
  if (state.key !== key) setState({ key, cursors: [""], index: 0, furthest: 0 });
  const index = state.key === key ? state.index : 0;
  const cursor = state.key === key ? state.cursors[index]! : "";

  const query = useQuery({
    queryKey: search ? ["search", search, filters, index] : ["browse", filters, cursor],
    queryFn: async ({ signal }): Promise<Page> => {
      if (!search) return browsePage(filters!, cursor, signal);
      // A search keeps its match order unless a sort was chosen; the shuffle isn't one.
      const sort = filters!.sort === "shuffle" ? undefined : filters!.sort;
      const issues = await api.search(search, { ...filters, sort, limit: PAGE_SIZE, page: index + 1 }, { signal });
      return { issues, more: issues.length === PAGE_SIZE, next: null };
    },
    enabled: filters !== null,
    placeholderData: keepPreviousData,
  });

  // Remember where the next page starts.
  const data = query.data;
  useEffect(() => {
    if (search || !data?.more || query.isPlaceholderData) return;
    setState((s) => {
      if (s.key !== key || s.cursors.length > s.index + 1) return s;
      return { ...s, cursors: [...s.cursors, data.next ?? ""] };
    });
  }, [data, key, search, query.isPlaceholderData]);

  const seen = state.key === key ? state.cursors.length : 1;
  return {
    ...query,
    index,
    /** Pages that can be opened: those seen, and the next one. */
    pages: search ? Math.max(state.key === key ? state.furthest : 0, index) + (data?.more ? 2 : 1) : seen,
    hasNext: !!data?.more && !query.isPlaceholderData,
    go: (i: number) => setState((s) => (s.key === key ? { ...s, index: i, furthest: Math.max(s.furthest, i) } : s)),
  };
}
