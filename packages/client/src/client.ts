import type { Transport } from "./transport";
import type {
  Facets, Issue, IssueFilters, IssuePage, RepositoryLookup, Stats, Theme, ThemeInput,
} from "./types";

function query(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

function filterParams(f: IssueFilters): Record<string, string | number | undefined> {
  const list = (v?: string[]) => (v && v.length ? v.join(",") : undefined);
  return {
    sort: f.sort,
    difficulty: list(f.difficulty),
    language: list(f.language),
    labels: list(f.labels),
    topics: list(f.topics),
    tech: list(f.tech),
    comments_min: f.comments_min,
    comments_max: f.comments_max,
    limit: f.limit,
  };
}

type Opts = { signal?: AbortSignal };

/** One typed method per API endpoint. */
export class Client {
  constructor(private readonly send: Transport) {}

  private get<T>(path: string, o?: Opts): Promise<T> {
    return this.send({ method: "GET", path, signal: o?.signal }) as Promise<T>;
  }

  /** Live catalog counts. Public and edge-cached for a few seconds. */
  stats(o?: Opts): Promise<Stats> {
    return this.get("/stats", o);
  }

  /** One page of the catalog; pass the previous page's next_cursor, or "" for the first. */
  browse(filters: IssueFilters, cursor = "", o?: Opts): Promise<IssuePage> {
    // ?cursor= must be present even when empty: it is what selects cursor
    // paging (and the shuffled feed) over page numbers.
    const q = query(filterParams(filters));
    return this.get(`/issues${q}${q ? "&" : "?"}cursor=${encodeURIComponent(cursor)}`, o);
  }

  /** Issues matching a search term, best category first. */
  search(term: string, filters: IssueFilters & { page?: number } = {}, o?: Opts): Promise<Issue[]> {
    return this.get(`/issues${query({ ...filterParams(filters), search: term, page: filters.page })}`, o);
  }

  facets(o?: Opts): Promise<Facets> {
    return this.get("/issues/facets", o);
  }

  lookupRepository(name: string, o?: Opts): Promise<RepositoryLookup> {
    return this.get(`/repositories/lookup${query({ name })}`, o);
  }

  themes(o?: Opts): Promise<Theme[]> {
    return this.get("/themes", o);
  }

  theme(id: string, o?: Opts): Promise<Theme> {
    return this.get(`/themes/${encodeURIComponent(id)}`, o);
  }

  createTheme(input: ThemeInput): Promise<Theme> {
    return this.send({ method: "POST", path: "/themes", body: input }) as Promise<Theme>;
  }

  updateTheme(id: string, input: ThemeInput): Promise<Theme> {
    return this.send({ method: "PUT", path: `/themes/${encodeURIComponent(id)}`, body: input }) as Promise<Theme>;
  }

  async deleteTheme(id: string): Promise<void> {
    await this.send({ method: "DELETE", path: `/themes/${encodeURIComponent(id)}` });
  }

  activeTheme(o?: Opts): Promise<Theme | null> {
    return this.get("/me/theme", o);
  }

  /** Apply a theme (your own or a public one); null goes back to the default. */
  setActiveTheme(id: string | null): Promise<Theme | null> {
    return this.send({ method: "PUT", path: "/me/theme", body: { id } }) as Promise<Theme | null>;
  }
}
