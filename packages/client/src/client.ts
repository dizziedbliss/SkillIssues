import type { Transport } from "./transport";
import type {
  AccountDeleted, ActiveTheme, Facets, Issue, IssueBody, IssueFilters, IssuePage, MyXP, Notifications, Profile, ProfileUpdate,
  ReportTarget, RepositoryLookup, SavedChange, SearchCategory, Session, SharedThemes, SignInStart, Stats, Theme, ThemeCheck,
  ThemeInput, TrackedPull, Claim, ClaimState, LearningPath, PathRole,
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
    repo: list(f.repo),
    owner: list(f.owner),
    unassigned: f.unassigned ? "true" : undefined,
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
  return {
    ...i, labels: i.labels ?? [], tech: i.tech ?? [], assignees: i.assignees ?? null,
    contributing_url: i.contributing_url ?? null, claimed: i.claimed ?? false,
  };
}

function withPath(p: LearningPath | null): LearningPath | null {
  return p && { ...p, steps: p.steps.map((s) => ({ ...s, issue: s.issue && withLists(s.issue) })) };
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
  async search(term: string, filters: IssueFilters & { page?: number; category?: SearchCategory } = {}, o?: Opts): Promise<Issue[]> {
    const issues = await this.get<Issue[]>(
      `/issues${query({ ...filterParams(filters), search: term, page: filters.page, category: filters.category })}`,
      o,
    );
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

  /** Replaces the editable profile; refused whole (400, with the reason) when out of bounds. */
  updateProfile(input: ProfileUpdate): Promise<Profile> {
    return this.send({ method: "PUT", path: "/profile", body: input }) as Promise<Profile>;
  }

  /** The languages and frameworks a profile may choose from. */
  async preferenceOptions(o?: Opts): Promise<string[]> {
    return (await this.get<{ options: string[] }>("/preferences/options", o)).options;
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

  /** Whether an issue is claimed, and your claim on it. */
  claimState(issueID: number, o?: Opts): Promise<ClaimState> {
    return this.get(`/issues/${issueID}/claim`, o);
  }

  /** Claims an issue for you; refused when someone else holds it or you hold too many. */
  claim(issueID: number): Promise<Claim> {
    return this.send({ method: "POST", path: `/issues/${issueID}/claim`, body: {} }) as Promise<Claim>;
  }

  async releaseClaim(issueID: number): Promise<void> {
    await this.send({ method: "DELETE", path: `/issues/${issueID}/claim` });
  }

  /** Your active claims, soonest to lapse first, with their issues. */
  async myClaims(o?: Opts): Promise<(Claim & { issue: Issue })[]> {
    const r = await this.get<{ claims: (Claim & { issue: Issue })[] }>("/me/claims", o);
    return r.claims.map((c) => ({ ...c, issue: withLists(c.issue) }));
  }

  /** The roles a learning path can be made of. */
  async pathRoles(o?: Opts): Promise<PathRole[]> {
    return (await this.get<{ roles: PathRole[] }>("/paths/roles", o)).roles;
  }

  /** Your learning path in progress, or null. */
  async myPath(o?: Opts): Promise<LearningPath | null> {
    return withPath((await this.get<{ path: LearningPath | null }>("/me/path", o)).path);
  }

  /** Starts a learning path of a role. */
  async startPath(role: string): Promise<LearningPath> {
    return withPath(((await this.send({ method: "POST", path: "/me/path", body: { role } })) as { path: LearningPath }).path)!;
  }

  /** Swaps a step that isn't done for another issue. */
  async swapPathStep(position: number): Promise<LearningPath> {
    return withPath(((await this.send({ method: "POST", path: `/me/path/steps/${position}/swap`, body: {} })) as { path: LearningPath }).path)!;
  }

  async abandonPath(): Promise<void> {
    await this.send({ method: "DELETE", path: "/me/path" });
  }

  /** Your level, streak, badges and latest XP, and where tracking stands. */
  myXP(o?: Opts): Promise<MyXP> {
    return this.get("/me/xp", o);
  }

  /** Your pull requests to catalog repositories, newest-updated first. */
  async myPulls(state: "open" | "all" = "all", o?: Opts): Promise<TrackedPull[]> {
    return (await this.get<{ pull_requests: TrackedPull[] }>(`/me/pulls${query({ state: state === "all" ? undefined : state })}`, o)).pull_requests;
  }

  /**
   * Asks for your pull requests to be read again soon. Refused (429) within a
   * few minutes of the last ask; resolves to when the next is allowed.
   */
  async refreshPulls(): Promise<string> {
    return ((await this.send({ method: "POST", path: "/me/pulls/refresh", body: {} })) as { next_allowed_at: string }).next_allowed_at;
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

  activeTheme(o?: Opts): Promise<ActiveTheme> {
    return this.get("/me/theme", o);
  }

  /** Apply a theme (your own or a public one); null goes back to the default. */
  setActiveTheme(id: string | null): Promise<ActiveTheme> {
    return this.send({ method: "PUT", path: "/me/theme", body: { id } }) as Promise<ActiveTheme>;
  }

  /** Apply a built-in theme as it is (packages/theme's PRESETS). */
  setActivePreset(preset: string): Promise<ActiveTheme> {
    return this.send({ method: "PUT", path: "/me/theme", body: { preset } }) as Promise<ActiveTheme>;
  }

  /** The community listing: public themes, most used or newest first, optionally by name. */
  publicThemes(opts: { sort?: "popular" | "newest"; search?: string; page?: number } = {}, o?: Opts): Promise<SharedThemes> {
    return this.get(`/themes/public${query({ sort: opts.sort, search: opts.search, page: opts.page })}`, o);
  }

  /**
   * Reports an issue, a repository or a public theme. Refused with ApiError:
   * 409 already reported by you, 404 nothing there, 429 past the day's limit.
   */
  async report(target: ReportTarget, reason: string, note = ""): Promise<void> {
    await this.send({ method: "POST", path: "/reports", body: { type: target.type, id: String(target.id), reason, note } });
  }

  /** Check a theme as a save would, storing nothing: refused with ApiError (422) and its problems. */
  validateTheme(input: { toml: string; css: string }): Promise<ThemeCheck> {
    return this.send({ method: "POST", path: "/themes/validate", body: { ...input, public: false } }) as Promise<ThemeCheck>;
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
