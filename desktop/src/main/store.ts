import { app, safeStorage } from "electron";
import fs from "node:fs";
import path from "node:path";
import type { Workspace } from "../shared/contract";

/*
 * What the app keeps on disk, under the user's app-data folder:
 *
 * secrets.json — the SkillIssues session and the push token, encrypted with
 *   the OS keychain through safeStorage. Where the OS has no real encryption
 *   (Linux without a keyring falls back to a fixed key, "basic_text"), they
 *   are kept in memory only and the person signs in again next launch: a
 *   plaintext token on disk is not an option.
 * state.json — the workspace folder and each issue's workspace. No secrets.
 */

export interface Secrets {
  session: string | null;
  push: string | null;
  /** The GitHub login the push token belongs to, to check it matches the account. */
  pushLogin: string | null;
}

interface State {
  workspaceRoot: string | null;
  workspaces: Record<string, Workspace>;
}

const empty: Secrets = { session: null, push: null, pushLogin: null };

function file(name: string): string {
  return path.join(app.getPath("userData"), name);
}

/** True when safeStorage really encrypts (not Linux's basic_text fallback). */
export function canPersistSecrets(): boolean {
  if (!safeStorage.isEncryptionAvailable()) return false;
  if (process.platform === "linux") return safeStorage.getSelectedStorageBackend() !== "basic_text";
  return true;
}

let secrets: Secrets = { ...empty };
let state: State = { workspaceRoot: null, workspaces: {} };

function writeAtomic(target: string, text: string) {
  fs.mkdirSync(path.dirname(target), { recursive: true });
  const tmp = `${target}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, text, { mode: 0o600 });
  fs.renameSync(tmp, target);
}

export function loadStore(): void {
  try {
    const raw = JSON.parse(fs.readFileSync(file("state.json"), "utf8")) as Partial<State>;
    state = { workspaceRoot: raw.workspaceRoot ?? null, workspaces: raw.workspaces ?? {} };
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") console.error("[store] state.json unreadable, starting fresh:", err);
  }
  if (!canPersistSecrets()) return;
  try {
    const raw = JSON.parse(fs.readFileSync(file("secrets.json"), "utf8")) as { v?: number; data?: string };
    if (raw.v === 1 && typeof raw.data === "string") {
      const parsed = JSON.parse(safeStorage.decryptString(Buffer.from(raw.data, "base64"))) as Partial<Secrets>;
      secrets = { ...empty, ...parsed };
    }
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") console.error("[store] saved sign-in unreadable; sign in again.");
    secrets = { ...empty };
  }
}

export function getSecrets(): Secrets {
  return secrets;
}

export function setSecrets(next: Partial<Secrets>): void {
  secrets = { ...secrets, ...next };
  if (!canPersistSecrets()) return;
  const data = safeStorage.encryptString(JSON.stringify(secrets)).toString("base64");
  writeAtomic(file("secrets.json"), JSON.stringify({ v: 1, data }));
}

export function clearSecrets(): void {
  secrets = { ...empty };
  try {
    fs.rmSync(file("secrets.json"), { force: true });
  } catch (err) {
    console.error("[store] could not remove secrets.json:", err);
    throw new Error("Signed out here, but the saved sign-in file couldn't be removed.");
  }
}

function saveState() {
  writeAtomic(file("state.json"), JSON.stringify(state, null, 2));
}

export function workspaceRoot(): string {
  return state.workspaceRoot ?? path.join(app.getPath("documents"), "SkillIssues");
}

export function setWorkspaceRoot(dir: string): void {
  state.workspaceRoot = dir;
  saveState();
}

export function getWorkspace(issueId: number): Workspace | null {
  return state.workspaces[String(issueId)] ?? null;
}

export function putWorkspace(ws: Workspace): void {
  state.workspaces[String(ws.issueId)] = ws;
  saveState();
}
