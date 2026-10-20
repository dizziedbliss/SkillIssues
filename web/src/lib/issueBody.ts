import { ApiError, type Issue } from "@skillissues/client";
import { api } from "./api";

/*
 * An issue's body. The browser asks GitHub itself first (public, no token,
 * counted against the visitor's own rate limit); only when GitHub refuses
 * for that limit does it ask our server, whose cache is capped per caller
 * (GET /issues/{id}/body). Bodies are attacker-controlled Markdown: render
 * them only through @skillissues/markdown.
 */

/** What GitHub says of the issue now, when it answered: newer than the catalog's copy. */
export interface Live {
  closed: boolean;
  closedAt: string | null;
}

export type BodyResult = { body: string; from: "github"; live: Live } | { body: string; from: "cache" } | { queued: string };

export async function loadIssueBody(issue: Issue, signal?: AbortSignal): Promise<BodyResult> {
  const [owner, repo] = issue.repository.split("/");
  const res = await fetch(
    `https://api.github.com/repos/${encodeURIComponent(owner ?? "")}/${encodeURIComponent(repo ?? "")}/issues/${issue.number}`,
    { headers: { Accept: "application/vnd.github+json" }, signal, credentials: "omit" },
  );
  if (res.ok) {
    const data = (await res.json()) as { body?: string | null; state?: string; closed_at?: string | null };
    return { body: data.body ?? "", from: "github", live: { closed: data.state === "closed", closedAt: data.closed_at ?? null } };
  }
  // 403 or 429 with no requests left is GitHub's rate limit: fall back to
  // our cache. Anything else (gone, moved, GitHub down) is the answer.
  const limited = (res.status === 403 || res.status === 429) && res.headers.get("x-ratelimit-remaining") === "0";
  if (!limited) throw new ApiError(res.status, `GitHub answered ${res.status} for this issue`);
  const cached = await api.issueBody(issue.id, { signal });
  return "body" in cached ? { body: cached.body, from: "cache" } : { queued: cached.reason };
}
