import path from "node:path";

/*
 * The desktop app's security rules, as pure functions with no Electron in
 * them, so security.test.ts pins every one. The main process applies them at
 * each boundary: what the window may do, what the renderer may ask the API,
 * which links leave the app, and where git may run.
 */

/** BrowserWindow webPreferences. Never loosen one of these. */
export const webPreferences = {
  contextIsolation: true,
  sandbox: true,
  nodeIntegration: false,
  nodeIntegrationInWorker: false,
  webSecurity: true,
  allowRunningInsecureContent: false,
  webviewTag: false,
  spellcheck: false,
} as const;

/**
 * Links the app hands to the system browser: GitHub pages, the SkillIssues
 * site, and VS Code's own scheme for a folder (checked separately against the
 * workspace root). Anything else stays closed.
 */
export function isAllowedExternalURL(raw: string): boolean {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return false;
  }
  if (url.username || url.password) return false;
  if (url.protocol !== "https:") return false;
  return url.hostname === "github.com" || url.hostname === "skillissues.dizziedbliss.lol";
}

/**
 * API paths the renderer may reach through the main process. Sign-in
 * (/auth/github/device) is the main process's own business and is not on it,
 * nor is anything operational.
 */
const apiPaths: { method: string; pattern: RegExp }[] = [
  { method: "GET", pattern: /^\/issues(\/facets|\/\d+(\/body)?)?$/ },
  { method: "GET", pattern: /^\/stats$/ },
  { method: "GET", pattern: /^\/repositories\/lookup$/ },
  { method: "GET", pattern: /^\/auth\/session$/ },
  { method: "GET", pattern: /^\/me$/ },
  { method: "GET", pattern: /^\/saved$/ },
  { method: "POST", pattern: /^\/saved$/ },
  { method: "DELETE", pattern: /^\/saved\/\d+$/ },
  { method: "GET", pattern: /^\/notifications$/ },
  { method: "POST", pattern: /^\/notifications(\/\d+\/read|\/read-all)$/ },
  { method: "GET", pattern: /^\/themes(\/[0-9a-f-]{36})?$/ },
  { method: "POST", pattern: /^\/themes$/ },
  { method: "PUT", pattern: /^\/themes\/[0-9a-f-]{36}$/ },
  { method: "DELETE", pattern: /^\/themes\/[0-9a-f-]{36}$/ },
  { method: "GET", pattern: /^\/me\/theme$/ },
  { method: "PUT", pattern: /^\/me\/theme$/ },
];

export function isAllowedApiCall(method: string, rawPath: string): boolean {
  if (typeof rawPath !== "string" || !rawPath.startsWith("/") || rawPath.startsWith("//")) return false;
  if (rawPath.includes("\\") || rawPath.includes("#")) return false;
  const [pathname] = rawPath.split("?", 1);
  if (!pathname || pathname.split("/").some((seg) => seg === ".." || seg === ".")) return false;
  return apiPaths.some((p) => p.method === method && p.pattern.test(pathname));
}

/** owner/name as GitHub allows it. */
const repoName = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})\/[A-Za-z0-9._-]{1,100}$/;

export function parseRepository(fullName: string): { owner: string; name: string } | null {
  if (typeof fullName !== "string" || !repoName.test(fullName)) return null;
  const [owner, name] = fullName.split("/") as [string, string];
  if (name === "." || name === "..") return null;
  return { owner, name };
}

/** The only remotes git may push to or clone from: a GitHub repository over HTTPS. */
export function githubRemote(fullName: string): string {
  if (!parseRepository(fullName)) throw new Error(`Not a GitHub repository name: ${fullName}`);
  return `https://github.com/${fullName}.git`;
}

export function isGitHubRemote(url: string): boolean {
  const m = /^https:\/\/github\.com\/([^/]+\/[^/]+?)(?:\.git)?$/.exec(url.trim());
  return m !== null && parseRepository(m[1]!) !== null;
}

/** A branch name git accepts and no shell or option parser can misread. */
export function isValidBranch(branch: string): boolean {
  return (
    typeof branch === "string" &&
    /^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$/.test(branch) &&
    !branch.includes("..") &&
    !branch.includes("//") &&
    !branch.endsWith("/") &&
    !branch.endsWith(".lock") &&
    !branch.endsWith(".")
  );
}

/** The branch for an issue: skillissues/<number>-<a few words of the title>. */
export function branchFor(issueNumber: number, title: string): string {
  const slug = title
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .split("-")
    .slice(0, 6)
    .join("-")
    .slice(0, 48)
    .replace(/-+$/, "");
  const branch = `skillissues/${issueNumber}${slug ? `-${slug}` : ""}`;
  return isValidBranch(branch) ? branch : `skillissues/${issueNumber}`;
}

/**
 * Whether candidate is root itself or inside it, by path alone. Callers
 * resolve symlinks first (realpath) when the path exists, so a link inside the
 * root cannot point out of it.
 */
export function isWithin(root: string, candidate: string, platform: NodeJS.Platform = process.platform): boolean {
  const p = platform === "win32" ? path.win32 : path.posix;
  const fold = (s: string) => (platform === "win32" ? s.toLowerCase() : s);
  const r = fold(p.resolve(root));
  const c = fold(p.resolve(candidate));
  return c === r || c.startsWith(r.endsWith(p.sep) ? r : r + p.sep);
}

/** A commit message: one to 500 characters, no NUL. */
export function cleanCommitMessage(message: string): string | null {
  if (typeof message !== "string") return null;
  const m = message.replace(/\r\n/g, "\n").trim();
  if (!m || m.length > 500 || m.includes("\0")) return null;
  return m;
}

/**
 * Removes a token from text before it is shown or logged: the token itself
 * and the Basic credential git sends it in. Never log a token, a prefix of
 * one, or its length (CLAUDE.md).
 */
export function redact(text: string, token: string | null | undefined): string {
  if (!token) return text;
  const basic = Buffer.from(`x-access-token:${token}`).toString("base64");
  return text.split(token).join("[token]").split(basic).join("[token]");
}

/**
 * The environment for one git run: no prompts, and the push token, if any,
 * as an http.extraheader for github.com only. Passed through the environment
 * because argv is readable by any local process.
 */
export function gitEnv(token: string | null, base: NodeJS.ProcessEnv = process.env): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ...base, GIT_TERMINAL_PROMPT: "0", GCM_INTERACTIVE: "never" };
  // Drop any config the parent passed this way, so ours is the only entry.
  for (const key of Object.keys(env)) {
    if (/^GIT_CONFIG_(COUNT|KEY_\d+|VALUE_\d+|PARAMETERS)$/.test(key)) delete env[key];
  }
  if (token) {
    const basic = Buffer.from(`x-access-token:${token}`).toString("base64");
    env.GIT_CONFIG_COUNT = "1";
    env.GIT_CONFIG_KEY_0 = "http.https://github.com/.extraheader";
    env.GIT_CONFIG_VALUE_0 = `AUTHORIZATION: basic ${basic}`;
  }
  return env;
}
