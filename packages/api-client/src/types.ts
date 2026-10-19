// Shapes mirror the JSON the Go API actually serializes
// (backend/internal/issue/search.go, backend/internal/contribution/lifecycle.go,
// backend/internal/profile/profile.go). Keep both in sync by hand — there is
// no shared schema generator — but there is now exactly one copy to update
// instead of two drifting ones in web/src/App.tsx and desktop/src/renderer/app.js.

export type Difficulty = "BEGINNER" | "INTERMEDIATE" | "ADVANCED" | "UNCLASSIFIED";

export interface Issue {
  id: number;
  github_id: number;
  number: number;
  title: string;
  url: string;
  state: string; // "OPEN" | "CLOSED"
  updated_at: string;
  // When the issue was opened on GitHub; "" when unknown.
  created_at: string;
  // null for UNCLASSIFIED (D6) — never defaulted to a number that would
  // imply BEGINNER or any other difficulty.
  difficulty_score: number | null;
  difficulty: string;
  required_skills: string[];
  technologies: string[];
  learning_tags: string[];
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
  match_category?: string;

  // Only populated on the saved-issues list (D2b): the caller's own latest
  // contribution against this issue — none|working|submitted|merged.
  lifecycle?: "none" | "working" | "submitted" | "merged";
}

export type ContributionState =
  | "STARTED"
  | "PR_OPEN"
  | "PR_CLOSED"
  | "PR_MERGED"
  | "VERIFIED";

export interface Contribution {
  id: number;
  user_id: number;
  issue_id: number;
  pr_url: string | null;
  pr_number: number | null;
  state: ContributionState;
  xp_awarded: boolean;
  created_at: string;
  updated_at: string;
  merged_at: string | null;
  submitted_at: string | null;
}

// The enriched shape returned by GET /contributions, /working, /contributions/{id}.
export interface ContributionDetail {
  id: number;
  user_id: number;
  issue_id: number;
  pr_url: string;
  pr_number: number;
  state: ContributionState;
  created_at: string;
  updated_at: string;
  merged_at: string;
  submitted_at: string;
  title: string;
  number: number;
  difficulty: string;
  issue_url: string;
  required_skills: string[];
  technologies: string[];
  issue_updated_at?: string;
  repository: string;
  repository_url?: string;
  language: string;
}

export interface BadgeDetail {
  name: string;
  icon: string;
}

export interface Profile {
  username: string;
  name?: string;
  avatar_url: string;
  bio: string;
  experience_level?: string;
  target_level?: string;
  interests: string[];
  preferred_languages: string[];
  demonstrated_skills: string[];
  progress_score: number;
  completed_count: number;
  total_xp?: number;
  level?: number;
  badges?: string[];
  badge_details?: BadgeDetail[];
  progress_percent?: number;
}

export interface ProfileUpdate {
  bio?: string;
  preferred_languages?: string[];
  interests?: string[];
  experience_level?: string;
  target_level?: string;
}

export interface SessionUser {
  username: string;
  avatar_url?: string;
}

export interface SessionInfo {
  authenticated: boolean;
  user?: SessionUser;
}

// POST /issues/{id}/freshness and the per-id entries of the batch endpoint.
export interface FreshnessResult {
  state: "open" | "closed";
  refresh_enqueued?: boolean;
}

export interface FreshnessBatchResult {
  results: Record<string, FreshnessResult>;
}

export interface IssueBodyResult {
  body: string;
}

export interface IssueBodyPending {
  status: "queued";
  // budget: the server's GitHub budget is spent for now; no-credential: the
  // server has no GitHub token; disabled: the fallback serves cached bodies
  // only (api.bodies.fallback_misses_per_hour is 0). "fetching" is kept for
  // older servers.
  reason: "budget" | "no-credential" | "disabled" | "fetching";
}

export interface PullRequestSummary {
  html_url: string;
  number: number;
  base: string;
}

/** GitHub refused to open the PR with the user's token; html_url is the compare page. */
export interface ManualComparePullRequest {
  html_url: string;
  base: string;
  manual_compare: true;
}

/**
 * Either the PR was opened and the contribution is now PR_OPEN, or no PR
 * exists yet and the contribution is unchanged (no `contribution` field).
 */
export type SubmitContributionResult =
  | { contribution: Contribution; pull_request: PullRequestSummary; web_url: string }
  | { pull_request: ManualComparePullRequest; web_url: string };

export interface VerifyResult {
  verified: boolean;
  state: string;
  already_awarded?: boolean;
  earned_xp: number;
  total_xp: number;
  old_level: number;
  level: number;
  level_up: boolean;
  unlocked_badges: string[];
  all_badges: string[];
  badges: string[];
  next_level_xp: number;
  progress_in_level: number;
  progress_percent: number;
  message: string;
  xp_info?: Record<string, unknown>;
}

export interface IssueFilters extends IssueFilterSet {
  difficulty?: "ALL" | "BEGINNER" | "INTERMEDIATE" | "ADVANCED" | "UNCLASSIFIED";
  language?: string;
  search?: string;
  limit?: number;
  page?: number;
}

// shuffle: every repository's best issue, repositories in a random order
// redrawn daily (the default for cursor paging). easiest: difficulty, then
// recently updated. newest: most recently opened.
export type IssueSort = "shuffle" | "easiest" | "newest";

// The catalog filters. Within one list any value matches; across filters every
// one must. Values for labels, topics, tech and languages come from
// getIssueFacets(); labels match case-insensitively.
export interface IssueFilterSet {
  difficulties?: Difficulty[];
  languages?: string[]; // the repository's main language
  labels?: string[];
  topics?: string[];
  tech?: string[]; // any language GitHub reports in the repository
  commentsMin?: number;
  commentsMax?: number; // commentsMax: 0 is "no comments yet"
}

// Filters for cursor paging (GET /issues?cursor=). Browsing only: a search
// pages by `page`. Keep the filters the same for every page of one walk.
export interface IssueBrowseFilters extends IssueFilterSet {
  sort?: IssueSort;
  limit?: number;
  // A previous page's nextCursor; omitted for the first page.
  cursor?: string;
}

// One filter dropdown value and how many open issues have it.
export interface IssueFacet {
  value: string;
  issues: number;
}

// GET /issues/facets: each filter's values, the most common first, refreshed
// daily by the server. refreshed_at is null until the first refresh.
export interface IssueFacets {
  labels: IssueFacet[];
  topics: IssueFacet[];
  tech: IssueFacet[];
  languages: IssueFacet[];
  refreshed_at: string | null;
}

// One cursor page. nextCursor is null on the last page.
export interface IssuePage {
  issues: Issue[];
  nextCursor: string | null;
}

export interface PreferenceOptions {
  options: string[];
}

// GET /health/detail (backend/cmd/api/handlers_health.go). A component that
// is down is reported as such (status "degraded", cache "unavailable"),
// never silently counted as connected.
export interface HealthDetail {
  status: string;
  service: string;
  repositories: number;
  issues: number;
  contributions: number;
  cache: string;
  db_size_mb: number;
  max_db_size_mb: number;
  ingestion_paused: boolean;
}

// GET /auth/github (backend/cmd/api/handlers_auth_oauth.go). enabled:false
// means GitHub OAuth isn't configured server-side — the caller must show
// `error`, never fall back to a token-entry form (OAuth/GitHub App is the
// only supported login path).
export interface GithubAuthorizeResult {
  enabled: boolean;
  authorize_url?: string;
  redirect_uri?: string;
  ticket?: string;
  error?: string;
}
