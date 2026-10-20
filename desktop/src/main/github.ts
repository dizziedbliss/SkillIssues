import { parseRepository } from "./security";

/*
 * The GitHub calls the desktop app makes itself, with a token that stays on
 * this machine: the device flow for both tokens, and forking and opening pull
 * requests with the push token. The SkillIssues server never sees these.
 */

const GITHUB = "https://github.com";
const API = "https://api.github.com";
const USER_AGENT = "SkillIssues-Desktop";

export interface DeviceGrant {
  device_code: string;
  user_code: string;
  verification_uri: string;
  expires_in: number;
  interval: number;
}

async function form(url: string, body: Record<string, string>, signal?: AbortSignal): Promise<Record<string, unknown>> {
  const res = await fetch(url, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded", "User-Agent": USER_AGENT },
    body: new URLSearchParams(body).toString(),
    signal,
  });
  const data = (await res.json().catch(() => ({}))) as Record<string, unknown>;
  if (!res.ok && typeof data.error !== "string") throw new Error(`GitHub answered HTTP ${res.status}.`);
  return data;
}

/** Starts GitHub's device flow for the Login app. scope "" asks for nothing. */
export async function requestDeviceCode(clientId: string, scope: string): Promise<DeviceGrant> {
  const data = await form(`${GITHUB}/login/device/code`, { client_id: clientId, scope });
  if (data.error === "device_flow_disabled") {
    throw new Error("GitHub sign-in from the desktop app isn't switched on yet (the Login app needs Device Flow enabled).");
  }
  if (typeof data.device_code !== "string" || typeof data.user_code !== "string") {
    throw new Error(typeof data.error_description === "string" ? data.error_description : "GitHub didn't start the sign-in.");
  }
  return {
    device_code: data.device_code,
    user_code: data.user_code,
    verification_uri: typeof data.verification_uri === "string" ? data.verification_uri : `${GITHUB}/login/device`,
    expires_in: Number(data.expires_in) || 900,
    interval: Number(data.interval) || 5,
  };
}

/** Waits for the person to enter the code; resolves with the access token. */
export async function pollDeviceToken(clientId: string, grant: DeviceGrant, signal: AbortSignal): Promise<string> {
  let interval = grant.interval;
  const deadline = Date.now() + grant.expires_in * 1000;
  while (Date.now() < deadline) {
    await new Promise<void>((resolve, reject) => {
      const t = setTimeout(resolve, interval * 1000);
      signal.addEventListener("abort", () => (clearTimeout(t), reject(new Error("Sign-in cancelled."))), { once: true });
    });
    const data = await form(
      `${GITHUB}/login/oauth/access_token`,
      { client_id: clientId, device_code: grant.device_code, grant_type: "urn:ietf:params:oauth:grant-type:device_code" },
      signal,
    );
    if (typeof data.access_token === "string" && data.access_token) return data.access_token;
    switch (data.error) {
      case "authorization_pending":
        continue;
      case "slow_down":
        interval = Number(data.interval) || interval + 5;
        continue;
      case "access_denied":
        throw new Error("The sign-in was cancelled on GitHub.");
      case "expired_token":
        throw new Error("The code expired. Start again.");
      default:
        throw new Error(typeof data.error_description === "string" ? data.error_description : "GitHub refused the sign-in.");
    }
  }
  throw new Error("The code expired. Start again.");
}

async function api<T>(token: string, method: string, path: string, body?: unknown): Promise<{ status: number; data: T }> {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
      "User-Agent": USER_AGENT,
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = (await res.json().catch(() => ({}))) as T;
  return { status: res.status, data };
}

function message(data: unknown, fallback: string): string {
  const m = (data as { message?: unknown })?.message;
  return typeof m === "string" ? `${fallback}: ${m}` : fallback;
}

/** The account a token belongs to. */
export async function tokenOwner(token: string): Promise<string> {
  const { status, data } = await api<{ login?: string }>(token, "GET", "/user");
  if (status === 401) throw new Error("GitHub doesn't accept that token any more. Connect again.");
  if (status !== 200 || typeof data.login !== "string") throw new Error(message(data, `GitHub answered HTTP ${status}`));
  return data.login;
}

export interface RepoInfo {
  fullName: string;
  defaultBranch: string;
}

/** Forks repository into the token's account (or finds the fork already there). */
export async function fork(token: string, repository: string): Promise<RepoInfo> {
  const repo = parseRepository(repository);
  if (!repo) throw new Error(`Not a repository: ${repository}`);
  const { status, data } = await api<{ full_name?: string; default_branch?: string }>(
    token,
    "POST",
    `/repos/${repo.owner}/${repo.name}/forks`,
    { default_branch_only: false },
  );
  if ((status !== 202 && status !== 200) || typeof data.full_name !== "string") {
    throw new Error(message(data, `GitHub couldn't fork ${repository} (HTTP ${status})`));
  }
  if (!parseRepository(data.full_name)) throw new Error("GitHub returned an unexpected fork name.");
  return { fullName: data.full_name, defaultBranch: data.default_branch ?? "main" };
}

export async function defaultBranch(token: string, repository: string): Promise<string> {
  const { status, data } = await api<{ default_branch?: string }>(token, "GET", `/repos/${repository}`);
  if (status !== 200 || typeof data.default_branch !== "string") {
    throw new Error(message(data, `GitHub couldn't read ${repository} (HTTP ${status})`));
  }
  return data.default_branch;
}

/** Opens a pull request from fork:branch into repository's default branch; returns its URL. */
export async function openPullRequest(
  token: string,
  repository: string,
  forkOwner: string,
  branch: string,
  title: string,
  body: string,
): Promise<string> {
  const base = await defaultBranch(token, repository);
  const { status, data } = await api<{ html_url?: string; errors?: { message?: string }[] }>(
    token,
    "POST",
    `/repos/${repository}/pulls`,
    { title, body, head: `${forkOwner}:${branch}`, base, maintainer_can_modify: true },
  );
  if (status === 201 && typeof data.html_url === "string") return data.html_url;
  const existing = data.errors?.find((e) => e.message?.includes("already exists"));
  if (status === 422 && existing) {
    const list = await api<{ html_url?: string }[]>(
      token,
      "GET",
      `/repos/${repository}/pulls?head=${encodeURIComponent(`${forkOwner}:${branch}`)}&state=open`,
    );
    const url = Array.isArray(list.data) ? list.data[0]?.html_url : undefined;
    if (url) return url;
  }
  throw new Error(message(data, `GitHub couldn't open the pull request (HTTP ${status})`));
}
