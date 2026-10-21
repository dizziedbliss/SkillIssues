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
  { method: "GET", pattern: /^\/themes\/public$/ },
  { method: "POST", pattern: /^\/themes\/validate$/ },
  { method: "PUT", pattern: /^\/profile$/ },
  { method: "GET", pattern: /^\/preferences\/options$/ },
  { method: "GET", pattern: /^\/recommendations$/ },
  { method: "POST", pattern: /^\/reports$/ },
  { method: "GET", pattern: /^\/me\/(xp|pulls|claims|path)$/ },
  { method: "POST", pattern: /^\/me\/pulls\/refresh$/ },
  { method: "POST", pattern: /^\/me\/path$/ },
  { method: "DELETE", pattern: /^\/me\/path$/ },
  { method: "POST", pattern: /^\/me\/path\/steps\/\d+\/swap$/ },
  { method: "GET", pattern: /^\/paths\/roles$/ },
  { method: "GET", pattern: /^\/issues\/\d+\/claim$/ },
  { method: "POST", pattern: /^\/issues\/\d+\/claim$/ },
  { method: "DELETE", pattern: /^\/issues\/\d+\/claim$/ },
  { method: "GET", pattern: /^\/users\/[A-Za-z0-9-]{1,39}$/ },
  { method: "GET", pattern: /^\/leaderboards$/ },
  { method: "POST", pattern: /^\/auth\/logout-all$/ },
];

/**
 * Public GitHub API reads the page may ask the main process for, made with no
 * credentials: an issue, its timeline, and pull request search (the web app
 * reads the same from the browser). Nothing else, and never a write.
 */
const githubReads: RegExp[] = [
  /^\/repos\/[A-Za-z0-9-]{1,39}\/[A-Za-z0-9._-]{1,100}\/issues\/\d{1,9}$/,
  /^\/repos\/[A-Za-z0-9-]{1,39}\/[A-Za-z0-9._-]{1,100}\/issues\/\d{1,9}\/timeline$/,
  /^\/search\/issues$/,
];

export function isAllowedGitHubRead(raw: string): boolean {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return false;
  }
  if (url.protocol !== "https:" || url.hostname !== "api.github.com" || url.port || url.username || url.password || url.hash) {
    return false;
  }
  if (url.pathname.split("/").some((seg) => seg === ".." || seg === ".")) return false;
  return githubReads.some((p) => p.test(url.pathname));
}

/**
 * The app page a skillissues:// link opens: skillissues://issue/<id> (the
 * site's "Work on it in the desktop app"). Anything else is ignored.
 */
export function deepLinkPath(raw: string): string | null {
  const m = /^skillissues:\/\/issue\/(\d{1,9})\/?(?:\?.*)?$/.exec(typeof raw === "string" ? raw.trim() : "");
  return m && Number(m[1]) > 0 ? `/app/issues/${Number(m[1])}` : null;
}

export function isAllowedApiCall(method: string, rawPath: string): boolean {
  if (typeof rawPath !== "string" || !rawPath.startsWith("/") || rawPath.startsWith("//")) return false;
  if (rawPath.includes("\\") || rawPath.includes("#")) return false;
  const [pathname] = rawPath.split("?", 1);
  if (!pathname || pathname.split("/").some((seg) => seg === ".." || seg === ".")) return false;
  return apiPaths.some((p) => p.method === method && p.pattern.test(pathname));
}

/** A #rrggbb colour, the only form the title bar accepts. */
export function isHexColor(v: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(v);
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

/**
 * A commit message from its parts: a title of one line, 1 to 200 characters,
 * and an optional description up to 5,000, joined by a blank line as git
 * expects. No NUL anywhere. Null when either part is out of bounds.
 */
export function commitMessage(title: unknown, body: unknown): string | null {
  if (typeof title !== "string" || typeof body !== "string") return null;
  const t = title.trim();
  const b = body.replace(/\r\n/g, "\n").trim();
  if (!t || t.length > 200 || /[\r\n\0]/.test(t)) return null;
  if (b.length > 5000 || b.includes("\0")) return null;
  return b ? `${t}\n\n${b}` : t;
}

/**
 * A pull request's title and body as the person wrote them. "Closes #n" is
 * added only when they ask for it. Null when out of GitHub's bounds.
 */
export function pullRequestText(
  input: unknown,
  issueNumber: number,
): { title: string; body: string } | null {
  const i = (input ?? {}) as { title?: unknown; body?: unknown; closesIssue?: unknown };
  if (typeof i.title !== "string" || typeof i.body !== "string" || typeof i.closesIssue !== "boolean") return null;
  const title = i.title.trim();
  let body = i.body.replace(/\r\n/g, "\n").trim();
  if (!title || title.length > 256 || /[\r\n\0]/.test(title)) return null;
  if (body.length > 20000 || body.includes("\0")) return null;
  if (i.closesIssue) body = body ? `${body}\n\nCloses #${issueNumber}` : `Closes #${issueNumber}`;
  return { title, body };
}

/**
 * VS Code's link for a folder: vscode://file/<path>, each segment encoded,
 * except a Windows drive's colon, which VS Code must see as it is ("C%3A"
 * is a folder that doesn't exist).
 */
export function vscodeFolderLink(dir: string, platform: NodeJS.Platform = process.platform): string {
  const p = platform === "win32" ? path.win32 : path.posix;
  const segments = p.resolve(dir).split(p.sep).filter((seg, i) => seg !== "" || i === 0);
  const encoded = segments.map((seg, i) => (i === 0 && /^[A-Za-z]:$/.test(seg) ? seg : encodeURIComponent(seg))).join("/");
  return `vscode://file/${encoded.replace(/^\/+/, "")}`;
}

/** Text the page may put on the clipboard: short, one line, printable (a sign-in code). */
export function isClipboardText(v: unknown): v is string {
  return typeof v === "string" && v.length > 0 && v.length <= 100 && !/[\0-\x1f\x7f]/.test(v);
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

/** A release version as the app's tags have them: x.y.z, no prefix. */
export function isVersion(v: unknown): v is string {
  return typeof v === "string" && /^\d{1,4}\.\d{1,4}\.\d{1,6}$/.test(v);
}

/** Whether version a is newer than b (both x.y.z). */
export function isNewerVersion(a: string, b: string): boolean {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if (pa[i]! !== pb[i]!) return pa[i]! > pb[i]!;
  }
  return false;
}
