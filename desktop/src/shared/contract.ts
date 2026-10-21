/*
 * The one contract between the renderer and the main process. The preload
 * exposes exactly these methods as window.skillissues, each over the channel
 * named here, and the main process registers a handler for each channel and
 * nothing else. Every call answers with a Result: errors cross the bridge as
 * data, with a message meant for the person using the app.
 */

export type Result<T> = { ok: true; value: T } | { ok: false; error: string };

export type ApiMethod = "GET" | "POST" | "PUT" | "DELETE";

/** One request to the SkillIssues API, made by the main process with the session. */
export interface ApiCall {
  method: ApiMethod;
  path: string;
  body?: unknown;
}

/** The API's answer as data, so the renderer's client can raise its own ApiError. */
export type ApiReply =
  | { ok: true; status: number; data: unknown }
  | { ok: false; status: number; error: string; problems: string[] };

export interface User {
  id: number;
  username: string;
  avatar_url: string;
}

export interface AuthState {
  signedIn: boolean;
  user: User | null;
  /** A GitHub token for forking and pushing is on this machine. */
  pushConnected: boolean;
  /**
   * Whether tokens survive a restart. False where the OS offers no real
   * encryption (Linux without a keyring): they are then kept in memory only.
   */
  persisted: boolean;
}

/** What to show while GitHub's device flow waits for the person. */
export interface DeviceCode {
  user_code: string;
  verification_uri: string;
  expires_in: number;
}

/** The issue a workspace is for. */
export interface IssueRef {
  id: number;
  repository: string; // owner/name
  number: number;
  title: string;
}

export interface Workspace {
  issueId: number;
  issueNumber: number;
  title: string;
  repository: string; // the upstream, owner/name
  fork: string; // the person's fork, owner/name
  path: string;
  branch: string;
  /** The fork's default branch the work started from (older workspaces: read from git). */
  base?: string;
  /** When work started, ISO time (missing on workspaces from before 1.1.2). */
  startedAt?: string;
  /** git status --porcelain lines; empty when clean. Read fresh, never stored. */
  changes: string[];
  /** Commits on the branch that aren't on the default branch. Read fresh. */
  commits: number;
  /** Of those, the ones not on your fork yet. Read fresh. */
  unpushed: number;
  /** Why the folder couldn't be read (moved, deleted), or null. */
  problem: string | null;
  /** The pull request opened from this workspace, once there is one. */
  pullRequestUrl: string | null;
}

/** A commit: a one-line title, and an optional longer description. */
export interface CommitInput {
  title: string;
  body: string;
}

/** A pull request as the person wrote it. */
export interface PullRequestInput {
  title: string;
  body: string;
  /** Add "Closes #<number>": GitHub closes the issue on merge, and SkillIssues links them. */
  closesIssue: boolean;
}

export interface Settings {
  workspaceRoot: string;
}

/**
 * Where the app's own updates stand (main/updater.ts). Nothing downloads
 * until the person asks: "available" waits for them.
 */
export interface UpdateState {
  status: "idle" | "checking" | "current" | "available" | "downloading" | "ready" | "error" | "unsupported";
  version?: string;
  percent?: number;
  /** macOS: installing means downloading it from the release page. */
  manual?: boolean;
  reason?: string;
}

/** A version's release notes, from its GitHub release. */
export interface ReleaseNotes {
  version: string;
  /** Markdown, rendered through packages/markdown. */
  body: string;
  url: string;
  published_at: string;
}

/** The title bar's look, from the theme in use. */
export interface TitleBarColors {
  /** The window buttons' colour, #rrggbb. */
  symbol: string;
  /**
   * Whether the theme is dark: the OS then draws the window's edge, its
   * shadow and its corners dark too, instead of a light border around a dark app.
   */
  dark: boolean;
}

/** A public GitHub API answer, read by the main process with no credentials. */
export interface GitHubReply {
  status: number;
  body: string;
  /** Only the rate-limit headers the page reads. */
  headers: Record<string, string>;
}

export const channels = {
  api: "api:request",
  authState: "auth:state",
  signInStart: "auth:sign-in-start",
  signInWait: "auth:sign-in-wait",
  pushStart: "auth:push-start",
  pushWait: "auth:push-wait",
  authCancel: "auth:cancel",
  pushDisconnect: "auth:push-disconnect",
  signOut: "auth:sign-out",
  deleteAccount: "auth:delete-account",
  settings: "settings:get",
  chooseWorkspaceRoot: "settings:choose-root",
  workspace: "workspace:get",
  workspaces: "workspace:list",
  startWork: "workspace:start",
  openFolder: "workspace:open-folder",
  openEditor: "workspace:open-editor",
  commit: "workspace:commit",
  push: "workspace:push",
  openPullRequest: "workspace:open-pr",
  forgetWorkspace: "workspace:forget",
  copyText: "clipboard:write",
  openExternal: "shell:open-external",
  githubGet: "github:get",
  takeLink: "app:take-link",
  windowInfo: "window:info",
  titleBar: "window:title-bar",
  updateState: "update:state",
  checkUpdates: "update:check",
  downloadUpdate: "update:download",
  installUpdate: "update:install",
  releaseNotes: "update:notes",
  whatsNew: "update:whats-new",
} as const;

export type Channel = (typeof channels)[keyof typeof channels];

/** window.skillissues */
export interface DesktopBridge {
  api(call: ApiCall): Promise<Result<ApiReply>>;
  authState(): Promise<Result<AuthState>>;
  /** Starts sign-in: shows a code and opens GitHub in the browser. */
  signInStart(): Promise<Result<DeviceCode>>;
  /** Resolves once the code is entered on GitHub (or expires, or is cancelled). */
  signInWait(): Promise<Result<AuthState>>;
  /** The same, for the token that forks and pushes (public_repo). */
  pushStart(): Promise<Result<DeviceCode>>;
  pushWait(): Promise<Result<AuthState>>;
  authCancel(): Promise<Result<null>>;
  pushDisconnect(): Promise<Result<AuthState>>;
  signOut(): Promise<Result<AuthState>>;
  deleteAccount(): Promise<Result<{ authorized_apps_url: string }>>;
  settings(): Promise<Result<Settings>>;
  chooseWorkspaceRoot(): Promise<Result<Settings>>;
  workspace(issueId: number): Promise<Result<Workspace | null>>;
  /** Every issue you've started work on here, most recently started first. */
  workspaces(): Promise<Result<Workspace[]>>;
  startWork(issue: IssueRef): Promise<Result<Workspace>>;
  openFolder(issueId: number): Promise<Result<null>>;
  openEditor(issueId: number): Promise<Result<null>>;
  /** Commits every change. Refused when there is nothing to commit. */
  commit(issueId: number, input: CommitInput): Promise<Result<Workspace>>;
  /** Pushes the branch to your fork. Refused with uncommitted changes, or nothing to push. */
  push(issueId: number): Promise<Result<Workspace>>;
  /** Opens the pull request on GitHub. Refused until your commits are pushed. */
  openPullRequest(issueId: number, input: PullRequestInput): Promise<Result<Workspace>>;
  /** Takes the workspace off the list; the folder stays where it is. */
  forgetWorkspace(issueId: number): Promise<Result<null>>;
  /** Puts short text (a sign-in code) on the clipboard. */
  copyText(text: string): Promise<Result<null>>;
  openExternal(url: string): Promise<Result<null>>;
  /** GETs an allowlisted public GitHub API URL, without credentials. */
  githubGet(url: string): Promise<Result<GitHubReply>>;
  /**
   * The app page a skillissues:// link asked for (e.g. "/app/issues/12"),
   * once, or null. The page asks on start and whenever the window is focused.
   */
  takeLink(): Promise<Result<string | null>>;
  /** Which OS draws the window buttons, and where (for the title bar's padding). */
  windowInfo(): Promise<Result<{ platform: string }>>;
  /** Matches the window buttons to the theme. */
  titleBar(colors: TitleBarColors): Promise<Result<null>>;
  updateState(): Promise<Result<UpdateState>>;
  checkUpdates(): Promise<Result<UpdateState>>;
  /** Downloads the available update, when the person says so (macOS: opens the release page). */
  downloadUpdate(): Promise<Result<UpdateState>>;
  /** Restarts into a downloaded update. */
  installUpdate(): Promise<Result<null>>;
  /** A version's release notes (x.y.z), or null when it has no release. */
  releaseNotes(version: string): Promise<Result<ReleaseNotes | null>>;
  /** The version the app was just updated to, once, on its first start after the update; else null. */
  whatsNew(): Promise<Result<string | null>>;
}
