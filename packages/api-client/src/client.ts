import { ApiError } from "./errors";
import type { Transport, TransportInit, TransportResponse } from "./transport";
import type {
  Contribution,
  ContributionDetail,
  FreshnessBatchResult,
  FreshnessResult,
  GithubAuthorizeResult,
  HealthDetail,
  Issue,
  IssueBodyPending,
  IssueBodyResult,
  IssueBrowseFilters,
  IssueFilters,
  IssuePage,
  PreferenceOptions,
  Profile,
  ProfileUpdate,
  SessionInfo,
  SubmitContributionResult,
  VerifyResult,
} from "./types";

export interface RequestOptions {
  signal?: AbortSignal;
}

function jsonInit(body: unknown, init?: RequestOptions): TransportInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal: init?.signal,
  };
}

// D15 — recommendations cache: localStorage + ETag revalidation. Both
// renderers run in a Chromium context (web directly, desktop's renderer
// process), so this is safe to share verbatim instead of duplicating it per
// client. The backend already invalidates its own cache (and so the ETag it
// serves) on preference/XP/level/contribution changes
// (InvalidateRecommendations), so revalidating against the stored ETag is
// sufficient — no separate "profile version" needs tracking client-side.
const RECOMMENDATIONS_CACHE_KEY = "skillissues.recommendations.v1";

interface RecommendationsCacheEntry {
  etag: string;
  data: Issue[];
}

function loadRecommendationsCache(): RecommendationsCacheEntry | null {
  try {
    const raw = localStorage.getItem(RECOMMENDATIONS_CACHE_KEY);
    return raw ? (JSON.parse(raw) as RecommendationsCacheEntry) : null;
  } catch {
    return null;
  }
}

function saveRecommendationsCache(etag: string, data: Issue[]) {
  try {
    localStorage.setItem(RECOMMENDATIONS_CACHE_KEY, JSON.stringify({ etag, data }));
  } catch {
    // Storage can be full or disabled (private mode) — the cache is a pure
    // speed optimization, never required for correctness.
  }
}

// Exported so callers can drop the cache on sign-out — recommendations are
// per-user, and leaving a signed-out browser holding the previous user's
// cached feed would (briefly, until the next ETag mismatch) be the wrong
// person's data in localStorage.
export function clearRecommendationsCache(): void {
  try {
    localStorage.removeItem(RECOMMENDATIONS_CACHE_KEY);
  } catch {
    // ignore — same reasoning as above
  }
}

export class ApiClient {
  private readonly transport: Transport;

  constructor(transport: Transport) {
    this.transport = transport;
  }

  // raw exposes the transport response directly — for the handful of
  // callers (recommendations' ETag revalidation) that need status/etag
  // alongside the body rather than just the typed payload.
  async raw(path: string, init?: TransportInit): Promise<TransportResponse> {
    return this.transport(path, init);
  }

  private async send<T>(path: string, init?: TransportInit): Promise<T> {
    const res = await this.transport(path, init);
    if (!res.ok) {
      throw new ApiError(res.status, res.detail ?? "");
    }
    return res.data as T;
  }

  // ----- Issues -----

  async getIssues(filters: IssueFilters = {}, opts?: RequestOptions): Promise<Issue[]> {
    const params = new URLSearchParams();
    if (filters.limit) params.set("limit", String(filters.limit));
    if (filters.page) params.set("page", String(filters.page));
    if (filters.difficulty && filters.difficulty !== "ALL") params.set("difficulty", filters.difficulty);
    if (filters.language) params.set("language", filters.language);
    if (filters.search) params.set("search", filters.search);
    const qs = params.toString();
    return this.send<Issue[]>(`/issues${qs ? `?${qs}` : ""}`, { signal: opts?.signal });
  }

  // getIssuesPage browses the catalog by cursor: every page costs the server
  // the same, however deep, unlike `page`. Pass each page's nextCursor back
  // for the next one until it is null.
  async getIssuesPage(filters: IssueBrowseFilters = {}, opts?: RequestOptions): Promise<IssuePage> {
    const params = new URLSearchParams();
    if (filters.limit) params.set("limit", String(filters.limit));
    if (filters.difficulty && filters.difficulty !== "ALL") params.set("difficulty", filters.difficulty);
    if (filters.language) params.set("language", filters.language);
    // Present even when empty: an empty cursor asks for the first page in
    // the cursor response shape.
    params.set("cursor", filters.cursor ?? "");
    const res = await this.send<{ issues: Issue[]; next_cursor: string | null }>(`/issues?${params.toString()}`, {
      signal: opts?.signal,
    });
    return { issues: res.issues, nextCursor: res.next_cursor };
  }

  async getIssue(id: number, opts?: RequestOptions): Promise<Issue> {
    return this.send<Issue>(`/issues/${id}`, { signal: opts?.signal });
  }

  // getIssueBody is the server's body fallback: a cached body, or one the
  // server fetches with its own GitHub budget, capped per caller (429 past
  // it). Prefer loadIssueBody, which only falls back to this when GitHub has
  // refused the caller itself.
  async getIssueBody(id: number, opts?: RequestOptions): Promise<IssueBodyResult | IssueBodyPending> {
    return this.send<IssueBodyResult | IssueBodyPending>(`/issues/${id}/body`, { signal: opts?.signal });
  }

  // loadIssueBody reads an issue's body from GitHub directly, and asks the
  // server only when GitHub answers that the caller's own rate limit is spent
  // (403/429 with x-ratelimit-remaining: 0) — the server's budget is for users
  // who have no other way. Any other GitHub failure is thrown as an ApiError
  // with GitHub's status. githubFetch lets a client route the GitHub call its
  // own way (desktop, with its user token); it defaults to fetch.
  async loadIssueBody(
    issue: Pick<Issue, "id" | "repository" | "number">,
    opts?: RequestOptions & { githubFetch?: typeof fetch },
  ): Promise<IssueBodyResult | IssueBodyPending> {
    const doFetch = opts?.githubFetch ?? fetch;
    const res = await doFetch(`https://api.github.com/repos/${issue.repository}/issues/${issue.number}`, {
      headers: { Accept: "application/vnd.github+json" },
      signal: opts?.signal,
    });
    if (res.ok) {
      const data = (await res.json()) as { body?: string | null };
      return { body: data.body ?? "" };
    }
    if ((res.status === 403 || res.status === 429) && res.headers.get("x-ratelimit-remaining") === "0") {
      return this.getIssueBody(issue.id, opts);
    }
    throw new ApiError(res.status, `GitHub answered ${res.status} for ${issue.repository}#${issue.number}`);
  }

  async checkIssueFreshness(id: number, opts?: RequestOptions): Promise<FreshnessResult> {
    return this.send<FreshnessResult>(`/issues/${id}/freshness`, { method: "POST", signal: opts?.signal });
  }

  async checkIssuesFreshnessBatch(issueIds: number[], opts?: RequestOptions): Promise<FreshnessBatchResult> {
    return this.send<FreshnessBatchResult>("/issues/freshness", jsonInit({ issue_ids: issueIds }, opts));
  }

  // ----- Saved issues -----

  async getSaved(opts?: RequestOptions): Promise<Issue[]> {
    return this.send<Issue[]>("/saved", { signal: opts?.signal });
  }

  async saveIssue(issueId: number, opts?: RequestOptions): Promise<void> {
    await this.send("/saved", jsonInit({ issue_id: issueId }, opts));
  }

  async unsaveIssue(issueId: number, opts?: RequestOptions): Promise<void> {
    await this.send(`/saved/${issueId}`, { method: "DELETE", signal: opts?.signal });
  }

  // ----- Recommendations -----

  // On a genuine failure (network blip, 5xx, the unauthenticated rate
  // limiter) this falls back to the last cached copy and reports it as
  // `stale` rather than throwing straight into an empty feed — the same
  // "keep previous data, flag it, never silently empty" rule as everywhere
  // else. It only throws when there is no cache to fall back to, so the
  // caller still has something to show the user (a retry affordance) rather
  // than an unexplained blank list.
  async getRecommendations(
    filters: IssueFilters = {},
    opts?: RequestOptions,
  ): Promise<{ data: Issue[]; stale: boolean }> {
    const params = new URLSearchParams();
    if (filters.limit) params.set("limit", String(filters.limit));
    if (filters.page) params.set("page", String(filters.page));
    if (filters.difficulty && filters.difficulty !== "ALL") params.set("difficulty", filters.difficulty);
    const qs = params.toString();
    // The ETag cache is a single slot shared across every filter combo, same
    // as the desktop client it was ported from — switching difficulty/page
    // just means the stored ETag won't match and the server returns a fresh
    // 200 instead of a 304, never stale data.
    const cached = loadRecommendationsCache();
    try {
      const res = await this.raw(`/recommendations${qs ? `?${qs}` : ""}`, {
        signal: opts?.signal,
        allow304: Boolean(cached),
        returnEtag: true,
        headers: cached ? { "If-None-Match": cached.etag } : undefined,
      });

      if (!res.ok) {
        throw new ApiError(res.status, res.detail ?? "");
      }
      if (res.notModified && cached) {
        return { data: cached.data, stale: false };
      }

      const data = res.data as Issue[];
      if (res.etag) saveRecommendationsCache(res.etag, data);
      return { data, stale: false };
    } catch (err) {
      if (cached) {
        return { data: cached.data, stale: true };
      }
      throw err;
    }
  }

  // ----- Profile -----

  async getMe(opts?: RequestOptions): Promise<Profile> {
    return this.send<Profile>("/me", { signal: opts?.signal });
  }

  async updateProfile(update: ProfileUpdate, opts?: RequestOptions): Promise<Profile> {
    return this.send<Profile>("/profile", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(update),
      signal: opts?.signal,
    });
  }

  async getPreferenceOptions(opts?: RequestOptions): Promise<PreferenceOptions> {
    return this.send<PreferenceOptions>("/preferences/options", { signal: opts?.signal });
  }

  // ----- Contributions -----

  async startContribution(issueId: number, opts?: RequestOptions): Promise<Contribution> {
    return this.send<Contribution>("/contributions", jsonInit({ issue_id: issueId }, opts));
  }

  async listContributions(opts?: RequestOptions): Promise<ContributionDetail[]> {
    return this.send<ContributionDetail[]>("/contributions", { signal: opts?.signal });
  }

  async getWorking(opts?: RequestOptions): Promise<ContributionDetail[]> {
    return this.send<ContributionDetail[]>("/working", { signal: opts?.signal });
  }

  async getContribution(id: number, opts?: RequestOptions): Promise<Contribution> {
    return this.send<Contribution>(`/contributions/${id}`, { signal: opts?.signal });
  }

  async attachPullRequest(
    id: number,
    prUrl: string,
    prNumber: number,
    opts?: RequestOptions,
  ): Promise<Contribution> {
    return this.send<Contribution>(`/contributions/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pr_url: prUrl, pr_number: prNumber }),
      signal: opts?.signal,
    });
  }

  async submitContribution(
    id: number,
    branch: string,
    base?: string,
    opts?: RequestOptions,
  ): Promise<SubmitContributionResult> {
    return this.send<SubmitContributionResult>(`/contributions/${id}/submit`, jsonInit({ branch, base }, opts));
  }

  async verifyContribution(id: number, opts?: RequestOptions): Promise<VerifyResult> {
    return this.send<VerifyResult>(`/contributions/${id}/verify`, { method: "POST", signal: opts?.signal });
  }

  // ----- GitHub -----

  async forkRepository(repository: string, opts?: RequestOptions): Promise<Record<string, unknown>> {
    return this.send(`/github/forks`, jsonInit({ repository }, opts));
  }

  async getGitToken(opts?: RequestOptions): Promise<{ token: string }> {
    return this.send<{ token: string }>("/github/git-token", { signal: opts?.signal });
  }

  // ----- Session -----

  async getSession(opts?: RequestOptions): Promise<SessionInfo> {
    return this.send<SessionInfo>("/auth/session", { signal: opts?.signal });
  }

  // GET /auth/github — begins the OAuth/GitHub App login flow. enabled:false
  // means the server has no GITHUB_CLIENT_ID configured; callers must show
  // `error`, never fall back to a token-entry form.
  async getGithubAuthorize(opts?: RequestOptions): Promise<GithubAuthorizeResult> {
    return this.send<GithubAuthorizeResult>("/auth/github", { signal: opts?.signal });
  }

  async logout(opts?: RequestOptions): Promise<void> {
    await this.send("/auth/logout", { method: "POST", signal: opts?.signal });
  }

  async purgeAccount(opts?: RequestOptions): Promise<void> {
    await this.send("/auth/purge", { method: "POST", signal: opts?.signal });
  }

  // ----- Health -----

  async getHealthDetail(opts?: RequestOptions): Promise<HealthDetail> {
    return this.send<HealthDetail>("/health/detail", { signal: opts?.signal });
  }
}

export function createApiClient(transport: Transport): ApiClient {
  return new ApiClient(transport);
}
