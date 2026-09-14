/**
 * @file SkillIssues Electron Preload Script
 *
 * Exposes a secure, sandboxed IPC bridge (`window.skillIssuesDesktop`) to the renderer process.
 * Renderer JavaScript has no direct access to Node.js APIs (fs, child_process) or backend secrets.
 */

const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("skillIssuesDesktop", {
  /**
   * Proxy an authenticated HTTP request through the Electron main process to the SkillIssues API.
   * Automatically attaches session cookies/headers.
   * @param {string} pathname - API path, e.g. '/me' or '/issues'
   * @param {RequestInit} [options] - Fetch request options (method, body, headers)
   * @returns {Promise<any>} Parsed JSON response from the API
   */
  apiRequest: (pathname, options) =>
    ipcRenderer.invoke("api:request", pathname, options),

  /**
   * Initiate standard browser-based GitHub OAuth authentication.
   * @returns {Promise<any>} Authenticated user profile payload
   */
  loginWithGitHub: () => ipcRenderer.invoke("auth:github"),

  /**
   * Authenticate directly with a GitHub Personal Access Token (PAT).
   * @param {string} token - GitHub PAT with repo scope
   * @returns {Promise<any>} Authenticated user profile payload
   */
  loginWithToken: (token) => ipcRenderer.invoke("auth:github-token", token),

  /**
   * Sign in using demo development credentials (when DEMO_MODE is enabled on API).
   * @returns {Promise<any>} Demo user profile payload
   */
  loginDemo: () => ipcRenderer.invoke("auth:demo"),

  /**
   * Invalidate the current session and clear local session state.
   * @returns {Promise<void>}
   */
  logout: () => ipcRenderer.invoke("auth:logout"),

  /**
   * Inspect a local repository workspace's current Git status (branch, dirty status).
   * @param {string} workspace - Absolute path to local repository directory
   * @returns {Promise<{exists: boolean, branch: string, status: string, isClean: boolean}>}
   */
  inspectWorkspace: (workspace) =>
    ipcRenderer.invoke("workspace:inspect", workspace),

  /**
   * Resolve the canonical local workspace filesystem path for a given repository.
   * Clones are strictly confined to `~/Documents/SkillIssues/<repo>`.
   * @param {string} repository - Repository full name, e.g. 'tiangolo/fastapi'
   * @returns {Promise<string>} Canonical workspace folder path
   */
  resolveRepositoryWorkspace: (repository) =>
    ipcRenderer.invoke("workspace:resolve-repository", repository),

  /**
   * Clone a public GitHub repository into the sandboxed `Documents/SkillIssues` workspace directory.
   * @param {string} repositoryUrl - Public GitHub HTTPS clone URL
   * @returns {Promise<{workspace: string, stdout: string}>}
   */
  cloneRepository: (repositoryUrl) =>
    ipcRenderer.invoke("workspace:clone", repositoryUrl),

  /**
   * Open the repository workspace folder in the system file explorer (Finder / Explorer).
   * @param {string} workspace - Absolute path to workspace directory
   * @returns {Promise<void>}
   */
  openWorkspace: (workspace) => ipcRenderer.invoke("workspace:open", workspace),

  /**
   * Launch the repository workspace directory directly in Visual Studio Code (`code .`).
   * @param {string} workspace - Absolute path to workspace directory
   * @returns {Promise<void>}
   */
  openInVSCode: (workspace) =>
    ipcRenderer.invoke("workspace:open-vscode", workspace),

  /**
   * Create and check out a new Git branch (e.g. `skillissues/issue-<id>`).
   * @param {string} workspace - Absolute path to workspace directory
   * @param {string} branch - Validated branch name string
   * @returns {Promise<{command: string, output: string}>}
   */
  createBranch: (workspace, branch) =>
    ipcRenderer.invoke("workspace:create-branch", workspace, branch),

  /**
   * Push a local feature branch to the remote repository (`git push --set-upstream origin <branch>`).
   * @param {string} workspace - Absolute path to workspace directory
   * @param {string} branch - Branch name to push
   * @returns {Promise<{command: string, output: string}>}
   */
  pushBranch: (workspace, branch) =>
    ipcRenderer.invoke("workspace:push", workspace, branch),

  /**
   * Stage all modified files, commit with a message, and push upstream in one seamless operation.
   * @param {string} workspace - Absolute path to workspace directory
   * @param {string} branch - Branch name to push
   * @param {string} message - Git commit message
   * @returns {Promise<{command: string, output: string}>}
   */
  commitAndPush: (workspace, branch, message) =>
    ipcRenderer.invoke("workspace:commit-and-push", workspace, branch, message),

  /**
   * Register a callback listener for incoming protocol deep links (`skillissues://...`).
   * @param {(url: string) => void} callback - Handler function receiving deep link URL
   */
  onOpenDeepLink: (callback) =>
    ipcRenderer.on("open-deeplink", (_event, url) => callback(url)),
});
