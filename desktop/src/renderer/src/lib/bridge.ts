// The one seam onto the existing preload bridge (desktop/src/preload/index.ts
// -> ipcMain "api:request" -> desktop/src/main/index.ts's apiRequest/apiJson).
// That IPC contract is untouched by this rewrite — only what calls into it
// changes, from app.js's bespoke `api()` helper to packages/api-client's
// typed ApiClient over createIpcTransport.
import { createApiClient, createIpcTransport, type DesktopApiBridge } from "@skillissues/api-client";

export interface GitHubLoginResult {
  authenticated: boolean;
  cancelled?: boolean;
  message?: string;
  user?: { username: string; avatar_url?: string };
}

export interface WorkspaceResult {
  path: string;
  output?: string;
  reused?: boolean;
}

export interface OpenResult {
  opened: boolean;
  path?: string;
  fallback?: boolean;
  message?: string;
  // Added alongside the ~4s race timeout (D3e): true when shell.openPath (or
  // the VS Code launch) actually resolved in time, false when the renderer
  // gave up waiting and is reporting "probably worked" rather than confirmed
  // success — xdg-open on WSL/Linux can hang indefinitely.
  confirmed: boolean;
}

// Full shape of window.skillIssuesDesktop (desktop/src/preload/index.ts).
// DesktopApiBridge (just `apiRequest`) is the slice packages/api-client needs;
// this is everything else the renderer calls directly for git/workspace/auth
// operations that aren't part of the typed API surface.
export interface DesktopBridge extends DesktopApiBridge {
  loginWithGitHub(): Promise<GitHubLoginResult>;
  cancelGitHubLogin(): Promise<{ cancelled: boolean }>;
  logout(): Promise<{ loggedOut: boolean }>;
  deleteAccount(): Promise<{ purged: boolean }>;
  minimizeWindow(): Promise<void>;
  maximizeWindow(): Promise<void>;
  closeWindow(): Promise<void>;
  isWindowMaximized(): Promise<boolean>;
  getWorkspaceParent(): Promise<{ path: string | null }>;
  chooseWorkspaceParent(): Promise<{ path?: string; changed?: boolean; cancelled?: boolean }>;
  inspectWorkspace(workspace: string): Promise<{ path: string; status: string }>;
  resolveRepositoryWorkspace(repository: string): Promise<{ path: string }>;
  cloneRepository(repositoryUrl: string): Promise<WorkspaceResult>;
  openWorkspace(workspace: string): Promise<OpenResult>;
  openInVSCode(workspace: string): Promise<OpenResult>;
  createBranch(workspace: string, branch: string): Promise<WorkspaceResult>;
  pushBranch(workspace: string, branch: string): Promise<WorkspaceResult>;
  commitAndPush(workspace: string, branch: string, message: string): Promise<WorkspaceResult>;
  onOpenDeepLink(callback: (url: string) => void): void;
}

declare global {
  interface Window {
    skillIssuesDesktop: DesktopBridge;
  }
}

export const desktop: DesktopBridge = window.skillIssuesDesktop;

export const apiClient = createApiClient(createIpcTransport(desktop));

// Bounds a bare IPC call with a wall-clock timeout. ipcRenderer.invoke has no
// native cancellation (see createIpcTransport's signal-stripping comment), so
// a hung main-process call — logout's network request, or shell.openPath
// wedged behind xdg-open on WSL/Linux — can otherwise wedge a button
// forever. The underlying call may still resolve later; its result is
// simply ignored once this has already timed out.
export function withTimeout<T>(promise: Promise<T>, ms: number, message: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), ms);
    promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (err) => {
        clearTimeout(timer);
        reject(err instanceof Error ? err : new Error(String(err)));
      },
    );
  });
}
