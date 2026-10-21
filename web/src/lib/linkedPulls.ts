import { ApiError, type Issue } from "@skillissues/client";

/*
 * Whether a pull request already points at an issue, read in the browser from
 * the issue's public timeline on GitHub: a pull request that mentions or
 * closes it shows there as a cross-reference. One request, like the
 * description's; GitHub's limit for a browser applies, and a refusal comes
 * back as an error rather than as "none".
 */

export interface LinkedPulls {
  /** Open pull requests that reference the issue. */
  open: number;
  /** Merged ones. */
  merged: number;
}

interface TimelineEvent {
  event?: string;
  source?: { issue?: { pull_request?: { merged_at?: string | null }; state?: string } };
}

export async function loadLinkedPulls(issue: Issue, signal?: AbortSignal): Promise<LinkedPulls> {
  const [owner, repo] = issue.repository.split("/");
  const res = await fetch(
    `https://api.github.com/repos/${encodeURIComponent(owner ?? "")}/${encodeURIComponent(repo ?? "")}/issues/${issue.number}/timeline?per_page=100`,
    { headers: { Accept: "application/vnd.github+json" }, signal, credentials: "omit" },
  );
  if (!res.ok) throw new ApiError(res.status, `GitHub answered ${res.status} for this issue's timeline`);
  const events = (await res.json()) as TimelineEvent[];
  let open = 0;
  let merged = 0;
  for (const e of events) {
    const pr = e.event === "cross-referenced" ? e.source?.issue : undefined;
    if (!pr?.pull_request) continue;
    if (pr.pull_request.merged_at) merged++;
    else if (pr.state === "open") open++;
  }
  return { open, merged };
}
