import type { Platform } from "@web/app/platform";
import { call, desktop } from "./bridge";
import { requestSignIn } from "./signIn";
import { WorkOnIt } from "./WorkOnIt";
import { DesktopSettings } from "./DesktopSettings";
import { WorkspacesPage } from "./WorkspacesPage";

/** What differs inside the desktop app (web/src/app/platform.ts). */
export const desktopPlatform: Partial<Platform> = {
  kind: "desktop",
  // GitHub's device flow, in a dialog: the page never leaves the app.
  signIn: () => requestSignIn(),
  async signOut() {
    await call(desktop.signOut());
  },
  deleteAccount: () => call(desktop.deleteAccount()),
  async githubFetch(url, { signal }) {
    const r = await call(desktop.githubGet(url));
    signal?.throwIfAborted();
    return new Response(r.body, { status: r.status, headers: r.headers });
  },
  siteOrigin: "https://skillissues.dizziedbliss.lol",
  WorkOnIt,
  DesktopSettings,
  Workspaces: WorkspacesPage,
};
