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
  /** git status --porcelain lines; empty when clean. */
  changes: string[];
  /** The pull request opened from this workspace, once there is one. */
  pullRequestUrl: string | null;
}

export interface Settings {
  workspaceRoot: string;
}

/** Where the app's own updates stand (main/updater.ts). */
export interface UpdateState {
  status: "idle" | "checking" | "current" | "downloading" | "ready" | "error" | "unsupported";
  version?: string;
  percent?: number;
  /** macOS: installing means downloading it from the release page. */
  manual?: boolean;
  reason?: string;
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
  startWork: "workspace:start",
  openFolder: "workspace:open-folder",
  openEditor: "workspace:open-editor",
  commitAndPush: "workspace:commit-push",
  openPullRequest: "workspace:open-pr",
  openExternal: "shell:open-external",
  githubGet: "github:get",
  takeLink: "app:take-link",
  windowInfo: "window:info",
  titleBar: "window:title-bar",
  updateState: "update:state",
  checkUpdates: "update:check",
  installUpdate: "update:install",
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
  startWork(issue: IssueRef): Promise<Result<Workspace>>;
  openFolder(issueId: number): Promise<Result<null>>;
  openEditor(issueId: number): Promise<Result<null>>;
  commitAndPush(issueId: number, message: string): Promise<Result<Workspace>>;
  openPullRequest(issueId: number): Promise<Result<Workspace>>;
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
  /** Restarts into a downloaded update (macOS: opens the release page). */
  installUpdate(): Promise<Result<null>>;
}
