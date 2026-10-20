import { dialog, shell, type BrowserWindow } from "electron";
import fs from "node:fs";
import path from "node:path";
import type { IssueRef, Settings, Workspace } from "../shared/contract";
import { git } from "./git";
import { fork, openPullRequest as createPullRequest } from "./github";
import { branchFor, cleanCommitMessage, githubRemote, isGitHubRemote, isValidBranch, isWithin, parseRepository } from "./security";
import { getSecrets, getWorkspace, putWorkspace, setWorkspaceRoot, workspaceRoot } from "./store";

/*
 * An issue's workspace: the person's fork of its repository, cloned into the
 * workspace folder, on a branch for the issue, with the original repository
 * as upstream. Every path is checked to be inside the workspace folder after
 * resolving links, and git only ever talks to github.com over HTTPS.
 */

function pushToken(): string {
  const { push } = getSecrets();
  if (!push) throw new Error("Connect GitHub for pushing first.");
  return push;
}

function ensureRoot(): string {
  const root = workspaceRoot();
  fs.mkdirSync(root, { recursive: true });
  return fs.realpathSync.native(root);
}

/** The workspace's folder, re-checked to be inside the root (it may have moved, or become a link). */
function checkedPath(ws: Workspace): string {
  const root = ensureRoot();
  let real: string;
  try {
    real = fs.realpathSync.native(ws.path);
  } catch {
    throw new Error(`The folder ${ws.path} is gone. Start work on the issue again.`);
  }
  if (!isWithin(root, real) || real === root) throw new Error("That folder is outside your workspace folder.");
  return real;
}

function known(issueId: number): Workspace {
  const ws = getWorkspace(issueId);
  if (!ws) throw new Error("There's no workspace for this issue yet.");
  return ws;
}

async function changes(dir: string): Promise<string[]> {
  const out = await git(dir, ["status", "--porcelain=v1"]);
  return out ? out.split("\n") : [];
}

export function settings(): Settings {
  return { workspaceRoot: workspaceRoot() };
}

export async function chooseRoot(win: BrowserWindow | null): Promise<Settings> {
  const opts: Electron.OpenDialogOptions = {
    title: "Choose where SkillIssues keeps your projects",
    buttonLabel: "Use this folder",
    defaultPath: workspaceRoot(),
    properties: ["openDirectory", "createDirectory"],
  };
  const result = win ? await dialog.showOpenDialog(win, opts) : await dialog.showOpenDialog(opts);
  const dir = result.filePaths[0];
  if (!result.canceled && dir) setWorkspaceRoot(fs.realpathSync.native(dir));
  return settings();
}

export async function get(issueId: number): Promise<Workspace | null> {
  const ws = getWorkspace(issueId);
  if (!ws) return null;
  try {
    return { ...ws, changes: await changes(checkedPath(ws)) };
  } catch {
    return { ...ws, changes: [] };
  }
}

/** Forks the issue's repository, clones the fork, adds upstream, and switches to the issue's branch. */
export async function start(issue: IssueRef): Promise<Workspace> {
  const repo = parseRepository(issue.repository);
  if (!repo || !Number.isInteger(issue.number) || issue.number <= 0 || !Number.isInteger(issue.id)) {
    throw new Error("That isn't an issue the app can work on.");
  }
  const token = pushToken();
  const root = ensureRoot();
  const forked = await fork(token, issue.repository);
  const dir = path.join(root, `${repo.owner}__${repo.name}`);
  if (!isWithin(root, dir) || dir === root) throw new Error("That repository name can't be used as a folder.");
  const branch = branchFor(issue.number, issue.title);

  if (fs.existsSync(path.join(dir, ".git"))) {
    const real = fs.realpathSync.native(dir);
    if (!isWithin(root, real)) throw new Error("That folder is outside your workspace folder.");
    const origin = await git(real, ["remote", "get-url", "origin"]);
    if (origin !== githubRemote(forked.fullName)) {
      throw new Error(`${dir} already holds a different clone (origin ${origin}). Move it away and try again.`);
    }
    await git(real, ["fetch", "--quiet", "origin"], token);
  } else {
    if (fs.existsSync(dir)) throw new Error(`${dir} exists and isn't a git clone. Move it away and try again.`);
    // A new fork can take a few seconds before it can be cloned.
    let lastError: unknown;
    for (let attempt = 0; attempt < 5; attempt++) {
      try {
        await git(root, ["clone", "--filter=blob:none", "--quiet", "--", githubRemote(forked.fullName), dir], token);
        lastError = null;
        break;
      } catch (err) {
        lastError = err;
        fs.rmSync(dir, { recursive: true, force: true });
        await new Promise((r) => setTimeout(r, 2000 * (attempt + 1)));
      }
    }
    if (lastError) throw lastError;
    await git(dir, ["remote", "add", "upstream", githubRemote(issue.repository)]);
  }

  const real = fs.realpathSync.native(dir);
  const exists = await git(real, ["branch", "--list", branch]);
  await git(real, exists ? ["switch", branch] : ["switch", "-c", branch]);

  const previous = getWorkspace(issue.id);
  const ws: Workspace = {
    issueId: issue.id,
    issueNumber: issue.number,
    title: issue.title,
    repository: issue.repository,
    fork: forked.fullName,
    path: real,
    branch,
    changes: await changes(real),
    pullRequestUrl: previous?.pullRequestUrl ?? null,
  };
  putWorkspace({ ...ws, changes: [] });
  return ws;
}

export async function openFolder(issueId: number): Promise<void> {
  const err = await shell.openPath(checkedPath(known(issueId)));
  if (err) throw new Error(err);
}

/** Opens the folder in VS Code through its own link scheme: no binary to find, no shell. */
export async function openEditor(issueId: number): Promise<void> {
  const dir = checkedPath(known(issueId));
  const href = `vscode://file/${dir.split(path.sep).map(encodeURIComponent).join("/").replace(/^\/?/, "/")}`;
  await shell.openExternal(href);
}

/** Commits everything changed (when anything is) and pushes the branch to the fork. */
export async function commitAndPush(issueId: number, rawMessage: string): Promise<Workspace> {
  const ws = known(issueId);
  const dir = checkedPath(ws);
  const message = cleanCommitMessage(rawMessage);
  if (!message) throw new Error("Write a commit message (up to 500 characters).");
  if (!isValidBranch(ws.branch)) throw new Error("This workspace's branch name isn't valid.");
  const origin = await git(dir, ["remote", "get-url", "origin"]);
  if (!isGitHubRemote(origin) || origin !== githubRemote(ws.fork)) {
    throw new Error("Push stopped: origin isn't your fork on GitHub any more.");
  }
  const current = await git(dir, ["branch", "--show-current"]);
  if (current !== ws.branch) throw new Error(`Switch back to ${ws.branch} first (you're on ${current || "a detached HEAD"}).`);
  if ((await changes(dir)).length > 0) {
    await git(dir, ["add", "--all"]);
    await git(dir, ["commit", "--quiet", "-m", message]);
  }
  await git(dir, ["push", "--quiet", "--set-upstream", "origin", ws.branch], pushToken());
  return { ...ws, path: dir, changes: await changes(dir) };
}

export async function openPullRequest(issueId: number): Promise<Workspace> {
  const ws = known(issueId);
  const forkOwner = parseRepository(ws.fork)!.owner;
  const body = `Closes #${ws.issueNumber}.\n\nOpened with SkillIssues.`;
  const url = await createPullRequest(pushToken(), ws.repository, forkOwner, ws.branch, ws.title, body);
  const next = { ...ws, pullRequestUrl: url };
  putWorkspace({ ...next, changes: [] });
  await shell.openExternal(url);
  return { ...next, changes: await changes(checkedPath(ws)).catch(() => []) };
}
