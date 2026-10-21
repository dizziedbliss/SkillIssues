import type { Difficulty, IssueFilters, Sort } from "@skillissues/client";

/*
 * Explore's state lives in the address, so a search or a set of filters can
 * be shared, reloaded and reached from the search overlay. Absent values are
 * the defaults. difficulty and language are three-way: absent means "from
 * your profile" (signed in), present and empty means "all of them".
 */

export const DIFFICULTIES: Difficulty[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED", "UNCLASSIFIED"];
/** The sorts Explore offers (the API still accepts shuffle). */
export const SORTS = ["foryou", "easiest", "newest"] as const satisfies readonly Sort[];

export interface Panel {
  labels: string[];
  tech: string[];
  topics: string[];
  language: string[];
  comments_min?: number;
  comments_max?: number;
  sort: Sort;
  /** Show issues someone is assigned to as well (?assigned=include); left out by default. */
  includeAssigned: boolean;
}

export interface CatalogQuery extends Panel {
  search: string;
  /** Narrowed to one repository (owner/name) or one owner, from an issue's links. */
  repo: string[];
  owner: string[];
  /** null: not chosen, so the profile's level applies. */
  difficulty: Difficulty[] | null;
  /** false: languages not chosen, so the profile's preferred languages apply. */
  languageChosen: boolean;
}

const list = (p: URLSearchParams, k: string) =>
  (p.get(k) ?? "")
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);

function count(p: URLSearchParams, k: string): number | undefined {
  const v = p.get(k);
  if (v === null || v === "") return undefined;
  const n = Number(v);
  return Number.isInteger(n) && n >= 0 ? n : undefined;
}

export function readQuery(p: URLSearchParams): CatalogQuery {
  const sort = p.get("sort") as Sort | null;
  return {
    search: (p.get("search") ?? "").trim(),
    difficulty: p.has("difficulty")
      ? (list(p, "difficulty").filter((d) => (DIFFICULTIES as string[]).includes(d)) as Difficulty[])
      : null,
    repo: list(p, "repo"),
    owner: list(p, "owner"),
    labels: list(p, "labels"),
    tech: list(p, "tech"),
    topics: list(p, "topics"),
    language: list(p, "language"),
    comments_min: count(p, "comments_min"),
    comments_max: count(p, "comments_max"),
    languageChosen: p.has("language"),
    sort: sort && (SORTS as readonly Sort[]).includes(sort) ? sort : "foryou",
    includeAssigned: p.get("assigned") === "include",
  };
}

export function writeQuery(q: CatalogQuery): URLSearchParams {
  const p = new URLSearchParams();
  if (q.search) p.set("search", q.search);
  if (q.difficulty) p.set("difficulty", q.difficulty.join(","));
  for (const k of ["repo", "owner", "labels", "tech", "topics"] as const) {
    if (q[k].length) p.set(k, q[k].join(","));
  }
  // Chosen, even as none ("every language"); not chosen, left out for the profile's.
  if (q.languageChosen || q.language.length) p.set("language", q.language.join(","));
  if (q.comments_min !== undefined) p.set("comments_min", String(q.comments_min));
  if (q.comments_max !== undefined) p.set("comments_max", String(q.comments_max));
  if (q.sort !== "foryou") p.set("sort", q.sort);
  if (q.includeAssigned) p.set("assigned", "include");
  return p;
}

export const emptyPanel: Panel = { labels: [], tech: [], topics: [], language: [], sort: "foryou", includeAssigned: false };

/** The API's filters for a query whose difficulty has been settled. */
export function toFilters(q: CatalogQuery, difficulty: Difficulty[]): IssueFilters {
  return {
    sort: q.sort,
    difficulty: difficulty.length ? difficulty : undefined,
    repo: q.repo,
    owner: q.owner,
    labels: q.labels,
    tech: q.tech,
    topics: q.topics,
    language: q.language,
    comments_min: q.comments_min,
    comments_max: q.comments_max,
    unassigned: !q.includeAssigned,
  };
}

/**
 * Explore narrowed to a repository or an owner: every difficulty and
 * language, easiest first (For you shows only one issue per repository).
 */
export function scopeURL(scope: { repo: string } | { owner: string }): string {
  const p = new URLSearchParams("difficulty=&language=&sort=easiest");
  if ("repo" in scope) p.set("repo", scope.repo);
  else p.set("owner", scope.owner);
  return `/app?${p.toString()}`;
}
