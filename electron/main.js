const { app, BrowserWindow, dialog, ipcMain } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { execFile } = require('node:child_process');

function recordCommand(workspace, command, output = '') {
  const logDirectory = path.join(workspace, '.skillissues');
  fs.mkdirSync(logDirectory, { recursive: true });
  fs.appendFileSync(path.join(logDirectory, 'commands.log'), `${new Date().toISOString()} $ ${command}\n${output}\n`, 'utf8');
}

function validBranchName(branch) {
  return typeof branch === 'string' && /^[A-Za-z0-9][A-Za-z0-9._/-]{0,80}$/.test(branch) && !branch.includes('..');
}

function runGit(workspace, args, command) {
  if (!isSafeWorkspace(workspace)) return Promise.reject(new Error('Choose an existing Git workspace.'));
  return new Promise((resolve, reject) => execFile('git', ['-C', path.resolve(workspace), ...args], (error, stdout, stderr) => {
    if (error) return reject(new Error(stderr.trim() || 'Git command failed.'));
    recordCommand(path.resolve(workspace), command, stdout.trim());
    resolve({ path: path.resolve(workspace), output: stdout.trim() });
  }));
}

function isSafeWorkspace(workspace) {
  if (typeof workspace !== 'string' || workspace.length === 0) return false;
  const resolved = path.resolve(workspace);
  return fs.existsSync(resolved) && fs.statSync(resolved).isDirectory();
}

function createWindow() {
  const window = new BrowserWindow({ width: 1100, height: 760, webPreferences: { contextIsolation: true, sandbox: true, preload: path.join(__dirname, 'preload.js') } });
  window.loadFile(path.join(__dirname, 'renderer.html'));
}

ipcMain.handle('workspace:inspect', async (_, workspace) => {
  if (!isSafeWorkspace(workspace)) throw new Error('Choose an existing local workspace directory.');
  return new Promise((resolve, reject) => execFile('git', ['-C', path.resolve(workspace), 'status', '--short', '--branch'], (error, stdout) => {
    if (error) return reject(new Error('The selected directory is not a Git workspace.'));
    recordCommand(path.resolve(workspace), `git -C "${path.resolve(workspace)}" status --short --branch`, stdout.trim());
    resolve({ path: path.resolve(workspace), status: stdout.trim() });
  }));
});

ipcMain.handle('workspace:choose', async () => {
  const result = await dialog.showOpenDialog({ properties: ['openDirectory', 'createDirectory'] });
  return result.canceled ? null : result.filePaths[0];
});

ipcMain.handle('workspace:clone', async (_, repositoryUrl, destination) => {
  if (typeof repositoryUrl !== 'string' || !/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repositoryUrl)) {
    throw new Error('Only public GitHub repository URLs are supported.');
  }
  if (!isSafeWorkspace(destination)) throw new Error('Choose an existing destination directory.');
  const repositoryName = repositoryUrl.split('/').pop();
  const target = path.join(path.resolve(destination), repositoryName);
  if (fs.existsSync(target)) throw new Error('The repository destination already exists.');
  return new Promise((resolve, reject) => execFile('git', ['clone', '--', repositoryUrl, target], (error, stdout, stderr) => {
    if (error) return reject(new Error(stderr.trim() || 'Repository clone failed.'));
    recordCommand(target, `git clone "${repositoryUrl}" "${target}"`, stdout.trim());
    resolve({ path: target, output: stdout.trim() });
  }));
});

ipcMain.handle('workspace:create-branch', async (_, workspace, branch) => {
  if (!validBranchName(branch)) throw new Error('Use a simple branch name such as skillissues/fix-auth-errors.');
  return runGit(workspace, ['switch', '-c', branch], `git switch -c "${branch}"`);
});

ipcMain.handle('workspace:push', async (_, workspace, branch) => {
  if (!validBranchName(branch)) throw new Error('Invalid branch name.');
  const remote = await runGit(workspace, ['remote', 'get-url', 'origin'], 'git remote get-url origin');
  if (!/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+(?:\.git)?$/.test(remote.output.trim())) {
    throw new Error('Push stopped: origin is not a public GitHub repository URL.');
  }
  return runGit(workspace, ['push', '--set-upstream', 'origin', branch], `git push --set-upstream origin "${branch}"`);
});

app.whenReady().then(createWindow);
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit(); });
