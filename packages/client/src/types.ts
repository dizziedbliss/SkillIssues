// Response shapes of the SkillIssues API. Field names are the wire names.

export type Difficulty = "BEGINNER" | "INTERMEDIATE" | "ADVANCED" | "UNCLASSIFIED";
export type Sort = "shuffle" | "easiest" | "newest";
/** A search's kinds of match, as match_category names them. */
export type SearchCategory = "Repository" | "Title" | "Language";

export interface Issue {
  id: number;
  github_id: number;
  number: number;
  title: string;
  url: string;
  state: "OPEN" | "CLOSED";
  updated_at: string;
  /** When the issue was opened on GitHub; "" when unknown. */
  created_at: string;
  /** null for UNCLASSIFIED: unknown difficulty is not easy difficulty. */
  difficulty_score: number | null;
  difficulty: Difficulty;
  required_skills: string[];
  technologies: string[];
  learning_tags: string[];
  /** The issue's labels as GitHub has them. */
  labels: string[];
  /** The repository's three largest languages by GitHub's names, largest first. */
  tech: string[];
  comments: number;
  author: string;
  repository: string;
  repository_url: string;
  repository_description: string;
  language: string;
  stars: number;
  license: string;
  recommendation_score?: number;
  why_recommended?: string[];
  match_category?: SearchCategory;
}

/** GET /issues with ?cursor=: one page and the cursor of the next, null at the end. */
export interface IssuePage {
  issues: Issue[];
  next_cursor: string | null;
}

/** Catalog filters. Values within one filter are alternatives; filters combine. */
export interface IssueFilters {
  sort?: Sort;
  difficulty?: Difficulty[];
  language?: string[];
  labels?: string[];
  topics?: string[];
  tech?: string[];
  /** owner/name */
  repo?: string[];
  owner?: string[];
  comments_min?: number;
  comments_max?: number;
  limit?: number;
}

export interface Facet {
  value: string;
  issues: number;
}

export interface Facets {
  labels: Facet[];
  topics: Facet[];
  tech: Facet[];
  languages: Facet[];
  refreshed_at: string | null;
}

export interface Activity {
  at: string;
  repository: string;
  arrived: number;
  closed: number;
}

/** GET /stats: live catalog counts. */
export interface Stats {
  open_issues: number;
  closed_since: number;
  tracking_since: string | null;
  arrived_today: number;
  closed_today: number;
  issues_indexed: number;
  repositories_indexed: number;
  developers: number;
  activity: Activity[];
  updated_at: string | null;
}

/** GET /repositories/lookup. */
export interface RepositoryLookup {
  found: boolean;
  full_name?: string;
  url?: string;
  eligible: boolean;
  /** Why a stored repository is not crawled; absent when it is. */
  reason?: string;
  stars: number;
  open_issues: number;
}

export interface Theme {
  id: string;
  name: string;
  toml: string;
  css: string;
  public: boolean;
  author: string;
  mine: boolean;
  created_at: string;
  updated_at: string;
}

export interface ThemeInput {
  toml: string;
  css: string;
  public: boolean;
}

/** GET /auth/session. */
export interface Session {
  authenticated: boolean;
  user?: { id: number; username: string; avatar_url: string };
}

/** GET /auth/github. enabled is false while sign-in is not configured on the server. */
export interface SignInStart {
  enabled: boolean;
  authorize_url: string;
  error?: string;
}

/** POST /auth/purge. */
export interface AccountDeleted {
  purged: boolean;
  /** GitHub's authorised-apps page, where the sign-in app can be removed too. */
  authorized_apps_url: string;
}

/** GET /issues/{id}/body: the body, or why it can't be had right now. */
export type IssueBody = { body: string } | { status: "queued"; reason: string };

/** POST /saved, DELETE /saved/{id}. */
export interface SavedChange {
  saved: boolean;
  issue_id: number;
}

export type Level = "BEGINNER" | "INTERMEDIATE" | "ADVANCED";

/** GET /me. */
export interface Profile {
  id: number;
  username: string;
  name: string;
  avatar_url: string;
  experience_level: Level;
  target_level: Level;
  bio: string;
  demonstrated_skills: string[];
  interests: string[];
  preferred_languages: string[];
  /** When the account was created here (RFC 3339). */
  joined_at: string;
}

/** One notification: today only an issue you saved being closed on GitHub. */
export interface Notification {
  id: number;
  kind: "issue_closed";
  created_at: string;
  read_at: string | null;
  issue: { id: number; repository: string; number: number; title: string; url: string; state: "OPEN" | "CLOSED" };
}

/** GET /notifications. */
export interface Notifications {
  notifications: Notification[];
  unread_count: number;
}
