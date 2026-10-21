import { app, shell } from "electron";
import { autoUpdater } from "electron-updater";
import type { ReleaseNotes, UpdateState } from "../shared/contract";
import { isNewerVersion, isVersion } from "./security";
import { lastVersion, setLastVersion } from "./store";

/*
 * Updates from the app itself, from GitHub releases (release.yml publishes
 * the installers with the latest*.yml files electron-updater reads; it checks
 * each file's sha512). The app checks on start and every few hours, and asks:
 * a new version is "available" until the person chooses to download it. Then
 * it installs on the next restart, or straight away from the title bar.
 * macOS only installs updates to a signed app, which these builds aren't
 * yet, so there downloading opens the release page instead.
 *
 * After an update, the first start shows that version's release notes
 * (whatsNew), read from its GitHub release.
 */

/** How often a running app checks again. */
const CHECK_EVERY_MS = 6 * 60 * 60 * 1000;
const RELEASES = "https://github.com/dizziedbliss/SkillIssues/releases/latest";
const RELEASE_API = "https://api.github.com/repos/dizziedbliss/SkillIssues/releases/tags/v";
/** Release notes longer than this are cut: they're shown in a dialog. */
const MAX_NOTES = 50_000;

let state: UpdateState = { status: "idle" };
let onChange: (s: UpdateState) => void = () => undefined;

function set(next: UpdateState) {
  state = next;
  onChange(state);
}

export function updateState(): UpdateState {
  return state;
}

const manual = process.platform === "darwin";

/** Decided once at start: the version this run was updated to, if it was. */
let justUpdatedTo: string | null = null;
let whatsNewTaken = false;

export function startUpdates(changed: (s: UpdateState) => void) {
  onChange = changed;

  const now = app.getVersion();
  const before = lastVersion();
  // The first run ever has nothing to compare with: no notes then.
  if (before && isVersion(before) && isVersion(now) && isNewerVersion(now, before)) justUpdatedTo = now;
  if (before !== now) setLastVersion(now);

  // A development run has nothing to update to.
  if (!app.isPackaged) {
    set({ status: "unsupported", reason: "development" });
    return;
  }
  // Ask first: nothing downloads until the person says so. Once they have,
  // closing the app installs it.
  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = true;
  autoUpdater.logger = { info: () => undefined, warn: console.warn, error: console.error, debug: () => undefined };
  autoUpdater.on("checking-for-update", () => {
    // A check while one is offered or downloading leaves that as it is.
    if (state.status !== "available" && state.status !== "downloading" && state.status !== "ready") set({ status: "checking" });
  });
  autoUpdater.on("update-not-available", () => set({ status: "current" }));
  autoUpdater.on("update-available", (info) => {
    if (state.status === "downloading" || state.status === "ready") return;
    set({ status: "available", version: info.version, manual });
  });
  autoUpdater.on("download-progress", (p) => set({ status: "downloading", version: state.version, percent: Math.round(p.percent) }));
  autoUpdater.on("update-downloaded", (info) => set({ status: "ready", version: info.version, manual: false }));
  autoUpdater.on("error", (err) => {
    console.error("[updater]", err);
    set({ status: "error", reason: err instanceof Error ? err.message : String(err) });
  });
  const check = () =>
    autoUpdater.checkForUpdates().catch((err: unknown) => {
      console.error("[updater] check failed:", err);
      set({ status: "error", reason: err instanceof Error ? err.message : String(err) });
    });
  void check();
  setInterval(() => void check(), CHECK_EVERY_MS);
}

/** Downloads the offered update, because the person asked (macOS: opens the release page). */
export async function downloadUpdate(): Promise<UpdateState> {
  if (state.status !== "available") throw new Error("There's no update to download.");
  if (state.manual) {
    await shell.openExternal(RELEASES);
    return state;
  }
  set({ status: "downloading", version: state.version, percent: 0 });
  // Progress and the end arrive as events; a failure is the error event too.
  autoUpdater.downloadUpdate().catch((err: unknown) => {
    console.error("[updater] download failed:", err);
    set({ status: "error", reason: err instanceof Error ? err.message : String(err) });
  });
  return state;
}

/** Restarts into the downloaded update now. */
export function installUpdate(): void {
  if (state.status !== "ready") throw new Error("There's no update ready yet.");
  autoUpdater.quitAndInstall();
}

/** Checks again now. */
export async function checkUpdates(): Promise<UpdateState> {
  if (!app.isPackaged) return state;
  await autoUpdater.checkForUpdates();
  return state;
}

/** The version this run was just updated to, the first time it's asked. */
export function whatsNew(): string | null {
  if (whatsNewTaken) return null;
  whatsNewTaken = true;
  return justUpdatedTo;
}

/** A version's release notes from its GitHub release, read without credentials. */
export async function releaseNotes(version: unknown): Promise<ReleaseNotes | null> {
  if (!isVersion(version)) throw new Error("That isn't a version.");
  let res: Response;
  try {
    res = await fetch(RELEASE_API + version, { headers: { Accept: "application/vnd.github+json" } });
  } catch {
    throw new Error("Can't reach GitHub for the release notes. Check your connection and try again.");
  }
  if (res.status === 404) return null;
  if (!res.ok) {
    throw new Error(
      res.status === 403 || res.status === 429
        ? "GitHub's rate limit is in the way of the release notes; try again later."
        : `GitHub didn't send the release notes (HTTP ${res.status}).`,
    );
  }
  const data = (await res.json()) as { body?: unknown; html_url?: unknown; published_at?: unknown };
  return {
    version,
    body: typeof data.body === "string" ? data.body.slice(0, MAX_NOTES) : "",
    url: typeof data.html_url === "string" ? data.html_url : RELEASES,
    published_at: typeof data.published_at === "string" ? data.published_at : "",
  };
}
