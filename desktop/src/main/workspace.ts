import { dialog, shell, type BrowserWindow } from "electron";
import fs from "node:fs";
import path from "node:path";
import type { IssueRef, Settings, Workspace } from "../shared/contract";
import { git } from "./git";
import { fork, openPullRequest as createPullRequest, pullRequestState, repositoryExists } from "./github";
import type { CommitInput } from "../shared/contract";
import {
  branchFor, commitMessage, githubRemote, isGitHubRemote, isValidBranch, isWithin, parseRepository, pullRequestText, vscodeFolderLink,
} from "./security";
import { forgetWorkspace, getSecrets, getWorkspace, listWorkspaces, putWorkspace, setWorkspaceRoot, workspaceRoot } from "./store";

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

/** The default branch the work started from: stored, or origin's HEAD for older workspaces. */
async function baseOf(ws: Workspace, dir: string): Promise<string> {
  if (ws.base && isValidBranch(ws.base)) return ws.base;
  const head = await git(dir, ["symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"]).catch(() => "");
  const base = head.replace(/^origin\//, "");
  if (!isValidBranch(base)) throw new Error("Can't tell which branch this work started from. Start work on the issue again.");
  return base;
}

async function count(dir: string, range: string): Promise<number> {
  const n = Number(await git(dir, ["rev-list", "--count", range]));
  if (!Number.isInteger(n) || n < 0) throw new Error("git gave an unreadable commit count.");
  return n;
}

/** Where a workspace's git stands: uncommitted changes, commits, and commits not pushed yet. */
async function withGit(ws: Workspace): Promise<Workspace> {
  let dir: string;
  try {
    dir = checkedPath(ws);
  } catch (err) {
    return { ...ws, changes: [], commits: 0, unpushed: 0, problem: err instanceof Error ? err.message : String(err) };
  }
  const base = await baseOf(ws, dir);
  const commits = await count(dir, `origin/${base}..HEAD`);
  const pushed = await git(dir, ["rev-parse", "--verify", "--quiet", `refs/remotes/origin/${ws.branch}`]).catch(() => "");
  const unpushed = pushed ? await count(dir, `origin/${ws.branch}..HEAD`) : commits;
  return { ...ws, path: dir, base, changes: await changes(dir), commits, unpushed, problem: null };
}

/**
 * What GitHub says now: whether the fork is still there (deleted, there's
 * nothing to push to until it's forked again), and whether the pull request
 * is open, merged or closed. Without a push token, or when GitHub can't be
 * asked, neither is known: the check is skipped (and logged), and a push or
 * a pull request still says what went wrong.
 */
async function withFork(ws: Workspace): Promise<Workspace> {
  const { push } = getSecrets();
  if (!push || ws.problem) return ws;
  const [fork, pr] = await Promise.allSettled([
    repositoryExists(push, ws.fork),
    ws.pullRequestUrl ? pullRequestState(push, ws.pullRequestUrl) : Promise.resolve(undefined),
  ]);
  const out = { ...ws };
  if (fork.status === "fulfilled") out.forkMissing = !fork.value;
  else console.warn("[workspace] couldn't check the fork:", fork.reason instanceof Error ? fork.reason.message : fork.reason);
  if (pr.status === "fulfilled") out.pullRequestState = pr.value;
  else console.warn("[workspace] couldn't check the pull request:", pr.reason instanceof Error ? pr.reason.message : pr.reason);
  return out;
}

export async function get(issueId: number): Promise<Workspace | null> {
  const ws = getWorkspace(issueId);
  return ws ? withFork(await withGit(ws)) : null;
}

/** Every workspace, most recently started first. */
export async function list(): Promise<Workspace[]> {
  const all = await Promise.all(listWorkspaces().map(async (ws) => withFork(await withGit(ws))));
  return all.sort((a, b) => (b.startedAt ?? "").localeCompare(a.startedAt ?? "") || b.issueId - a.issueId);
}

export function forget(issueId: number): void {
  known(issueId);
  forgetWorkspace(issueId);
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
    base: isValidBranch(forked.defaultBranch) ? forked.defaultBranch : undefined,
    startedAt: previous?.startedAt ?? new Date().toISOString(),
    changes: [],
    commits: 0,
    unpushed: 0,
    problem: null,
    pullRequestUrl: previous?.pullRequestUrl ?? null,
  };
  putWorkspace(ws);
  return withGit(ws);
}

export async function openFolder(issueId: number): Promise<void> {
  const err = await shell.openPath(checkedPath(known(issueId)));
  if (err) throw new Error(err);
}

/** Opens the folder in VS Code through its own link scheme: no binary to find, no shell. */
export async function openEditor(issueId: number): Promise<void> {
  await shell.openExternal(vscodeFolderLink(checkedPath(known(issueId))));
}

/** Checks the workspace is still the fork, on its branch, before git writes anything. */
async function checkedForWrite(ws: Workspace): Promise<string> {
  const dir = checkedPath(ws);
  if (!isValidBranch(ws.branch)) throw new Error("This workspace's branch name isn't valid.");
  const origin = await git(dir, ["remote", "get-url", "origin"]);
  if (!isGitHubRemote(origin) || origin !== githubRemote(ws.fork)) {
    throw new Error("Stopped: origin isn't your fork on GitHub any more.");
  }
  const current = await git(dir, ["branch", "--show-current"]);
  if (current !== ws.branch) throw new Error(`Switch back to ${ws.branch} first (you're on ${current || "a detached HEAD"}).`);
  return dir;
}

/** Commits every change, with the title and description given. */
export async function commit(issueId: number, input: CommitInput): Promise<Workspace> {
  const ws = known(issueId);
  const message = commitMessage(input?.title, input?.body ?? "");
  if (!message) throw new Error("Give the commit a one-line title (up to 200 characters); the description is optional.");
  const dir = await checkedForWrite(ws);
  if ((await changes(dir)).length === 0) throw new Error("There's nothing to commit: change some files first.");
  await git(dir, ["add", "--all"]);
  // Each -m is a paragraph: the title, then the description after a blank line.
  const [title, ...rest] = message.split("\n\n");
  const body = rest.join("\n\n");
  await git(dir, ["commit", "--quiet", "-m", title!, ...(body ? ["-m", body] : [])]);
  return withGit(ws);
}

/** Pushes the branch to the fork: only commits, never uncommitted changes. */
export async function push(issueId: number): Promise<Workspace> {
  const ws = known(issueId);
  const dir = await checkedForWrite(ws);
  const state = await withGit(ws);
  if (state.changes.length > 0) throw new Error("You have changes that aren't committed. Commit them first, then push.");
  if (state.unpushed === 0) {
    throw new Error(state.commits === 0 ? "There's nothing to push yet: commit your changes first." : "Everything is already pushed.");
  }
  const token = pushToken();
  if (!(await repositoryExists(token, ws.fork))) {
    throw new Error(`Your fork ${ws.fork} is gone from GitHub. Fork it again, then push: your commits are still here.`);
  }
  await git(dir, ["push", "--quiet", "--set-upstream", "origin", ws.branch], token);
  return withGit(ws);
}

/**
 * Forks the repository again after the fork was deleted on GitHub: origin
 * points at the new fork, what the old one had is forgotten, and the old
 * pull request link goes (GitHub closes a pull request whose fork is
 * deleted). The commits in the folder stay, ready to push.
 */
export async function refork(issueId: number): Promise<Workspace> {
  const ws = known(issueId);
  const dir = checkedPath(ws);
  const token = pushToken();
  const forked = await fork(token, ws.repository);
  await git(dir, ["remote", "set-url", "origin", githubRemote(forked.fullName)]);
  // A new fork can take a few seconds before it answers.
  let lastError: unknown;
  for (let attempt = 0; attempt < 5; attempt++) {
    try {
      await git(dir, ["fetch", "--quiet", "--prune", "origin"], token);
      lastError = null;
      break;
    } catch (err) {
      lastError = err;
      await new Promise((r) => setTimeout(r, 2000 * (attempt + 1)));
    }
  }
  if (lastError) throw lastError;
  const next: Workspace = {
    ...ws,
    fork: forked.fullName,
    base: isValidBranch(forked.defaultBranch) ? forked.defaultBranch : ws.base,
    pullRequestUrl: null,
  };
  putWorkspace(next);
  return { ...(await withGit(next)), forkMissing: false };
}

/** Opens the pull request, as the person wrote it, once their commits are on the fork. */
export async function openPullRequest(issueId: number, input: unknown): Promise<Workspace> {
  const ws = known(issueId);
  const text = pullRequestText(input, ws.issueNumber);
  if (!text) throw new Error("Give the pull request a one-line title (up to 256 characters).");
  const state = await withGit(ws);
  if (state.problem) throw new Error(state.problem);
  if (state.commits === 0) throw new Error("There are no commits on this branch yet: commit and push your fix first.");
  if (state.unpushed > 0) throw new Error("Push your commits first, so the pull request has them.");
  const forkOwner = parseRepository(ws.fork)!.owner;
  const url = await createPullRequest(pushToken(), ws.repository, forkOwner, ws.branch, text.title, text.body);
  putWorkspace({ ...ws, pullRequestUrl: url });
  await shell.openExternal(url);
  return { ...state, pullRequestUrl: url, pullRequestState: "open" };
}
