import { app, BrowserWindow, Menu, session, shell } from "electron";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { channels } from "../shared/contract";
import { registerIpc } from "./ipc";
import { isAllowedExternalURL, webPreferences } from "./security";
import { loadStore } from "./store";

/*
 * The SkillIssues desktop app's main process: one window, locked down
 * (security.ts webPreferences), talking to its page only through the IPC
 * contract (shared/contract.ts). The page can't navigate away, open windows,
 * ask for permissions, or reach the network itself (its CSP has no
 * connect-src beyond the dev server).
 */

if (!app.requestSingleInstanceLock()) app.quit();

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
    backgroundColor: "#ffffff",
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

app.on("second-instance", () => {
  if (!win) return;
  if (win.isMinimized()) win.restore();
  win.focus();
});

app.whenReady().then(() => {
  loadStore();
  session.defaultSession.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
  session.defaultSession.setPermissionCheckHandler(() => false);
  const registered = registerIpc(() => win, isAppURL);
  const missing = Object.values(channels).filter((c) => !registered.includes(c));
  if (missing.length) throw new Error(`IPC channels without a handler: ${missing.join(", ")}`);
  createWindow();
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

process.on("unhandledRejection", (err) => console.error("[main] unhandled rejection:", err));
