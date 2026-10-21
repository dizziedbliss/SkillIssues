import type { ComponentType } from "react";
import type { Issue } from "@skillissues/client";

/*
 * Where the app runs: the web, or inside the desktop app, which renders these
 * same screens over its own bridge (desktop/src/renderer). Everything that
 * differs between the two goes through here, so the screens stay one set.
 * The desktop calls setPlatform once, before anything renders.
 */
export interface Platform {
  kind: "web" | "desktop";
  /** Starts a GitHub sign-in. On the web the page leaves for GitHub. */
  signIn: (beginWeb: () => Promise<string>) => Promise<void>;
  /** Signs this app out (the session ends either way). */
  signOut?: () => Promise<void>;
  /** Deletes the account; resolves to GitHub's authorised-apps page. */
  deleteAccount?: () => Promise<{ authorized_apps_url: string }>;
  /**
   * Reads a public GitHub API URL with no credentials. The desktop's page
   * reaches no network itself, so its main process makes these reads.
   */
  githubFetch: (url: string, init: { signal?: AbortSignal; accept?: string }) => Promise<Response>;
  /** The site's origin for docs and legal pages; set, they open in the browser. */
  siteOrigin?: string;
  /** What "Work on it" is on an issue page. */
  WorkOnIt?: ComponentType<{ issue: Issue; className?: string }>;
  /** The Desktop app view in Settings. */
  DesktopSettings?: ComponentType;
}

let current: Platform = {
  kind: "web",
  async signIn(begin) {
    window.location.assign(await begin());
  },
  githubFetch: (url, { signal, accept }) =>
    fetch(url, { headers: { Accept: accept ?? "application/vnd.github+json" }, credentials: "omit", signal }),
};

export const platform = (): Platform => current;

/** Replaces the web defaults; the desktop calls it once at start. */
export function setPlatform(p: Partial<Platform>) {
  current = { ...current, ...p };
}
