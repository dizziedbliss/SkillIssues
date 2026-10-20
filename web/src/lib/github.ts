/*
 * A person's public pull requests, read in the browser from GitHub's search
 * API with no token: the server holds no user token and doesn't need one for
 * public data. GitHub allows a browser ten searches a minute without one, so
 * a person's summary costs three searches and is kept in this browser for
 * CACHE_MS. When GitHub says no, the reason and when to try again come back
 * as an error, never as zeros.
 */

const API = "https://api.github.com/search/issues";
const CACHE_MS = 30 * 60_000;
const CACHE_PREFIX = "si.gh.";
/** Search pages read for the year's activity: 100 pull requests each. */
const YEAR_PAGES = 3;

export interface PullRequest {
  title: string;
  number: number;
  url: string;
  /** owner/name */
  repository: string;
  state: "open" | "closed";
  merged: boolean;
  created_at: string;
  closed_at: string | null;
}

export interface Contributions {
  login: string;
  /** Every public pull request, all time. */
  total: number;
  /** Merged ones, all time. */
  merged: number;
  /** Distinct repositories among the merged pull requests read (up to 100). */
  repositories: number;
  /** Whether `repositories` saw every merged pull request. */
  repositoriesComplete: boolean;
  /** The latest pull requests, newest first. */
  recent: PullRequest[];
  /** The latest merged pull requests, newest first. */
  recentMerged: PullRequest[];
  /** Pull requests opened per day over the last year (YYYY-MM-DD → count). */
  days: Record<string, number>;
  /** Whether `days` saw every pull request of the year. */
  daysComplete: boolean;
  fetched_at: number;
}

export class GitHubLimitError extends Error {
  /** When GitHub will answer again (ms since epoch), when it said. */
  readonly resetAt: number | null;
  constructor(message: string, resetAt: number | null) {
    super(message);
    this.name = "GitHubLimitError";
    this.resetAt = resetAt;
  }
}

interface SearchItem {
  title: string;
  number: number;
  html_url: string;
  repository_url: string;
  state: "open" | "closed";
  created_at: string;
  closed_at: string | null;
  pull_request?: { merged_at: string | null };
}

interface SearchPage {
  total_count: number;
  items: SearchItem[];
}

async function search(q: string, perPage: number, page: number, signal?: AbortSignal): Promise<SearchPage> {
  const url = `${API}?q=${encodeURIComponent(q)}&sort=created&order=desc&per_page=${perPage}&page=${page}`;
  const res = await fetch(url, { headers: { Accept: "application/vnd.github+json" }, credentials: "omit", signal });
  if (res.ok) return (await res.json()) as SearchPage;
  if (res.status === 403 || res.status === 429) {
    const reset = Number(res.headers.get("x-ratelimit-reset"));
    throw new GitHubLimitError("GitHub is limiting how often this browser can search.", reset ? reset * 1000 : null);
  }
  // 422: the login doesn't exist on GitHub (renamed or deleted).
  throw new Error(res.status === 422 ? "GitHub doesn’t know this account’s username any more." : `GitHub answered ${res.status}.`);
}

function toPR(i: SearchItem): PullRequest {
  return {
    title: i.title,
    number: i.number,
    url: i.html_url,
    repository: i.repository_url.replace("https://api.github.com/repos/", ""),
    state: i.state,
    merged: !!i.pull_request?.merged_at,
    created_at: i.created_at,
    closed_at: i.closed_at,
  };
}

const day = (iso: string) => iso.slice(0, 10);

function readCache(login: string): Contributions | null {
  try {
    const raw = localStorage.getItem(CACHE_PREFIX + login.toLowerCase());
    if (!raw) return null;
    const c = JSON.parse(raw) as Contributions;
    return Date.now() - c.fetched_at < CACHE_MS ? c : null;
  } catch {
    return null;
  }
}

function writeCache(c: Contributions) {
  try {
    localStorage.setItem(CACHE_PREFIX + c.login.toLowerCase(), JSON.stringify(c));
  } catch {
    // Storage full or blocked: the next visit asks GitHub again.
  }
}

/** A login GitHub would accept: letters, digits and single hyphens. Anything else never reaches the query. */
const LOGIN = /^[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$/;

/** `login`'s public pull requests, summarised. `fresh` skips this browser's copy. */
export async function loadContributions(login: string, signal?: AbortSignal, fresh = false): Promise<Contributions> {
  if (!LOGIN.test(login)) throw new Error("That isn’t a GitHub username.");
  if (!fresh) {
    const cached = readCache(login);
    if (cached) return cached;
  }
  const since = new Date(Date.now() - 365 * 24 * 3600 * 1000).toISOString().slice(0, 10);
  const [all, merged, year] = await Promise.all([
    search(`type:pr author:${login}`, 20, 1, signal),
    search(`type:pr author:${login} is:merged`, 100, 1, signal),
    search(`type:pr author:${login} created:>=${since}`, 100, 1, signal),
  ]);

  const yearItems = [...year.items];
  for (let page = 2; page <= YEAR_PAGES && yearItems.length < year.total_count; page++) {
    yearItems.push(...(await search(`type:pr author:${login} created:>=${since}`, 100, page, signal)).items);
  }
  const days: Record<string, number> = {};
  for (const i of yearItems) days[day(i.created_at)] = (days[day(i.created_at)] ?? 0) + 1;

  const c: Contributions = {
    login,
    total: all.total_count,
    merged: merged.total_count,
    repositories: new Set(merged.items.map((i) => i.repository_url)).size,
    repositoriesComplete: merged.items.length >= merged.total_count,
    recent: all.items.map(toPR),
    recentMerged: merged.items.slice(0, 20).map(toPR),
    days,
    daysComplete: yearItems.length >= year.total_count,
    fetched_at: Date.now(),
  };
  writeCache(c);
  return c;
}
