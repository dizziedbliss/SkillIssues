const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("skillIssuesDesktop", {
  apiRequest: (pathname, options) =>
    ipcRenderer.invoke("api:request", pathname, options),
  loginWithGitHub: () => ipcRenderer.invoke("auth:github"),
  loginWithToken: (token) => ipcRenderer.invoke("auth:github-token", token),
  loginDemo: () => ipcRenderer.invoke("auth:demo"),
  logout: () => ipcRenderer.invoke("auth:logout"),
  inspectWorkspace: (workspace) =>
    ipcRenderer.invoke("workspace:inspect", workspace),
  resolveRepositoryWorkspace: (repository) =>
    ipcRenderer.invoke("workspace:resolve-repository", repository),
  cloneRepository: (repositoryUrl) =>
    ipcRenderer.invoke("workspace:clone", repositoryUrl),
  openWorkspace: (workspace) => ipcRenderer.invoke("workspace:open", workspace),
  openInVSCode: (workspace) =>
    ipcRenderer.invoke("workspace:open-vscode", workspace),
  createBranch: (workspace, branch) =>
    ipcRenderer.invoke("workspace:create-branch", workspace, branch),
  pushBranch: (workspace, branch) =>
    ipcRenderer.invoke("workspace:push", workspace, branch),
  commitAndPush: (workspace, branch, message) =>
    ipcRenderer.invoke("workspace:commit-and-push", workspace, branch, message),
});
