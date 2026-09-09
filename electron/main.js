const { app, BrowserWindow, dialog, ipcMain } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { execFile } = require('node:child_process');

function isSafeWorkspace(workspace) {
  if (typeof workspace !== 'string' || workspace.length === 0) return false;
  const resolved = path.resolve(workspace);
  return !resolved.includes('..') && fs.existsSync(resolved) && fs.statSync(resolved).isDirectory();
}

function createWindow() {
  const window = new BrowserWindow({ width: 1100, height: 760, webPreferences: { contextIsolation: true, sandbox: true } });
  window.loadURL(process.env.FRONTEND_URL || 'http://localhost:5173');
}

ipcMain.handle('workspace:inspect', async (_, workspace) => {
  if (!isSafeWorkspace(workspace)) throw new Error('Choose an existing local workspace directory.');
  return new Promise((resolve, reject) => execFile('git', ['-C', path.resolve(workspace), 'status', '--short --branch'], (error, stdout) => {
    if (error) return reject(new Error('The selected directory is not a Git workspace.'));
    resolve({ path: path.resolve(workspace), status: stdout.trim() });
  }));
});

ipcMain.handle('workspace:choose', async () => {
  const result = await dialog.showOpenDialog({ properties: ['openDirectory', 'createDirectory'] });
  return result.canceled ? null : result.filePaths[0];
});

app.whenReady().then(createWindow);
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit(); });
