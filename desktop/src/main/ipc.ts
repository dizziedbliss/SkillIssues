import { clipboard, ipcMain, nativeTheme, shell, type BrowserWindow, type IpcMainInvokeEvent } from "electron";
import { channels, type Channel, type CommitInput, type IssueRef, type Result } from "../shared/contract";
import { proxy } from "./api";
import { githubGet } from "./githubRead";
import { isClipboardText, isHexColor } from "./security";
import * as updater from "./updater";
import * as auth from "./auth";
import { isAllowedExternalURL } from "./security";
import * as workspace from "./workspace";

/*
 * One handler per contract channel, and nothing else. Each checks the call
 * came from the app's own page (not a frame or a page navigated elsewhere),
 * and turns any failure into a Result the renderer can show.
 */

type Handler = (...args: unknown[]) => unknown;

/** The title bar's height, here and in the page (desktop.module.css). */
export const TITLE_BAR_HEIGHT = 36;

export function registerIpc(
  window: () => BrowserWindow | null,
  isAppURL: (url: string) => boolean,
  takeLink: () => string | null,
): Channel[] {
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
  handle(channels.workspaces, () => workspace.list());
  handle(channels.commit, (id, input) => workspace.commit(issueId(id), input as CommitInput));
  handle(channels.push, (id) => workspace.push(issueId(id)));
  handle(channels.openPullRequest, (id, input) => workspace.openPullRequest(issueId(id), input));
  handle(channels.forgetWorkspace, (id) => workspace.forget(issueId(id)));
  // The page's own clipboard is off with every other permission; a code goes through here.
  handle(channels.copyText, (text) => {
    if (!isClipboardText(text)) throw new Error("That can't be copied.");
    clipboard.writeText(text);
  });
  handle(channels.githubGet, (url) => {
    if (typeof url !== "string") throw new Error("That GitHub request isn't allowed from the app.");
    return githubGet(url);
  });
  handle(channels.takeLink, () => takeLink());
  handle(channels.windowInfo, () => ({ platform: process.platform }));
  handle(channels.titleBar, (colors) => {
    const { symbol, dark } = (colors ?? {}) as { symbol?: unknown; dark?: unknown };
    if (typeof symbol !== "string" || !isHexColor(symbol) || typeof dark !== "boolean") throw new Error("Not a colour.");
    nativeTheme.themeSource = dark ? "dark" : "light";
    // macOS draws its own traffic lights; elsewhere the buttons overlay the page.
    if (process.platform !== "darwin") window()?.setTitleBarOverlay({ color: "#00000000", symbolColor: symbol, height: TITLE_BAR_HEIGHT });
  });
  handle(channels.updateState, () => updater.updateState());
  handle(channels.checkUpdates, () => updater.checkUpdates());
  handle(channels.installUpdate, () => updater.installUpdate());
  handle(channels.openExternal, async (url) => {
    if (typeof url !== "string" || !isAllowedExternalURL(url)) throw new Error("That link can't be opened from the app.");
    await shell.openExternal(url);
  });

  return registered;
}
