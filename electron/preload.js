const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('skillIssuesDesktop', {
  chooseWorkspace: () => ipcRenderer.invoke('workspace:choose'),
  inspectWorkspace: (workspace) => ipcRenderer.invoke('workspace:inspect', workspace),
  cloneRepository: (repositoryUrl, destination) => ipcRenderer.invoke('workspace:clone', repositoryUrl, destination),
});