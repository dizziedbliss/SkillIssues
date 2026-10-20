import { ipcMain, shell, type BrowserWindow, type IpcMainInvokeEvent } from "electron";
import { channels, type Channel, type IssueRef, type Result } from "../shared/contract";
import { proxy } from "./api";
import * as auth from "./auth";
import { isAllowedExternalURL } from "./security";
import * as workspace from "./workspace";

/*
 * One handler per contract channel, and nothing else. Each checks the call
 * came from the app's own page (not a frame or a page navigated elsewhere),
 * and turns any failure into a Result the renderer can show.
 */

type Handler = (...args: unknown[]) => unknown;

export function registerIpc(window: () => BrowserWindow | null, isAppURL: (url: string) => boolean): Channel[] {
  const registered: Channel[] = [];

  const handle = (channel: Channel, fn: Handler) => {
    registered.push(channel);
    ipcMain.handle(channel, async (event: IpcMainInvokeEvent, ...args: unknown[]): Promise<Result<unknown>> => {
      const frame = event.senderFrame;
      if (!frame || frame.parent !== null || !isAppURL(frame.url)) {
        return { ok: false, error: "Refused: not from the app." };
      }
      try {
        return { ok: true, value: (await fn(...args)) ?? null };
      } catch (err) {
        return { ok: false, error: err instanceof Error ? err.message : String(err) };
      }
    });
  };

  const issueId = (v: unknown): number => {
    if (typeof v !== "number" || !Number.isInteger(v) || v <= 0) throw new Error("Not an issue.");
    return v;
  };

  handle(channels.api, (call) => proxy(call as Parameters<typeof proxy>[0]));
  handle(channels.authState, () => auth.state());
  handle(channels.signInStart, () => auth.signInStart());
  handle(channels.signInWait, () => auth.signInWait());
  handle(channels.pushStart, () => auth.pushStart());
  handle(channels.pushWait, () => auth.pushWait());
  handle(channels.authCancel, () => auth.cancel());
  handle(channels.pushDisconnect, () => auth.pushDisconnect());
  handle(channels.signOut, () => auth.signOut());
  handle(channels.deleteAccount, () => auth.deleteAccount());
  handle(channels.settings, () => workspace.settings());
  handle(channels.chooseWorkspaceRoot, () => workspace.chooseRoot(window()));
  handle(channels.workspace, (id) => workspace.get(issueId(id)));
  handle(channels.startWork, (issue) => {
    const i = issue as IssueRef;
    if (!i || typeof i.repository !== "string" || typeof i.title !== "string") throw new Error("Not an issue.");
    return workspace.start({ id: issueId(i.id), number: issueId(i.number), repository: i.repository, title: i.title.slice(0, 300) });
  });
  handle(channels.openFolder, (id) => workspace.openFolder(issueId(id)));
  handle(channels.openEditor, (id) => workspace.openEditor(issueId(id)));
  handle(channels.commitAndPush, (id, message) => workspace.commitAndPush(issueId(id), String(message ?? "")));
  handle(channels.openPullRequest, (id) => workspace.openPullRequest(issueId(id)));
  handle(channels.openExternal, async (url) => {
    if (typeof url !== "string" || !isAllowedExternalURL(url)) throw new Error("That link can't be opened from the app.");
    await shell.openExternal(url);
  });

  return registered;
}
