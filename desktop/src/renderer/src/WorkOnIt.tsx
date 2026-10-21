import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { useT } from "@web/i18n";
import { useConfirm } from "@web/app/Confirm";
import type { AuthState, Workspace } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { DeviceFlow } from "./DeviceFlow";
import { requestSignIn } from "./signIn";
import { useAppRoot } from "./appRoot";
import s from "./desktop.module.css";

/** What the work flow needs to know about an issue. */
export type WorkIssue = Pick<Issue, "id" | "number" | "repository" | "title" | "url">;

/** The workspace queries: one per issue, and the list on Working on. */
export const workspaceKey = (issueId: number) => ["workspace", issueId];
export const workspacesKey = ["workspaces"];

/**
 * "Work on it here", on an issue page: a dialog with the way from reading an
 * issue to a pull request. Start work (fork, clone, branch), then commit,
 * push and open the pull request, each when there is something to do.
 */
export function WorkOnIt({ issue, className }: { issue: WorkIssue; className?: string }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)}>{t("work.open")}</button>
      {open && <WorkDialog issue={issue} onClose={() => setOpen(false)} />}
    </>
  );
}

export function WorkDialog({ issue, onClose }: { issue: WorkIssue; onClose: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (dialog.current && !dialog.current.open) dialog.current.showModal();
  }, []);

  const auth = useQuery({ queryKey: ["desktopAuth"], queryFn: () => call(desktop.authState()) });
  const ws = useQuery({ queryKey: workspaceKey(issue.id), queryFn: () => call(desktop.workspace(issue.id)) });
  const setAuth = (a: AuthState) => queryClient.setQueryData(["desktopAuth"], a);
  const start = useMutation({
    mutationFn: () => call(desktop.startWork({ id: issue.id, number: issue.number, repository: issue.repository, title: issue.title })),
    onSuccess: (w) => {
      queryClient.setQueryData(workspaceKey(issue.id), w);
      return queryClient.invalidateQueries({ queryKey: workspacesKey });
    },
  });
  const signIn = useMutation({
    mutationFn: () => requestSignIn(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["desktopAuth"] }),
  });

  const failure = start.error ?? signIn.error ?? ws.error ?? auth.error;
  const w = ws.data;
  const host = useAppRoot() ?? document.body;

  return createPortal(
    <dialog ref={dialog} className={s.dialog} aria-labelledby="work-title" onClose={onClose} data-ui="work-on-it">
      <div className={s.panel}>
        <p className={s.muted}>{issue.repository} #{issue.number}</p>
        <h2 id="work-title" className={s.title}>{issue.title}</h2>

        {auth.data && !auth.data.signedIn ? (
          <section className={s.step}>
            <h3 className={s.stepTitle}>{t("work.signin.title")}</h3>
            <p className={s.muted}>{t("work.signin.body")}</p>
            <button type="button" className={s.button} onClick={() => signIn.mutate()} disabled={signIn.isPending}>
              {t("work.signin.button")}
            </button>
          </section>
        ) : auth.data && !auth.data.pushConnected ? (
          <section className={s.step}>
            <h3 className={s.stepTitle}>{t("work.connect.title")}</h3>
            <p className={s.muted}>{t("work.connect.body")}</p>
            <DeviceFlow label={t("work.connect.button")} start={desktop.pushStart} wait={desktop.pushWait} onDone={setAuth} />
          </section>
        ) : !w ? (
          <section className={s.step}>
            <h3 className={s.stepTitle}>{t("work.start.title")}</h3>
            <p className={s.muted}>{t("work.start.body", { repo: issue.repository })}</p>
            <button type="button" className={s.button} onClick={() => start.mutate()} disabled={start.isPending || ws.isPending || !auth.data}>
              {start.isPending ? t("work.start.busy") : t("work.start.button")}
            </button>
          </section>
        ) : (
          <WorkspaceSteps ws={w} issue={issue} />
        )}
        {failure && <p className={s.error} role="alert">{failure.message}</p>}
        <div className={s.row}>
          <button type="button" className={s.link} onClick={() => void desktop.openExternal(issue.url)}>{t("work.read")}</button>
          <button type="button" className={s.link} onClick={() => dialog.current?.close()}>{t("work.close")}</button>
        </div>
      </div>
    </dialog>,
    host,
  );
}

/** A started workspace: where it is, then commit, push and pull request in order. */
export function WorkspaceSteps({ ws: w, issue }: { ws: Workspace; issue: WorkIssue }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [confirmDialog, confirm] = useConfirm();
  const [commitTitle, setCommitTitle] = useState("");
  const [commitBody, setCommitBody] = useState("");
  const [prTitle, setPrTitle] = useState(issue.title);
  const [prBody, setPrBody] = useState("");
  const [closes, setCloses] = useState(false);
  // A closed pull request can be followed by a new one.
  const [another, setAnother] = useState(false);

  const update = (next: Workspace) => {
    queryClient.setQueryData(workspaceKey(issue.id), next);
    return queryClient.invalidateQueries({ queryKey: workspacesKey });
  };
  const refresh = useMutation({ mutationFn: () => call(desktop.workspace(issue.id)), onSuccess: (n) => n && update(n) });
  const refork = useMutation({ mutationFn: () => call(desktop.refork(issue.id)), onSuccess: update });
  const commit = useMutation({
    mutationFn: () => call(desktop.commit(issue.id, { title: commitTitle, body: commitBody })),
    onSuccess: (n) => {
      setCommitTitle("");
      setCommitBody("");
      return update(n);
    },
  });
  const push = useMutation({ mutationFn: () => call(desktop.push(issue.id)), onSuccess: update });
  const pr = useMutation({
    mutationFn: () => call(desktop.openPullRequest(issue.id, { title: prTitle, body: prBody, closesIssue: closes })),
    onSuccess: update,
  });
  const reveal = useMutation({
    mutationFn: (where: "editor" | "folder") => call(where === "editor" ? desktop.openEditor(issue.id) : desktop.openFolder(issue.id)),
  });
  const forget = useMutation({
    mutationFn: () => call(desktop.forgetWorkspace(issue.id)),
    onSuccess: () => {
      queryClient.setQueryData(workspaceKey(issue.id), null);
      return queryClient.invalidateQueries({ queryKey: workspacesKey });
    },
  });

  const failure = refork.error ?? refresh.error ?? commit.error ?? push.error ?? pr.error ?? reveal.error ?? forget.error;
  const dirty = w.changes.length > 0;
  const canPush = !dirty && w.unpushed > 0 && !w.forkMissing;
  const canOpenPr = !dirty && w.commits > 0 && w.unpushed === 0 && !w.forkMissing;
  // An open or merged pull request is the end of the road; a closed one isn't.
  const prDone = !!w.pullRequestUrl && w.pullRequestState !== "closed";
  const pushed = w.commits > 0 && w.unpushed === 0;
  const forkURL = `https://github.com/${w.fork}${pushed ? `/tree/${w.branch.split("/").map(encodeURIComponent).join("/")}` : ""}`;

  return (
    <>
      {confirmDialog}
      <section className={s.step}>
        <h3 className={s.stepTitle}>{t("work.ws.title")}</h3>
        <p className={s.mono}>{w.path}</p>
        <p className={s.muted}>{t("work.ws.branch", { branch: w.branch, fork: w.fork })}</p>
        {w.problem && <p className={s.error} role="alert">{w.problem}</p>}
        <div className={s.row}>
          <button type="button" className={s.button} onClick={() => reveal.mutate("editor")} disabled={!!w.problem}>{t("work.ws.editor")}</button>
          <button type="button" className={s.link} onClick={() => reveal.mutate("folder")} disabled={!!w.problem}>{t("work.ws.folder")}</button>
          <button type="button" className={s.link} onClick={() => void desktop.openExternal(forkURL)}>{t("work.ws.fork")}</button>
          <button type="button" className={s.link} onClick={() => refresh.mutate()} disabled={refresh.isPending}>
            {refresh.isPending ? t("work.ws.refreshing") : t("work.ws.refresh")}
          </button>
          <button
            type="button"
            className={s.link}
            disabled={forget.isPending}
            onClick={async () => {
              const yes = await confirm({ title: t("work.ws.forget.title"), body: t("work.ws.forget.body", { path: w.path }), confirm: t("work.ws.forget.confirm") });
              if (yes) forget.mutate();
            }}
          >
            {t("work.ws.forget")}
          </button>
        </div>
      </section>

      {w.forkMissing && (
        <section className={s.step} data-ui="work-fork" data-ui-state="missing">
          <h3 className={s.stepTitle}>{t("work.fork.title")}</h3>
          <p className={s.error} role="alert">{t("work.fork.body", { fork: w.fork })}</p>
          <button type="button" className={s.button} onClick={() => refork.mutate()} disabled={refork.isPending}>
            {refork.isPending ? t("work.fork.busy") : t("work.fork.again")}
          </button>
        </section>
      )}

      <section className={s.step} data-ui="work-commit" data-ui-state={dirty ? "ready" : "idle"}>
        <h3 className={s.stepTitle}>{t("work.commit.title")}</h3>
        {dirty ? (
          <>
            <p className={s.muted}>{t("work.commit.changes", { count: w.changes.length })}</p>
            <ul className={s.files}>
              {w.changes.slice(0, 8).map((line) => <li key={line} className={s.mono}>{line}</li>)}
              {w.changes.length > 8 && <li className={s.muted}>{t("work.commit.more", { count: w.changes.length - 8 })}</li>}
            </ul>
            <label className={s.field}>
              <span>{t("work.commit.summary")}</span>
              <input className={s.input} value={commitTitle} onChange={(e) => setCommitTitle(e.target.value)}
                placeholder={t("work.commit.summaryHint")} maxLength={200} />
            </label>
            <label className={s.field}>
              <span>{t("work.commit.body")}</span>
              <textarea className={s.input} value={commitBody} onChange={(e) => setCommitBody(e.target.value)} rows={3} maxLength={5000} />
            </label>
            <button type="button" className={s.button} onClick={() => commit.mutate()} disabled={commit.isPending || !commitTitle.trim()}>
              {commit.isPending ? t("work.commit.busy") : t("work.commit.button", { count: w.changes.length })}
            </button>
          </>
        ) : (
          <p className={s.muted}>{w.commits > 0 ? t("work.commit.clean") : t("work.commit.none")}</p>
        )}
      </section>

      <section className={s.step} data-ui="work-push" data-ui-state={canPush ? "ready" : "idle"}>
        <h3 className={s.stepTitle}>{t("work.push.title")}</h3>
        <p className={s.muted}>
          {w.forkMissing
            ? t("work.push.noFork")
            : dirty
            ? t("work.push.dirty")
            : w.commits === 0
              ? t("work.push.nothing")
              : w.unpushed > 0
                ? t("work.push.ahead", { count: w.unpushed })
                : t("work.push.done")}
        </p>
        {canPush && (
          <button type="button" className={s.button} onClick={() => push.mutate()} disabled={push.isPending}>
            {push.isPending ? t("work.push.busy") : t("work.push.button")}
          </button>
        )}
      </section>

      <section className={s.step} data-ui="work-pr" data-ui-state={w.pullRequestUrl ? (w.pullRequestState ?? "open") : canOpenPr ? "ready" : "idle"}>
        <h3 className={s.stepTitle}>{t("work.pr.title")}</h3>
        {w.pullRequestUrl && (prDone || !another) ? (
          <>
            <p className={w.pullRequestState === "closed" ? s.error : s.muted}>
              {w.pullRequestState === "merged" ? t("work.pr.merged") : w.pullRequestState === "closed" ? t("work.pr.closed") : t("work.pr.opened")}
            </p>
            <div className={s.row}>
              <button type="button" className={s.button} onClick={() => void desktop.openExternal(w.pullRequestUrl!)}>{t("work.pr.goto")}</button>
              {w.pullRequestState === "closed" && (
                <button type="button" className={s.link} onClick={() => setAnother(true)} disabled={!canOpenPr}>{t("work.pr.another")}</button>
              )}
            </div>
            {w.pullRequestState === "closed" && !canOpenPr && <p className={s.muted}>{t("work.pr.needsPush")}</p>}
          </>
        ) : canOpenPr ? (
          <>
            <label className={s.field}>
              <span>{t("work.pr.summary")}</span>
              <input className={s.input} value={prTitle} onChange={(e) => setPrTitle(e.target.value)} maxLength={256} />
            </label>
            <label className={s.field}>
              <span>{t("work.pr.body")}</span>
              <textarea className={s.input} value={prBody} onChange={(e) => setPrBody(e.target.value)} rows={4}
                placeholder={t("work.pr.bodyHint")} maxLength={20000} />
            </label>
            <label className={s.check}>
              <input type="checkbox" checked={closes} onChange={(e) => setCloses(e.target.checked)} />
              <span>
                {t("work.pr.closes", { number: issue.number })}
                <span className={s.muted}> {t("work.pr.closesHelp")}</span>
              </span>
            </label>
            <button type="button" className={s.button} onClick={() => pr.mutate()} disabled={pr.isPending || !prTitle.trim()}>
              {pr.isPending ? t("work.pr.busy") : t("work.pr.open")}
            </button>
          </>
        ) : (
          <p className={s.muted}>{t("work.pr.needsPush")}</p>
        )}
      </section>
      {failure && <p className={s.error} role="alert">{failure.message}</p>}
    </>
  );
}

/** What's next for a workspace, in a few words: Working on's status line. */
export function nextStep(w: Workspace): "missing" | "fork" | "edit" | "commit" | "push" | "pr" | "open" | "merged" | "closed" {
  if (w.problem) return "missing";
  if (w.pullRequestUrl && w.pullRequestState === "merged") return "merged";
  if (w.forkMissing) return "fork";
  if (w.pullRequestUrl && w.pullRequestState === "closed") return "closed";
  if (w.pullRequestUrl) return "open";
  if (w.changes.length > 0) return "commit";
  if (w.unpushed > 0) return "push";
  if (w.commits > 0) return "pr";
  return "edit";
}
