import { shell } from "electron";
import type { AuthState, DeviceCode, User } from "../shared/contract";
import { request } from "./api";
import { pollDeviceToken, requestDeviceCode, tokenOwner, type DeviceGrant } from "./github";
import { canPersistSecrets, clearSecrets, getSecrets, setSecrets } from "./store";

/*
 * Two GitHub device flows on the Login app, kept apart:
 *
 * Sign-in asks for no scopes. Its token is posted once to POST
 * /auth/github/device, which checks it was issued to the Login app, reads who
 * it belongs to and drops it; the app keeps only the SkillIssues session.
 *
 * Pushing asks for public_repo, to fork, push and open pull requests. That
 * token never leaves this machine, and must belong to the signed-in account.
 */

const PUSH_SCOPE = "public_repo";

let user: User | null = null;
let pending: { kind: "signin" | "push"; grant: DeviceGrant; clientId: string; abort: AbortController } | null = null;

async function clientId(): Promise<string> {
  const reply = await request("GET", "/auth/github/device", undefined, null);
  if (!reply.ok) throw new Error(reply.error);
  const cfg = reply.data as { enabled?: boolean; client_id?: string };
  if (!cfg.enabled || !cfg.client_id) throw new Error("Sign-in isn't available on the SkillIssues server right now.");
  return cfg.client_id;
}

export async function state(): Promise<AuthState> {
  const { session, push } = getSecrets();
  if (session && !user) {
    const reply = await request("GET", "/auth/session");
    if (reply.ok && (reply.data as { authenticated?: boolean }).authenticated) {
      user = (reply.data as { user: User }).user;
    } else if (reply.ok || reply.status === 401) {
      // The server no longer knows this session (expired, signed out everywhere).
      clearSecrets();
    }
  }
  const signedIn = Boolean(getSecrets().session && user);
  return { signedIn, user: signedIn ? user : null, pushConnected: signedIn && Boolean(push), persisted: canPersistSecrets() };
}

async function start(kind: "signin" | "push"): Promise<DeviceCode> {
  pending?.abort.abort();
  const id = await clientId();
  const grant = await requestDeviceCode(id, kind === "push" ? PUSH_SCOPE : "");
  pending = { kind, grant, clientId: id, abort: new AbortController() };
  await shell.openExternal(grant.verification_uri);
  return { user_code: grant.user_code, verification_uri: grant.verification_uri, expires_in: grant.expires_in };
}

async function wait(kind: "signin" | "push"): Promise<string> {
  if (!pending || pending.kind !== kind) throw new Error("Start again: there is no code waiting.");
  const { grant, clientId: id, abort } = pending;
  try {
    return await pollDeviceToken(id, grant, abort.signal);
  } finally {
    if (pending?.grant === grant) pending = null;
  }
}

export const signInStart = () => start("signin");
export const pushStart = () => start("push");

export function cancel(): void {
  pending?.abort.abort();
  pending = null;
}

export async function signInWait(): Promise<AuthState> {
  const token = await wait("signin");
  const reply = await request("POST", "/auth/github/device", { access_token: token }, null);
  if (!reply.ok) throw new Error(reply.error);
  const data = reply.data as { session_token?: string; user?: User };
  if (!data.session_token || !data.user) throw new Error("The server didn't start a session.");
  user = data.user;
  setSecrets({ session: data.session_token });
  return state();
}

export async function pushWait(): Promise<AuthState> {
  const token = await wait("push");
  const login = await tokenOwner(token);
  if (!user || login.toLowerCase() !== user.username.toLowerCase()) {
    throw new Error(`That GitHub account is ${login}, but you're signed in as ${user?.username ?? "nobody"}. Connect the same account.`);
  }
  setSecrets({ push: token, pushLogin: login });
  return state();
}

export async function pushDisconnect(): Promise<AuthState> {
  setSecrets({ push: null, pushLogin: null });
  return state();
}

export async function signOut(): Promise<AuthState> {
  const reply = await request("POST", "/auth/logout", {});
  user = null;
  clearSecrets();
  if (!reply.ok && reply.status !== 401) {
    throw new Error(`Signed out on this computer, but the server didn't end the session: ${reply.error}`);
  }
  return state();
}

export async function deleteAccount(): Promise<{ authorized_apps_url: string }> {
  const reply = await request("POST", "/auth/purge", { confirm: true });
  if (!reply.ok) throw new Error(reply.error);
  user = null;
  clearSecrets();
  return { authorized_apps_url: (reply.data as { authorized_apps_url?: string }).authorized_apps_url ?? "https://github.com/settings/applications" };
}
