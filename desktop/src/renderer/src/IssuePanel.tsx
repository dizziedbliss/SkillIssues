import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import type { AuthState, Workspace } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { DeviceFlow } from "./DeviceFlow";

/*
 * One issue and the way from reading it to a pull request: start work (fork,
 * clone, branch), open it in an editor or the file manager, commit and push,
 * open the pull request.
 */
export function IssuePanel({ issue, state, onAuth }: { issue: Issue; state: AuthState; onAuth: (s: AuthState) => void }) {
  const queryClient = useQueryClient();
  const key = ["workspace", issue.id];
  const ws = useQuery({ queryKey: key, queryFn: () => call(desktop.workspace(issue.id)) });
  const [message, setMessage] = useState("");
  const setWs = (w: Workspace) => queryClient.setQueryData(key, w);

  const start = useMutation({
    mutationFn: () => call(desktop.startWork({ id: issue.id, number: issue.number, repository: issue.repository, title: issue.title })),
    onSuccess: setWs,
  });
  const push = useMutation({
    mutationFn: () => call(desktop.commitAndPush(issue.id, message || `Fix #${issue.number}: ${issue.title}`)),
    onSuccess: (w) => (setWs(w), setMessage("")),
  });
  const pr = useMutation({ mutationFn: () => call(desktop.openPullRequest(issue.id)), onSuccess: setWs });
  const open = useMutation({
    mutationFn: (where: "editor" | "folder") => call(where === "editor" ? desktop.openEditor(issue.id) : desktop.openFolder(issue.id)),
  });

  const failure = start.error ?? push.error ?? pr.error ?? open.error ?? ws.error;
  const w = ws.data;

  return (
    <section className="panel" aria-labelledby="issue-title">
      <p className="repo">{issue.repository} #{issue.number}</p>
      <h2 id="issue-title" className="serif">{issue.title}</h2>
      <p className="meta">
        {[issue.language, issue.difficulty === "UNCLASSIFIED" ? "unrated" : issue.difficulty.toLowerCase(), `${issue.comments} comments`, issue.author && `opened by ${issue.author}`]
          .filter(Boolean)
          .join(" · ")}
      </p>
      <button type="button" className="link" onClick={() => void desktop.openExternal(issue.url)}>Read it on GitHub</button>

      <div className="steps">
        {!state.pushConnected ? (
          <div className="step">
            <h3>Connect GitHub for pushing</h3>
            <p className="quiet">
              To fork the project and push your fix, the app needs a GitHub token that can write to your public
              repositories. It stays on this computer and is never sent to SkillIssues.
            </p>
            <DeviceFlow label="Connect GitHub" start={desktop.pushStart} wait={desktop.pushWait} onDone={onAuth} />
          </div>
        ) : !w ? (
          <div className="step">
            <h3>Start work</h3>
            <p className="quiet">Forks {issue.repository} to your account, clones your fork and switches to a branch for this issue.</p>
            <button type="button" className="button" onClick={() => start.mutate()} disabled={start.isPending || ws.isPending}>
              {start.isPending ? "Forking and cloning…" : "Start work"}
            </button>
          </div>
        ) : (
          <>
            <div className="step">
              <h3>Your workspace</h3>
              <p className="mono small">{w.path}</p>
              <p className="quiet small">Branch {w.branch} on your fork {w.fork}.</p>
              <div className="row">
                <button type="button" className="button" onClick={() => open.mutate("editor")}>Open in VS Code</button>
                <button type="button" className="link" onClick={() => open.mutate("folder")}>Show the folder</button>
                <button type="button" className="link" onClick={() => void ws.refetch()}>Refresh</button>
              </div>
            </div>
            <div className="step">
              <h3>Commit and push</h3>
              <p className="quiet small">
                {w.changes.length === 0 ? "No uncommitted changes." : `${w.changes.length} changed file${w.changes.length === 1 ? "" : "s"}.`}
              </p>
              <input
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={`Fix #${issue.number}: ${issue.title}`}
                aria-label="Commit message"
                maxLength={500}
              />
              <button type="button" className="button" onClick={() => push.mutate()} disabled={push.isPending}>
                {push.isPending ? "Pushing…" : w.changes.length ? "Commit and push" : "Push"}
              </button>
            </div>
            <div className="step">
              <h3>Pull request</h3>
              {w.pullRequestUrl ? (
                <button type="button" className="link" onClick={() => void desktop.openExternal(w.pullRequestUrl!)}>{w.pullRequestUrl}</button>
              ) : (
                <button type="button" className="button" onClick={() => pr.mutate()} disabled={pr.isPending}>
                  {pr.isPending ? "Opening…" : "Open a pull request"}
                </button>
              )}
            </div>
          </>
        )}
      </div>
      {failure && <p className="error" role="alert">{failure.message}</p>}
    </section>
  );
}
