const { app, BrowserWindow, ipcMain, shell, session } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');

const execFileAsync = promisify(execFile);
const GITHUB_REPOSITORY_URL = /^https:\/\/github\.com\/([A-Za-z0-9_.-]+)\/([A-Za-z0-9_.-]+?)(?:\.git)?\/?$/;
const SAFE_EXTERNAL_URL = /^https:\/\/(?:github\.com|www\.github\.com)(?:\/|$)/;
const COMMAND_TIMEOUT_MS = 120000;
const MAX_OUTPUT_BYTES = 1024 * 1024;
const API_URL = process.env.SKILLISSUES_API_URL || 'http://localhost:8000';
let apiSessionToken = '';

function logRuntimeError(scope, error) {
  const message = error instanceof Error ? error.stack || error.message : String(error);
  console.error(`[${scope}] ${message}`);
}

function friendlyGitError(error) {
  const raw = error.stderr?.trim() || '';
  if (/403|permission to .* denied|unable to access/i.test(raw)) {
    return 'GitHub rejected the push. You cannot push directly to the upstream repository. Fork it on GitHub, update origin to your fork, then push again.';
  }
  if (/authentication failed|could not read Username/i.test(raw)) {
    return 'GitHub authentication failed. Sign in to GitHub and configure Git credentials before pushing.';
  }
  return raw || (error.killed ? 'Git command timed out.' : 'Git command failed.');
}

async function apiRequest(pathname, options = {}, retry = true) {
  const headers = { Accept: 'application/json', ...(options.headers || {}) };
  if (apiSessionToken) headers.Authorization = `Bearer ${apiSessionToken}`;
  const response = await fetch(`${API_URL}${pathname}`, { ...options, headers });
  const cookie = response.headers.get('set-cookie') || '';
  const match = cookie.match(/skillissues_session=([^;]+)/);
  if (match) apiSessionToken = match[1];
  if (response.status === 401 && retry && pathname !== '/auth/demo') {
    await apiRequest('/auth/demo', { method: 'POST' }, false);
    return apiRequest(pathname, options, false);
  }
  return response;
}

async function apiJson(pathname, options = {}) {
  const response = await apiRequest(pathname, options);
  const body = await response.text();
  let data = {};
  try { data = body ? JSON.parse(body) : {}; } catch { data = { detail: body }; }
  if (!response.ok) throw new Error(data.detail || `API request failed (${response.status})`);
  return data;
}

if (!app.requestSingleInstanceLock()) {
  process.exit(0);
}

function recordCommand(workspace, command, output = '') {
  const logDirectory = path.join(workspace, '.skillissues');
  fs.mkdirSync(logDirectory, { recursive: true });
  fs.appendFileSync(path.join(logDirectory, 'commands.log'), `${new Date().toISOString()} $ ${command}\n${output.slice(0, MAX_OUTPUT_BYTES)}\n`, { encoding: 'utf8', mode: 0o600 });
}

function validBranchName(branch) {
  return typeof branch === 'string' && /^[A-Za-z0-9][A-Za-z0-9._/-]{0,80}$/.test(branch) && !branch.includes('..');
}

function workspaceRoot() {
  const root = path.join(app.getPath('documents'), 'SkillIssues');
  fs.mkdirSync(root, { recursive: true });
  return root;
}

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

async function runGit(workspace, args, command) {
  if (!isSafeWorkspace(workspace)) return Promise.reject(new Error('Use a SkillIssues workspace under Documents.'));
  try {
    const result = await execFileAsync('git', ['-C', path.resolve(workspace), ...args], {
      timeout: COMMAND_TIMEOUT_MS,
      maxBuffer: MAX_OUTPUT_BYTES,
      windowsHide: true,
    });
    recordCommand(path.resolve(workspace), command, result.stdout.trim());
    return { path: path.resolve(workspace), output: result.stdout.trim() };
  } catch (error) {
    const message = friendlyGitError(error);
    recordCommand(path.resolve(workspace), command, message);
    throw new Error(message);
  }
}

function isSafeWorkspace(workspace) {
  if (typeof workspace !== 'string' || workspace.length === 0) return false;
  const resolved = path.resolve(workspace);
  return isWithinRoot(resolved) && fs.existsSync(resolved) && fs.lstatSync(resolved).isDirectory();
}

function createWindow() {
  const window = new BrowserWindow({
    width: 1100,
    height: 760,
    minWidth: 860,
    minHeight: 640,
    show: false,
    backgroundColor: '#f7f2fd',
    webPreferences: { contextIsolation: true, sandbox: true, nodeIntegration: false, webSecurity: true, preload: path.join(__dirname, 'preload.js') },
  });
  window.once('ready-to-show', () => window.show());
  window.webContents.setWindowOpenHandler(({ url }) => {
    if (SAFE_EXTERNAL_URL.test(url)) void shell.openExternal(url);
    return { action: 'deny' };
  });
  window.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith('file://')) event.preventDefault();
  });
  window.loadFile(path.join(__dirname, 'renderer.html'));
  return window;
}

ipcMain.handle('workspace:inspect', async (_, workspace) => {
  if (!isSafeWorkspace(workspace)) throw new Error('Use a SkillIssues workspace under Documents.');
  try {
    const result = await runGit(workspace, ['status', '--short', '--branch'], 'git status --short --branch');
    return { path: result.path, status: result.output };
  } catch {
    throw new Error('The workspace is not a Git repository.');
  }
});

ipcMain.handle('workspace:resolve-repository', async (_, repository) => {
  if (typeof repository !== 'string' || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)) throw new Error('Invalid repository name.');
  const repositoryName = repository.split('/')[1];
  const workspace = path.join(workspaceRoot(), repositoryName);
  if (!isSafeWorkspace(workspace)) throw new Error(`No local clone found at Documents/SkillIssues/${repositoryName}. Solve the issue and clone it first.`);
  return { path: workspace };
});

ipcMain.handle('api:request', async (_, pathname, options = {}) => {
  if (typeof pathname !== 'string' || !pathname.startsWith('/') || pathname.startsWith('//')) throw new Error('Invalid API path.');
  return apiJson(pathname, options);
});

ipcMain.handle('auth:github', async () => {
  const authWindow = new BrowserWindow({ width: 560, height: 760, parent: BrowserWindow.getFocusedWindow() || undefined, modal: true, webPreferences: { contextIsolation: true, sandbox: true } });
  const loginResponse = await fetch(`${API_URL}/auth/github`, { headers: { Accept: 'application/json' } });
  const login = await loginResponse.json();
  if (!login.authorize_url) throw new Error(login.message || 'GitHub login is not configured.');
  const stateCookie = (loginResponse.headers.get('set-cookie') || '').match(/skillissues_oauth_state=([^;]+)/);
  if (stateCookie) {
    await authWindow.webContents.session.cookies.set({ url: `${API_URL}/`, name: 'skillissues_oauth_state', value: stateCookie[1], httpOnly: true, sameSite: 'lax', expirationDate: Math.floor(Date.now() / 1000) + 600 });
  }
  await authWindow.loadURL(login.authorize_url);
  return new Promise((resolve) => {
    let finished = false;
    const complete = async () => {
      if (finished) return;
      finished = true;
      const cookies = await authWindow.webContents.session.cookies.get({ url: `${API_URL}/` });
      const sessionCookie = cookies.find((cookie) => cookie.name === 'skillissues_session');
      if (sessionCookie) apiSessionToken = sessionCookie.value;
      if (!authWindow.isDestroyed()) authWindow.close();
      resolve({ authenticated: Boolean(sessionCookie) });
    };
    authWindow.webContents.on('did-navigate', (_event, url) => { if (url.startsWith(process.env.FRONTEND_URL || 'http://localhost:5173')) void complete(); });
    authWindow.on('closed', () => { if (!finished) { finished = true; resolve({ authenticated: false }); } });
  });
});

ipcMain.handle('auth:demo', async () => {
  await apiJson('/auth/demo', { method: 'POST' });
  return { authenticated: true };
});

ipcMain.handle('auth:logout', async () => {
  try { await apiJson('/auth/logout', { method: 'POST' }); } finally { apiSessionToken = ''; }
  return { loggedOut: true };
});

ipcMain.handle('workspace:clone', async (_, repositoryUrl) => {
  const match = typeof repositoryUrl === 'string' && repositoryUrl.match(GITHUB_REPOSITORY_URL);
  if (!match) {
    throw new Error('Only public GitHub repository URLs are supported.');
  }
  const destination = workspaceRoot();
  let cloneUrl = repositoryUrl;
  try {
    const fork = await apiJson('/github/forks', { method: 'POST', body: JSON.stringify({ repository: `${match[1]}/${match[2]}` }), headers: { 'Content-Type': 'application/json' } });
    cloneUrl = fork.clone_url;
  } catch (error) {
    if (/public_repo|Connect GitHub|fork/i.test(error.message)) throw error;
  }
  const repositoryName = match[2];
  const target = path.join(path.resolve(destination), repositoryName);
  if (!isWithinRoot(target)) throw new Error('Invalid repository destination.');
  if (fs.existsSync(target)) {
    if (isSafeWorkspace(target)) return { path: target, output: 'Using existing local clone.', reused: true };
    throw new Error('A non-Git folder already exists at the repository destination. Remove it or choose a different repository.');
  }
  try {
    const result = await execFileAsync('git', ['clone', '--', cloneUrl, target], { timeout: COMMAND_TIMEOUT_MS, maxBuffer: MAX_OUTPUT_BYTES, windowsHide: true });
    recordCommand(target, `git clone "${cloneUrl}"`, result.stdout.trim());
    return { path: target, output: result.stdout.trim() };
  } catch (error) {
    if (fs.existsSync(target)) fs.rmSync(target, { recursive: true, force: true });
    throw new Error(error.stderr?.trim() || (error.killed ? 'Repository clone timed out.' : 'Repository clone failed.'));
  }
});

ipcMain.handle('workspace:open', async (_, workspace) => {
  if (!isSafeWorkspace(workspace)) throw new Error('Invalid SkillIssues workspace.');
  const error = await shell.openPath(path.resolve(workspace));
  if (error) throw new Error(error);
  return { opened: true };
});

ipcMain.handle('workspace:open-vscode', async (_, workspace) => {
  if (!isSafeWorkspace(workspace)) throw new Error('Invalid SkillIssues workspace.');
  try {
    await execFileAsync('code', [path.resolve(workspace)], { timeout: 15000, windowsHide: true });
    return { opened: true };
  } catch {
    throw new Error('VS Code command not found. Install VS Code and enable the `code` shell command.');
  }
});

ipcMain.handle('workspace:create-branch', async (_, workspace, branch) => {
  if (!validBranchName(branch)) throw new Error('Use a simple branch name such as skillissues/fix-auth-errors.');
  try {
    return await runGit(workspace, ['switch', '-c', branch], `git switch -c "${branch}"`);
  } catch (error) {
    if (/branch .* already exists/i.test(error.message)) {
      return runGit(workspace, ['switch', branch], `git switch "${branch}"`);
    }
    throw error;
  }
});

ipcMain.handle('workspace:push', async (_, workspace, branch) => {
  if (!validBranchName(branch)) throw new Error('Invalid branch name.');
  const remote = await runGit(workspace, ['remote', 'get-url', 'origin'], 'git remote get-url origin');
  if (!/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+(?:\.git)?$/.test(remote.output.trim())) {
    throw new Error('Push stopped: origin is not a public GitHub repository URL.');
  }
  const credentials = await apiJson('/github/git-token');
  const auth = Buffer.from(`x-access-token:${credentials.token}`).toString('base64');
  return runGit(workspace, ['-c', `http.extraheader=AUTHORIZATION: basic ${auth}`, 'push', '--set-upstream', 'origin', branch], `git push --set-upstream origin "${branch}"`);
});

app.whenReady().then(() => {
  session.defaultSession.setPermissionRequestHandler((_webContents, _permission, callback) => callback(false));
  createWindow();
});
process.on('uncaughtException', (error) => logRuntimeError('main:uncaughtException', error));
process.on('unhandledRejection', (error) => logRuntimeError('main:unhandledRejection', error));
app.on('render-process-gone', (_event, _webContents, details) => logRuntimeError('renderer:gone', details));
app.on('second-instance', () => BrowserWindow.getAllWindows()[0]?.show());
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit(); });
