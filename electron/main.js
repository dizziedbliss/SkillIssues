/**
 * @file SkillIssues Electron Main Process
 *
 * Manages the desktop window lifecycle, secure sandboxed Git operations,
 * local workspace isolation in ~/Documents/SkillIssues, protocol deep linking,
 * and authenticated API communications.
 */

const { app, BrowserWindow, ipcMain, shell, session } = require("electron");
const path = require("node:path");
const fs = require("node:fs");
const { execFile } = require("node:child_process");
const { promisify } = require("node:util");

const execFileAsync = promisify(execFile);

// ==============================================================================
// Configuration & Security Constants
// ==============================================================================

const GITHUB_REPOSITORY_URL =
  /^https:\/\/github\.com\/([A-Za-z0-9_.-]+)\/([A-Za-z0-9_.-]+?)(?:\.git)?\/?$/;
const SAFE_EXTERNAL_URL = /^https:\/\/(?:github\.com|www\.github\.com)(?:\/|$)/;
const COMMAND_TIMEOUT_MS = 120000;
const MAX_OUTPUT_BYTES = 1024 * 1024;
const API_URL = process.env.SKILLISSUES_API_URL || "http://localhost:8000";
let apiSessionToken = "";

function getSessionFilePath() {
  try {
    return path.join(app.getPath("userData"), "session.json");
  } catch {
    return null;
  }
}

function loadSavedSession() {
  const sessionFile = getSessionFilePath();
  if (!sessionFile) return;
  try {
    if (fs.existsSync(sessionFile)) {
      const data = JSON.parse(fs.readFileSync(sessionFile, "utf8"));
      if (data && typeof data.token === "string" && data.token.trim()) {
        apiSessionToken = data.token.trim();
      }
    }
  } catch (err) {
    logRuntimeError("loadSavedSession", err);
  }
}

function saveSession(token) {
  apiSessionToken = token || "";
  const sessionFile = getSessionFilePath();
  if (!sessionFile) return;
  try {
    fs.mkdirSync(path.dirname(sessionFile), { recursive: true });
    fs.writeFileSync(sessionFile, JSON.stringify({ token: apiSessionToken }), "utf8");
  } catch (err) {
    logRuntimeError("saveSession", err);
  }
}

function clearSavedSession() {
  apiSessionToken = "";
  const sessionFile = getSessionFilePath();
  if (!sessionFile) return;
  try {
    if (fs.existsSync(sessionFile)) {
      fs.unlinkSync(sessionFile);
    }
  } catch (err) {
    logRuntimeError("clearSavedSession", err);
  }
}

// Enforce single-instance lock to prevent concurrent conflicting processes
if (!app.requestSingleInstanceLock()) {
  process.exit(0);
}

// ==============================================================================
// Error Handling & Git Diagnostics
// ==============================================================================

/**
 * Log runtime exceptions with diagnostic context.
 * @param {string} scope - Context or subsystem identifier
 * @param {any} error - Exception object or message
 */
function logRuntimeError(scope, error) {
  const message =
    error instanceof Error ? error.stack || error.message : String(error);
  console.error(`[${scope}] ${message}`);
}

/**
 * Translate raw Git stderr output into clear, actionable advice for developers.
 * @param {any} error - Git process error object
 * @returns {string} Human-friendly error description
 */
function friendlyGitError(error) {
  const raw = error.stderr?.trim() || "";
  if (/403|permission to .* denied|unable to access/i.test(raw)) {
    return "GitHub rejected the push. You cannot push directly to the upstream repository. Fork it on GitHub, update origin to your fork, then push again.";
  }
  if (/authentication failed|could not read Username/i.test(raw)) {
    return "GitHub authentication failed. Sign in to GitHub and configure Git credentials before pushing.";
  }
  return (
    raw || (error.killed ? "Git command timed out." : "Git command failed.")
  );
}

// ==============================================================================
// Authenticated API Proxying
// ==============================================================================

/**
 * Send an authenticated HTTP request to the SkillIssues backend API.
 * Automatically manages session token persistence from cookies/headers.
 * @param {string} pathname - API endpoint path
 * @param {RequestInit} [options] - Fetch configuration options
 * @returns {Promise<Response>}
 */
async function apiRequest(pathname, options = {}) {
  const headers = { Accept: "application/json", ...(options.headers || {}) };
  if (apiSessionToken) headers.Authorization = `Bearer ${apiSessionToken}`;
  const response = await fetch(`${API_URL}${pathname}`, {
    ...options,
    headers,
  });
  const cookie = response.headers.get("set-cookie") || "";
  const match = cookie.match(/skillissues_session=([^;]+)/);
  if (match) apiSessionToken = match[1];
  return response;
}

/**
 * Perform an API request and parse JSON payload, throwing descriptive errors on failure.
 * @param {string} pathname - API endpoint path
 * @param {RequestInit} [options] - Fetch configuration options
 * @returns {Promise<any>} Parsed response data
 */
async function apiJson(pathname, options = {}) {
  const response = await apiRequest(pathname, options);
  const body = await response.text();
  let data = {};
  try {
    data = body ? JSON.parse(body) : {};
  } catch {
    data = { detail: body };
  }
  if (!response.ok)
    throw new Error(data.detail || `API request failed (${response.status})`);
  return data;
}

// ==============================================================================
// Workspace Sandboxing & Safe Git Execution
// ==============================================================================

/**
 * Append executed Git commands and diagnostic logs to `.skillissues/commands.log`.
 * @param {string} workspace - Local repository workspace path
 * @param {string} command - Git command string representation
 * @param {string} [output] - Command output or error text
 */
function recordCommand(workspace, command, output = "") {
  const logDirectory = path.join(workspace, ".skillissues");
  fs.mkdirSync(logDirectory, { recursive: true });
  fs.appendFileSync(
    path.join(logDirectory, "commands.log"),
    `${new Date().toISOString()} $ ${command}\n${output.slice(0, MAX_OUTPUT_BYTES)}\n`,
    { encoding: "utf8", mode: 0o600 },
  );
}

/**
 * Validate branch names to prevent command injection and ensure Git ref compliance.
 * Allows letters, digits, '.', '_', '/', '#', and '-' up to 80 characters.
 * @param {string} branch - Proposed branch name
 * @returns {boolean}
 */
function validBranchName(branch) {
  return (
    typeof branch === "string" &&
    /^[A-Za-z0-9][A-Za-z0-9._/#-]{0,80}$/.test(branch) &&
    !branch.includes("..")
  );
}

/**
 * Resolve the designated workspace root directory (`~/Documents/SkillIssues`).
 * @returns {string} Absolute path to workspace root
 */
function workspaceRoot() {
  const root = path.join(app.getPath("documents"), "SkillIssues");
  fs.mkdirSync(root, { recursive: true });
  return root;
}

/**
 * Ensure a candidate directory path is strictly located within the designated workspace root.
 * Prevents directory traversal attacks (`..`).
 * @param {string} candidate - Directory path to verify
 * @returns {boolean}
 */
function isWithinRoot(candidate) {
  const root = fs.realpathSync.native(workspaceRoot());
  let resolved;
  try {
    resolved = fs.realpathSync.native(candidate);
  } catch {
    resolved = path.resolve(candidate);
  }
  return resolved === root || resolved.startsWith(`${root}${path.sep}`);
}

/**
 * Validate that a workspace path exists, is a directory, and is confined to the safe root.
 * @param {string} workspace - Candidate workspace path
 * @returns {boolean}
 */
function isSafeWorkspace(workspace) {
  if (typeof workspace !== "string" || workspace.length === 0) return false;
  const resolved = path.resolve(workspace);
  return (
    isWithinRoot(resolved) &&
    fs.existsSync(resolved) &&
    fs.lstatSync(resolved).isDirectory()
  );
}

/**
 * Execute a bounded Git command in the given safe workspace.
 * @param {string} workspace - Workspace directory
 * @param {string[]} args - Argument list passed to git CLI
 * @param {string} command - Human-readable command description for logging
 * @returns {Promise<{path: string, output: string}>}
 */
async function runGit(workspace, args, command) {
  if (!isSafeWorkspace(workspace))
    return Promise.reject(
      new Error("Use a SkillIssues workspace under Documents."),
    );
  try {
    const result = await execFileAsync(
      "git",
      ["-C", path.resolve(workspace), ...args],
      {
        timeout: COMMAND_TIMEOUT_MS,
        maxBuffer: MAX_OUTPUT_BYTES,
        windowsHide: true,
      },
    );
    recordCommand(path.resolve(workspace), command, result.stdout.trim());
    return { path: path.resolve(workspace), output: result.stdout.trim() };
  } catch (error) {
    const message = friendlyGitError(error);
    recordCommand(path.resolve(workspace), command, message);
    throw new Error(message);
  }
}

// ==============================================================================
// Window Management & Protocol Handling
// ==============================================================================

let mainWindow = null;
let pendingDeepLinkUrl = null;

// Register custom URL protocol client for deep linking (skillissues://)
if (process.defaultApp) {
  if (process.argv.length >= 2) {
    app.setAsDefaultProtocolClient("skillissues", process.execPath, [
      path.resolve(process.argv[1]),
    ]);
  }
} else {
  app.setAsDefaultProtocolClient("skillissues");
}

function sendDeepLinkToWindow(url) {
  if (!url || typeof url !== "string") return;
  if (mainWindow && mainWindow.webContents) {
    mainWindow.webContents.send("open-deeplink", url);
  } else {
    pendingDeepLinkUrl = url;
  }
}

/**
 * Create and configure the primary application window with strict security defaults.
 * @returns {BrowserWindow}
 */
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 760,
    minWidth: 860,
    minHeight: 640,
    show: false,
    icon: path.join(__dirname, "icon.png"),
    backgroundColor: "#f7f2fd",
    webPreferences: {
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      webSecurity: true,
      preload: path.join(__dirname, "preload.js"),
    },
  });

  mainWindow.once("ready-to-show", () => {
    mainWindow.show();
    if (pendingDeepLinkUrl) {
      sendDeepLinkToWindow(pendingDeepLinkUrl);
      pendingDeepLinkUrl = null;
    }
  });

  // Confine external link clicks to GitHub URLs opened in the system default browser
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (SAFE_EXTERNAL_URL.test(url)) void shell.openExternal(url);
    return { action: "deny" };
  });

  mainWindow.webContents.on("will-navigate", (event, url) => {
    if (!url.startsWith("file://")) event.preventDefault();
  });

  mainWindow.loadFile(path.join(__dirname, "renderer.html"));
  return mainWindow;
}

// ==============================================================================
// IPC Handlers: Workspace Inspection & Resolution
// ==============================================================================

ipcMain.handle("workspace:inspect", async (_, workspace) => {
  if (!isSafeWorkspace(workspace))
    throw new Error("Use a SkillIssues workspace under Documents.");
  try {
    const result = await runGit(
      workspace,
      ["status", "--short", "--branch"],
      "git status --short --branch",
    );
    return { path: result.path, status: result.output };
  } catch {
    throw new Error("The workspace is not a Git repository.");
  }
});

ipcMain.handle("workspace:resolve-repository", async (_, repository) => {
  if (
    typeof repository !== "string" ||
    !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)
  )
    throw new Error("Invalid repository name.");
  const repositoryName = repository.split("/")[1];
  const workspace = path.join(workspaceRoot(), repositoryName);
  if (!isSafeWorkspace(workspace))
    throw new Error(
      `No local clone found at Documents/SkillIssues/${repositoryName}. Solve the issue and clone it first.`,
    );
  return { path: workspace };
});

ipcMain.handle("api:request", async (_, pathname, options = {}) => {
  if (
    typeof pathname !== "string" ||
    !pathname.startsWith("/") ||
    pathname.startsWith("//")
  )
    throw new Error("Invalid API path.");
  return apiJson(pathname, options);
});

// ==============================================================================
// IPC Handlers: Authentication
// ==============================================================================

ipcMain.handle("auth:github", async () => {
  try {
    const loginResponse = await fetch(`${API_URL}/auth/github`, {
      headers: { Accept: "application/json" },
    });
    if (!loginResponse.ok) {
      throw new Error(`API returned HTTP ${loginResponse.status}`);
    }
    const login = await loginResponse.json();
    if (!login.authorize_url) {
      return {
        authenticated: false,
        message: login.message || login.error || "GitHub OAuth is not configured on the API server.",
      };
    }
    const authWindow = new BrowserWindow({
      width: 560,
      height: 760,
      parent: mainWindow || BrowserWindow.getFocusedWindow() || undefined,
      modal: true,
      autoHideMenuBar: true,
      webPreferences: { contextIsolation: true, sandbox: true },
    });
    await authWindow.loadURL(login.authorize_url);

    return new Promise((resolve) => {
      let finished = false;

      const closeAuthWindow = () => {
        try {
          if (!authWindow.isDestroyed()) {
            authWindow.removeAllListeners("closed");
            authWindow.destroy();
          }
        } catch {
          // ignore destroy errors
        }
        if (mainWindow && !mainWindow.isDestroyed()) {
          mainWindow.show();
          mainWindow.focus();
        }
      };

      const complete = async (tokenFromNav) => {
        if (finished) return;
        finished = true;

        if (tokenFromNav) {
          saveSession(tokenFromNav);
        }

        if (!apiSessionToken) {
          try {
            const cookies = await authWindow.webContents.session.cookies.get({
              url: API_URL,
              name: "skillissues_session",
            });
            if (cookies && cookies[0] && cookies[0].value) {
              saveSession(cookies[0].value);
            }
          } catch {
            // ignore cookie read error
          }
        }

        if (!apiSessionToken) {
          try {
            const cookies = await authWindow.webContents.session.cookies.get({
              name: "skillissues_session",
            });
            if (cookies && cookies[0] && cookies[0].value) {
              saveSession(cookies[0].value);
            }
          } catch {
            // ignore cookie read error
          }
        }

        closeAuthWindow();

        if (!apiSessionToken) {
          resolve({ authenticated: false, message: "Authentication was not completed." });
          return;
        }

        try {
          const user = await apiJson("/me");
          resolve({ authenticated: true, user });
        } catch {
          resolve({ authenticated: true, user: { username: "connected" } });
        }
      };

      const handleUrl = (targetUrl, event) => {
        if (!targetUrl || finished) return;
        try {
          const parsed = new URL(targetUrl);
          const token = parsed.searchParams.get("token");
          const error = parsed.searchParams.get("error");
          const errorDesc = parsed.searchParams.get("error_description");

          if (error) {
            if (event && typeof event.preventDefault === "function") {
              event.preventDefault();
            }
            finished = true;
            closeAuthWindow();
            resolve({
              authenticated: false,
              message: errorDesc ? `${error}: ${errorDesc}` : error,
            });
            return;
          }

          if (token) {
            if (event && typeof event.preventDefault === "function") {
              event.preventDefault();
            }
            void complete(token);
            return;
          }

          const frontendBase = (process.env.FRONTEND_URL || "http://localhost:5173").replace(/\/$/, "");
          if (targetUrl.startsWith(frontendBase)) {
            if (event && typeof event.preventDefault === "function") {
              event.preventDefault();
            }
            void complete();
          }
        } catch {
          // ignore invalid url
        }
      };

      const onNav = (event, url) => handleUrl(url, event);
      authWindow.webContents.on("will-redirect", onNav);
      authWindow.webContents.on("will-navigate", onNav);
      authWindow.webContents.on("did-redirect-navigation", onNav);
      authWindow.webContents.on("did-navigate", onNav);

      authWindow.on("closed", () => {
        if (!finished) {
          finished = true;
          if (mainWindow && !mainWindow.isDestroyed()) {
            mainWindow.focus();
          }
          if (apiSessionToken) {
            apiJson("/me")
              .then((user) => resolve({ authenticated: true, user }))
              .catch(() => resolve({ authenticated: true, user: { username: "connected" } }));
          } else {
            resolve({ authenticated: false, message: "Sign in window was closed." });
          }
        }
      });
    });
  } catch (err) {
    return {
      authenticated: false,
      message: err instanceof Error ? err.message : "Failed to initiate GitHub authentication.",
    };
  }
});

ipcMain.handle("auth:github-token", async (_, token) => {
  if (typeof token !== "string" || !token.trim())
    throw new Error("Enter a valid GitHub Personal Access Token.");
  const data = await apiJson("/auth/github/token", {
    method: "POST",
    body: JSON.stringify({ token: token.trim() }),
    headers: { "Content-Type": "application/json" },
  });
  if (data.session_token) {
    saveSession(data.session_token);
  }
  return { authenticated: true, user: data.user };
});

ipcMain.handle("auth:demo", async () => {
  const data = await apiJson("/auth/demo", { method: "POST" });
  if (data && data.session_token) {
    saveSession(data.session_token);
  }
  return { authenticated: true };
});

ipcMain.handle("auth:logout", async () => {
  try {
    await apiJson("/auth/purge", { method: "POST" });
    await apiJson("/auth/logout", { method: "POST" });
    await session.defaultSession.clearStorageData({ storages: ["cookies"] });
  } catch {
    /* ignore logout network errors */
  } finally {
    clearSavedSession();
  }
  return { loggedOut: true };
});

// ==============================================================================
// IPC Handlers: Repository Cloning & Workspace Launchers
// ==============================================================================

ipcMain.handle("workspace:clone", async (_, repositoryUrl) => {
  const match =
    typeof repositoryUrl === "string" &&
    repositoryUrl.match(GITHUB_REPOSITORY_URL);
  if (!match) {
    throw new Error("Only public GitHub repository URLs are supported.");
  }
  const destination = workspaceRoot();
  let cloneUrl = repositoryUrl;
  let isFork = false;
  try {
    const fork = await apiJson("/github/forks", {
      method: "POST",
      body: JSON.stringify({ repository: `${match[1]}/${match[2]}` }),
      headers: { "Content-Type": "application/json" },
    });
    if (fork && fork.clone_url) {
      cloneUrl = fork.clone_url;
      isFork = true;
    }
  } catch (error) {
    throw new Error(
      error.message ||
        'Please click "Connect GitHub" before starting a challenge.',
    );
  }
  const repositoryName = match[2];
  const target = path.join(path.resolve(destination), repositoryName);
  if (!isWithinRoot(target)) throw new Error("Invalid repository destination.");
  if (fs.existsSync(target)) {
    if (isSafeWorkspace(target)) {
      if (isFork) {
        try {
          await runGit(
            target,
            ["remote", "set-url", "origin", cloneUrl],
            `git remote set-url origin "${cloneUrl}"`,
          );
        } catch {
          /* ignore remote update error */
        }
      }
      return {
        path: target,
        output: `Using local clone at ${target} (origin -> ${cloneUrl})`,
        reused: true,
      };
    }
    throw new Error(
      "A non-Git folder already exists at the repository destination. Remove it or choose a different repository.",
    );
  }
  try {
    const result = await execFileAsync(
      "git",
      ["clone", "--", cloneUrl, target],
      {
        timeout: COMMAND_TIMEOUT_MS,
        maxBuffer: MAX_OUTPUT_BYTES,
        windowsHide: true,
      },
    );
    recordCommand(target, `git clone "${cloneUrl}"`, result.stdout.trim());
    if (isFork) {
      try {
        await runGit(
          target,
          ["remote", "set-url", "origin", cloneUrl],
          `git remote set-url origin "${cloneUrl}"`,
        );
      } catch {
        /* ignore remote update error */
      }
    }
    return { path: target, output: result.stdout.trim() };
  } catch (error) {
    if (fs.existsSync(target))
      fs.rmSync(target, { recursive: true, force: true });
    throw new Error(
      error.stderr?.trim() ||
        (error.killed
          ? "Repository clone timed out."
          : "Repository clone failed."),
    );
  }
});

ipcMain.handle("workspace:open", async (_, workspace) => {
  const root = workspaceRoot();
  let target =
    typeof workspace === "string" && workspace.trim()
      ? path.resolve(workspace)
      : root;
  if (!fs.existsSync(target)) {
    fs.mkdirSync(target, { recursive: true });
  }
  if (!isWithinRoot(target)) target = root;
  const error = await shell.openPath(target);
  if (error) throw new Error(error);
  return { opened: true, path: target };
});

ipcMain.handle("workspace:open-vscode", async (_, workspace) => {
  const root = workspaceRoot();
  let target =
    typeof workspace === "string" && workspace.trim()
      ? path.resolve(workspace)
      : root;
  if (!fs.existsSync(target)) {
    fs.mkdirSync(target, { recursive: true });
  }
  if (!isWithinRoot(target)) target = root;

  const localAppData = process.env.LOCALAPPDATA || "";
  const programFiles = process.env["ProgramFiles"] || "C:\\Program Files";
  const programFilesX86 =
    process.env["ProgramFiles(x86)"] || "C:\\Program Files (x86)";

  // Standard platform binary candidates for Microsoft VS Code
  const candidates = [
    { bin: "code.cmd", options: { shell: true } },
    { bin: "code", options: { shell: process.platform === "win32" } },
    {
      bin: path.join(localAppData, "Programs", "Microsoft VS Code", "Code.exe"),
      options: {},
    },
    {
      bin: path.join(
        localAppData,
        "Programs",
        "Microsoft VS Code",
        "bin",
        "code.cmd",
      ),
      options: { shell: true },
    },
    {
      bin: path.join(programFiles, "Microsoft VS Code", "Code.exe"),
      options: {},
    },
    {
      bin: path.join(programFiles, "Microsoft VS Code", "bin", "code.cmd"),
      options: { shell: true },
    },
    {
      bin: path.join(programFilesX86, "Microsoft VS Code", "Code.exe"),
      options: {},
    },
    { bin: "/usr/local/bin/code", options: {} },
    { bin: "/usr/bin/code", options: {} },
  ];

  for (const candidate of candidates) {
    if (!candidate.bin) continue;
    if (candidate.bin.includes(path.sep) && !fs.existsSync(candidate.bin))
      continue;
    try {
      await execFileAsync(candidate.bin, [target], {
        timeout: 15000,
        windowsHide: true,
        ...candidate.options,
      });
      return { opened: true, message: `Opened ${target} in VS Code.` };
    } catch {
      // Continue to next executable candidate
    }
  }

  // Fallback to opening folder in system file explorer if VS Code executable is missing
  await shell.openPath(target);
  return {
    opened: true,
    fallback: true,
    message: `VS Code binary not found in standard paths. Opened ${target} in File Explorer.`,
  };
});

// ==============================================================================
// IPC Handlers: Git Branch & Push Operations
// ==============================================================================

ipcMain.handle("workspace:create-branch", async (_, workspace, branch) => {
  if (!validBranchName(branch))
    throw new Error(
      "Use a simple branch name such as skillissues/fix-auth-errors.",
    );
  try {
    return await runGit(
      workspace,
      ["switch", "-c", branch],
      `git switch -c "${branch}"`,
    );
  } catch (error) {
    if (/branch .* already exists/i.test(error.message)) {
      return runGit(workspace, ["switch", branch], `git switch "${branch}"`);
    }
    throw error;
  }
});

ipcMain.handle("workspace:push", async (_, workspace, branch) => {
  if (!validBranchName(branch)) throw new Error("Invalid branch name.");
  const remote = await runGit(
    workspace,
    ["remote", "get-url", "origin"],
    "git remote get-url origin",
  );
  if (
    !/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+(?:\.git)?$/.test(
      remote.output.trim(),
    )
  ) {
    throw new Error(
      "Push stopped: origin is not a public GitHub repository URL.",
    );
  }
  const credentials = await apiJson("/github/git-token");
  const auth = Buffer.from(`x-access-token:${credentials.token}`).toString(
    "base64",
  );
  return runGit(
    workspace,
    [
      "-c",
      `http.extraheader=AUTHORIZATION: basic ${auth}`,
      "push",
      "--set-upstream",
      "origin",
      branch,
    ],
    `git push --set-upstream origin "${branch}"`,
  );
});

ipcMain.handle(
  "workspace:commit-and-push",
  async (_, workspace, branch, commitMessage) => {
    if (!validBranchName(branch)) throw new Error("Invalid branch name.");
    if (!isSafeWorkspace(workspace)) throw new Error("Invalid workspace.");
    try {
      const status = await runGit(
        workspace,
        ["status", "--porcelain"],
        "git status --porcelain",
      );
      if (status.output.trim().length > 0) {
        const msg =
          (commitMessage && commitMessage.trim()) ||
          `SkillIssues: update for ${branch}`;
        await runGit(workspace, ["add", "."], "git add .");
        await runGit(
          workspace,
          ["commit", "-m", msg],
          `git commit -m "${msg}"`,
        );
      }
    } catch {
      /* proceed if commit not required */
    }
    const remote = await runGit(
      workspace,
      ["remote", "get-url", "origin"],
      "git remote get-url origin",
    );
    if (
      !/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+(?:\.git)?$/.test(
        remote.output.trim(),
      )
    ) {
      throw new Error(
        "Push stopped: origin is not a public GitHub repository URL.",
      );
    }
    const credentials = await apiJson("/github/git-token");
    const auth = Buffer.from(`x-access-token:${credentials.token}`).toString(
      "base64",
    );
    return runGit(
      workspace,
      [
        "-c",
        `http.extraheader=AUTHORIZATION: basic ${auth}`,
        "push",
        "--set-upstream",
        "origin",
        branch,
      ],
      `git push --set-upstream origin "${branch}"`,
    );
  },
);

// ==============================================================================
// App Lifecycle & Process Event Listeners
// ==============================================================================

app.whenReady().then(() => {
  loadSavedSession();
  session.defaultSession.setPermissionRequestHandler(
    (_webContents, _permission, callback) => callback(false),
  );
  createWindow();
});

process.on("uncaughtException", (error) =>
  logRuntimeError("main:uncaughtException", error),
);
process.on("unhandledRejection", (error) =>
  logRuntimeError("main:unhandledRejection", error),
);
app.on("render-process-gone", (_event, _webContents, details) =>
  logRuntimeError("renderer:gone", details),
);

// Handle second instance activation (focus existing window and pass deep link)
app.on("second-instance", (event, commandLine) => {
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.focus();
  }
  const deepLink = (commandLine || []).find(
    (arg) => typeof arg === "string" && arg.startsWith("skillissues://"),
  );
  if (deepLink) sendDeepLinkToWindow(deepLink);
});

// macOS protocol open-url handler
app.on("open-url", (event, url) => {
  event.preventDefault();
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.focus();
  }
  sendDeepLinkToWindow(url);
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
