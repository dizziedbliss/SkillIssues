import { contextBridge, ipcRenderer } from "electron";
import { channels, type DesktopBridge } from "../shared/contract";

/*
 * The bridge, and all of it: window.skillissues exposes one method per
 * contract channel. The page gets no ipcRenderer, no event listeners, and no
 * way to name a channel itself.
 */

const invoke = (channel: string, ...args: unknown[]) => ipcRenderer.invoke(channel, ...args);

const bridge: DesktopBridge = {
  api: (call) => invoke(channels.api, call),
  authState: () => invoke(channels.authState),
  signInStart: () => invoke(channels.signInStart),
  signInWait: () => invoke(channels.signInWait),
  pushStart: () => invoke(channels.pushStart),
  pushWait: () => invoke(channels.pushWait),
  authCancel: () => invoke(channels.authCancel),
  pushDisconnect: () => invoke(channels.pushDisconnect),
  signOut: () => invoke(channels.signOut),
  deleteAccount: () => invoke(channels.deleteAccount),
  settings: () => invoke(channels.settings),
  chooseWorkspaceRoot: () => invoke(channels.chooseWorkspaceRoot),
  workspace: (issueId) => invoke(channels.workspace, issueId),
  startWork: (issue) => invoke(channels.startWork, issue),
  openFolder: (issueId) => invoke(channels.openFolder, issueId),
  openEditor: (issueId) => invoke(channels.openEditor, issueId),
  commitAndPush: (issueId, message) => invoke(channels.commitAndPush, issueId, message),
  openPullRequest: (issueId) => invoke(channels.openPullRequest, issueId),
  openExternal: (url) => invoke(channels.openExternal, url),
};

contextBridge.exposeInMainWorld("skillissues", bridge);
