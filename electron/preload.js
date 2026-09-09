const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('skillIssuesDesktop', {
  inspectWorkspace: (workspace) => ipcRenderer.invoke('workspace:inspect', workspace),
  cloneRepository: (repositoryUrl) => ipcRenderer.invoke('workspace:clone', repositoryUrl),
  openWorkspace: (workspace) => ipcRenderer.invoke('workspace:open', workspace),
  createBranch: (workspace, branch) => ipcRenderer.invoke('workspace:create-branch', workspace, branch),
  pushBranch: (workspace, branch) => ipcRenderer.invoke('workspace:push', workspace, branch),
});