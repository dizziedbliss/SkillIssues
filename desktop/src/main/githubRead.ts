import type { GitHubReply } from "../shared/contract";
import { isAllowedGitHubRead } from "./security";

/*
 * Public GitHub API reads for the page, which reaches no network itself: an
 * issue's description, its timeline, and pull request search, the same reads
 * the web app makes from the browser. No credential is ever sent, so GitHub
 * counts them against this computer's address, as it would a browser.
 */

/** Largest answer handed to the page. */
const MAX_BYTES = 5 * 1024 * 1024;

/** The headers the page reads (rate limits); nothing else crosses. */
const KEEP = ["x-ratelimit-remaining", "x-ratelimit-reset", "retry-after"];

export async function githubGet(url: string): Promise<GitHubReply> {
  if (!isAllowedGitHubRead(url)) throw new Error("That GitHub request isn't allowed from the app.");
  let res: Response;
  try {
    res = await fetch(url, { headers: { Accept: "application/vnd.github+json", "User-Agent": "SkillIssues-Desktop" }, redirect: "error" });
  } catch {
    throw new Error("Can't reach GitHub. Check your connection and try again.");
  }
  const body = await res.text();
  if (body.length > MAX_BYTES) throw new Error("GitHub's answer was too large to show.");
  const headers: Record<string, string> = {};
  for (const h of KEEP) {
    const v = res.headers.get(h);
    if (v !== null) headers[h] = v;
  }
  return { status: res.status, body, headers };
}
