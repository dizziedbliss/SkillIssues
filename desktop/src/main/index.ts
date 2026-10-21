import { app, BrowserWindow, Menu, nativeTheme, session, shell } from "electron";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { channels } from "../shared/contract";
import { registerIpc, TITLE_BAR_HEIGHT } from "./ipc";
import { startUpdates } from "./updater";
import { deepLinkPath, isAllowedExternalURL, webPreferences } from "./security";
import { loadStore } from "./store";

/*
 * The SkillIssues desktop app's main process: one window, locked down
 * (security.ts webPreferences), talking to its page only through the IPC
 * contract (shared/contract.ts). The page can't navigate away, open windows,
 * ask for permissions, or reach the network itself (its CSP has no
 * connect-src beyond the dev server).
 */

if (!app.requestSingleInstanceLock()) app.quit();

// skillissues://issue/<id> opens that issue here (the site's "Work on it in
// the desktop app"). The page collects it with takeLink on start and focus.
app.setAsDefaultProtocolClient("skillissues");
let pendingLink: string | null = null;
function noteLink(raw: string | undefined) {
  const p = raw ? deepLinkPath(raw) : null;
  if (p) pendingLink = p;
}
function takeLink(): string | null {
  const p = pendingLink;
  pendingLink = null;
  return p;
}
process.argv.forEach(noteLink);
app.on("open-url", (event, url) => {
  event.preventDefault();
  noteLink(url);
  win?.focus();
});

Menu.setApplicationMenu(null);

let win: BrowserWindow | null = null;

const devURL = process.env.ELECTRON_RENDERER_URL;
const pageFile = path.join(__dirname, "../renderer/index.html");
const pageURL = devURL ?? pathToFileURL(pageFile).href;

/** The app's own page: the built file, or the dev server's origin under electron-vite dev. */
function isAppURL(url: string): boolean {
  if (devURL) return new URL(url).origin === new URL(devURL).origin;
  return url.split("#")[0]!.split("?")[0] === pageURL;
}

function createWindow() {
  win = new BrowserWindow({
    width: 1240,
    height: 820,
    minWidth: 900,
    minHeight: 620,
    show: false,
    backgroundColor: "#0a0a0c", // Midnight, the app's default, so it opens dark
    // No native frame: the page draws a slim title bar, and the window's own
    // buttons sit over it (Windows, Linux) or inset (macOS), as Discord does.
    titleBarStyle: "hidden",
    ...(process.platform === "darwin"
      ? { trafficLightPosition: { x: 14, y: 12 } }
      : { titleBarOverlay: { color: "#00000000", symbolColor: "#f2f2f2", height: TITLE_BAR_HEIGHT } }),
    title: "SkillIssues",
    icon: path.join(app.getAppPath(), "icon.png"),
    autoHideMenuBar: true,
    webPreferences: { ...webPreferences, preload: path.join(__dirname, "../preload/index.js") },
  });

  win.once("ready-to-show", () => win?.show());

  // Links go to the system browser, and only allowlisted ones.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (isAllowedExternalURL(url)) void shell.openExternal(url);
    return { action: "deny" };
  });
  win.webContents.on("will-navigate", (event, url) => {
    if (!isAppURL(url)) event.preventDefault();
  });
  win.webContents.on("will-attach-webview", (event) => event.preventDefault());

  if (devURL) void win.loadURL(devURL);
  else void win.loadFile(pageFile);

  win.on("closed", () => {
    win = null;
  });
}

app.on("second-instance", (_event, argv) => {
  argv.forEach(noteLink);
  if (!win) return;
  if (win.isMinimized()) win.restore();
  win.focus();
});

app.whenReady().then(() => {
  loadStore();
  // Dark until the page says which mode its theme is in (the app starts dark).
  nativeTheme.themeSource = "dark";
  session.defaultSession.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
  session.defaultSession.setPermissionCheckHandler(() => false);
  const registered = registerIpc(() => win, isAppURL, takeLink);
  const missing = Object.values(channels).filter((c) => !registered.includes(c));
  if (missing.length) throw new Error(`IPC channels without a handler: ${missing.join(", ")}`);
  createWindow();
  startUpdates(() => undefined);
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

process.on("unhandledRejection", (err) => console.error("[main] unhandled rejection:", err));
