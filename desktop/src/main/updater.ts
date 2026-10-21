import { app, shell } from "electron";
import { autoUpdater } from "electron-updater";
import type { UpdateState } from "../shared/contract";

/*
 * Updates from the app itself, from GitHub releases (release.yml publishes
 * the installers with the latest*.yml files electron-updater reads; it checks
 * each file's sha512). On Windows and Linux (AppImage) a new version
 * downloads in the background and installs on restart, or when the person
 * presses the title bar's update button. macOS only installs updates to a
 * signed app, which these builds aren't yet, so there the button opens the
 * release page instead.
 */

/** How often a running app checks again. */
const CHECK_EVERY_MS = 6 * 60 * 60 * 1000;
const RELEASES = "https://github.com/dizziedbliss/SkillIssues/releases/latest";

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

export function startUpdates(changed: (s: UpdateState) => void) {
  onChange = changed;
  // A development run has nothing to update to.
  if (!app.isPackaged) {
    set({ status: "unsupported", reason: "development" });
    return;
  }
  autoUpdater.autoDownload = !manual;
  autoUpdater.autoInstallOnAppQuit = !manual;
  autoUpdater.logger = { info: () => undefined, warn: console.warn, error: console.error, debug: () => undefined };
  autoUpdater.on("checking-for-update", () => set({ status: "checking" }));
  autoUpdater.on("update-not-available", () => set({ status: "current" }));
  autoUpdater.on("update-available", (info) =>
    set({ status: manual ? "ready" : "downloading", version: info.version, percent: 0, manual }),
  );
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

/** Installs a downloaded update now (restarting the app), or opens the release page on macOS. */
export async function installUpdate(): Promise<void> {
  if (state.status !== "ready") throw new Error("There's no update ready yet.");
  if (state.manual) {
    await shell.openExternal(RELEASES);
    return;
  }
  autoUpdater.quitAndInstall();
}

/** Checks again now. */
export async function checkUpdates(): Promise<UpdateState> {
  if (!app.isPackaged) return state;
  await autoUpdater.checkForUpdates();
  return state;
}
