import type { Transport } from "./transport";
import type {
  AccountDeleted, Facets, Issue, IssueBody, IssueFilters, IssuePage, Notifications, Profile, RepositoryLookup,
  SavedChange, Session, SignInStart, Stats, Theme, ThemeInput,
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

/**
 * labels and tech arrived after the rest of an issue: a response cached before
 * then (by the edge, or the API's own cache) has neither, which means none.
 */
function withLists(i: Issue): Issue {
  return { ...i, labels: i.labels ?? [], tech: i.tech ?? [] };
}

/** One typed method per API endpoint. */
export class Client {
  constructor(private readonly send: Transport) {}

  private get<T>(path: string, o?: Opts): Promise<T> {
    return this.send({ method: "GET", path, signal: o?.signal }) as Promise<T>;
  }

  /** Live catalog counts. Public and edge-cached for a few seconds, so sent without the session cookie. */
  stats(o?: Opts): Promise<Stats> {
    return this.send({ method: "GET", path: "/stats", signal: o?.signal, anonymous: true }) as Promise<Stats>;
  }

  /** One page of the catalog; pass the previous page's next_cursor, or "" for the first. */
  async browse(filters: IssueFilters, cursor = "", o?: Opts): Promise<IssuePage> {
    // ?cursor= must be present even when empty: it is what selects cursor
    // paging (and the shuffled feed) over page numbers.
    const q = query(filterParams(filters));
    const page = await this.get<IssuePage>(`/issues${q}${q ? "&" : "?"}cursor=${encodeURIComponent(cursor)}`, o);
    return { ...page, issues: page.issues.map(withLists) };
  }

  /** Issues matching a search term, best category first. */
  async search(term: string, filters: IssueFilters & { page?: number } = {}, o?: Opts): Promise<Issue[]> {
    const issues = await this.get<Issue[]>(`/issues${query({ ...filterParams(filters), search: term, page: filters.page })}`, o);
    return issues.map(withLists);
  }

  /** One issue by SkillIssues' id. */
  async issue(id: number, o?: Opts): Promise<Issue> {
    return withLists(await this.get<Issue>(`/issues/${id}`, o));
  }

  /**
   * An issue's body from the server's cache, for when GitHub refuses the
   * browser's own request (its rate limit). Capped per caller; may answer
   * that it is queued instead.
   */
  issueBody(id: number, o?: Opts): Promise<IssueBody> {
    return this.get(`/issues/${id}/body`, o);
  }

  /** The signed-in person's saved issues, closed ones included. */
  async saved(o?: Opts): Promise<Issue[]> {
    return (await this.get<Issue[]>("/saved", o)).map(withLists);
  }

  save(issueID: number): Promise<SavedChange> {
    return this.send({ method: "POST", path: "/saved", body: { issue_id: issueID } }) as Promise<SavedChange>;
  }

  unsave(issueID: number): Promise<SavedChange> {
    return this.send({ method: "DELETE", path: `/saved/${issueID}`, body: {} }) as Promise<SavedChange>;
  }

  /** The signed-in person's profile. */
  me(o?: Opts): Promise<Profile> {
    return this.get("/me", o);
  }

  /** The signed-in person's latest notifications and how many are unread. */
  notifications(o?: Opts): Promise<Notifications> {
    return this.get("/notifications", o);
  }

  async readNotification(id: number): Promise<void> {
    await this.send({ method: "POST", path: `/notifications/${id}/read`, body: {} });
  }

  async readAllNotifications(): Promise<void> {
    await this.send({ method: "POST", path: "/notifications/read-all", body: {} });
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
    // An empty JSON body: the API refuses a cookie-authenticated mutation without JSON.
    await this.send({ method: "DELETE", path: `/themes/${encodeURIComponent(id)}`, body: {} });
  }

  activeTheme(o?: Opts): Promise<Theme | null> {
    return this.get("/me/theme", o);
  }

  /** Apply a theme (your own or a public one); null goes back to the default. */
  setActiveTheme(id: string | null): Promise<Theme | null> {
    return this.send({ method: "PUT", path: "/me/theme", body: { id } }) as Promise<Theme | null>;
  }

  /** Who is signed in, if anyone. */
  session(o?: Opts): Promise<Session> {
    return this.get("/auth/session", o);
  }

  /**
   * Starts a GitHub sign-in: sets the state cookie that binds the callback to
   * this browser and returns the authorize URL to send the browser to. The
   * callback lands back on the site with ?auth=ok, or ?error=<code>.
   */
  beginSignIn(): Promise<SignInStart> {
    return this.get("/auth/github");
  }

  /** Ends this session. Sent with a JSON body: the API refuses a cookie-authenticated mutation without one. */
  async signOut(): Promise<void> {
    await this.send({ method: "POST", path: "/auth/logout", body: {} });
  }

  /** Ends every session of the signed-in account, on every device. */
  async signOutEverywhere(): Promise<void> {
    await this.send({ method: "POST", path: "/auth/logout-all", body: {} });
  }

  /** Deletes the signed-in account and everything tied to it. Cannot be undone. */
  deleteAccount(): Promise<AccountDeleted> {
    return this.send({ method: "POST", path: "/auth/purge", body: { confirm: true } }) as Promise<AccountDeleted>;
  }
}
