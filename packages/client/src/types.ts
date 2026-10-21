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
  /** People assigned on GitHub; null when not read yet. */
  assignees: number | null;
  /** The repository's contributing guide on GitHub; null when it has none or it isn't read yet. */
  contributing_url: string | null;
  /** Someone on SkillIssues said they're working on it. */
  claimed: boolean;
  comments: number;
  author: string;
  repository: string;
  /** SkillIssues' id for the repository: what a repository report names. */
  repository_id: number;
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
  /** Only issues nobody is assigned to on GitHub (one never read counts as unassigned). */
  unassigned?: boolean;
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

/** GET /me/theme: a stored theme, a built-in applied as it is, or null (the default). */
export type ActiveTheme = Theme | { preset: string } | null;

/** POST /themes/validate when the theme is valid. */
export interface ThemeCheck {
  valid: true;
  name: string;
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

/** PUT /profile: the whole editable profile. */
export interface ProfileUpdate {
  bio: string;
  experience_level: Level;
  target_level: Level;
  interests: string[];
  preferred_languages: string[];
}

/**
 * One notification: an issue you saved closed on GitHub (issue), or a pull
 * request of yours merged and paid (pull_request).
 */
export type Notification = {
  id: number;
  created_at: string;
  read_at: string | null;
} & (
  | { kind: "issue_closed"; issue: { id: number; repository: string; number: number; title: string; url: string; state: "OPEN" | "CLOSED" } }
  | { kind: "pr_merged"; pull_request: { id: number; repository: string; number: number; title: string; url: string; xp: number } }
);

/** Where an XP total stands: level L runs from start up to next. */
export interface XPLevel {
  level: number;
  start: number;
  next: number;
}

/** How an XP amount was reached, as the ledger stores it. */
export interface XPDetail {
  base?: number;
  difficulty?: Difficulty;
  issue_id?: number;
  first_in_repo?: number;
  backlog?: number;
  ci?: number;
  approval?: number;
  clean_merge?: number;
  /** Backfill: the share paid for a merge from before signing up. */
  percent?: number;
  /** Taken off by the daily cap per repository. */
  capped?: number;
  repository_id?: number;
  language?: string;
  docs?: boolean;
}

export type XPSource = "merged" | "backfill" | "opened" | "opened_reversed" | "quest" | "path";

/** One ledger row. */
export interface XPEvent {
  source: XPSource;
  key?: string;
  amount: number;
  detail: XPDetail;
  earned_at: string;
  pull_request?: { id: number; repository: string; number: number; title: string; url: string };
}

/** Level, streak, badges and languages, from the ledger. */
export interface XPSummary {
  total: number;
  level: XPLevel;
  merges: number;
  languages: { language: string; xp: number; level: XPLevel }[];
  /** UTC weeks in a row with a merge. */
  streak: { current: number; best: number; this_week: boolean };
  badges: { key: string; earned_at: string }[];
}

/** Where tracking of your pull requests stands. */
export interface PRSync {
  /** False until the tracker has enrolled you, within minutes of signing up. */
  tracked: boolean;
  synced_through: string | null;
  /** The first read of your history hasn't finished. */
  backfilling: boolean;
  last_checked_at: string | null;
  next_check_at: string | null;
  requested_at: string | null;
  last_error: string | null;
}

/** GET /me/xp. */
export interface MyXP {
  summary: XPSummary;
  recent: XPEvent[];
  sync: PRSync;
  /** Every badge, in display order: those not in summary.badges are still to earn. */
  badge_keys: string[];
}

/** Your claim on an issue. */
export interface Claim {
  issue_id: number;
  claimed_at: string;
  expires_at: string;
  /** Past expires_at but kept: you have an open pull request that closes it. */
  extended: boolean;
}

/** GET /issues/{id}/claim. */
export interface ClaimState {
  claimed: boolean;
  /** Yours, when the claim is yours. */
  yours: Claim | null;
}

/** A pull request of yours to a catalog repository, and what it earned. */
export interface TrackedPull {
  id: number;
  repository: string;
  number: number;
  title: string;
  url: string;
  state: "OPEN" | "CLOSED" | "MERGED";
  created_at: string;
  updated_at: string;
  merged_at: string | null;
  closed_at: string | null;
  /** The last commit's checks; null when the repository runs none. */
  checks: string | null;
  approvals: number;
  changes_requested: number;
  /** Why it earns nothing; null when it can. */
  no_xp_reason: "own_repository" | "organisation_member" | null;
  /** Catalog issues it says it closes. */
  issues: number[];
  xp: number;
}

/** GET /notifications. */
export interface Notifications {
  notifications: Notification[];
  unread_count: number;
}

/** What can be reported, and why (POST /reports). */
export type ReportTarget = { type: "issue"; id: number } | { type: "repository"; id: number } | { type: "theme"; id: string };
export const REPORT_REASONS = {
  issue: ["spam", "abuse", "already-fixed", "not-an-issue", "wrong-difficulty", "other"],
  repository: ["spam", "malware", "abandoned", "other"],
  theme: ["abuse", "misleading", "broken", "other"],
} as const;
export type ReportReason<T extends ReportTarget["type"]> = (typeof REPORT_REASONS)[T][number];

/** A public theme in the community listing. */
export interface SharedTheme extends Theme {
  uses: number;
}

/** GET /themes/public. */
export interface SharedThemes {
  themes: SharedTheme[];
  page: number;
  more: boolean;
}
