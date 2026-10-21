import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { useT } from "@web/i18n";
import type { AuthState, Workspace } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { DeviceFlow } from "./DeviceFlow";
import { useAppRoot } from "./appRoot";
import s from "./desktop.module.css";

/**
 * "Work on it here", on an issue page: a dialog with the way from reading an
 * issue to a pull request. Start work (fork, clone, branch), open it in an
 * editor or the file manager, commit and push, open the pull request.
 */
export function WorkOnIt({ issue, className }: { issue: Issue; className?: string }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)}>{t("work.open")}</button>
      {open && <WorkDialog issue={issue} onClose={() => setOpen(false)} />}
    </>
  );
}

function WorkDialog({ issue, onClose }: { issue: Issue; onClose: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (dialog.current && !dialog.current.open) dialog.current.showModal();
  }, []);

  const auth = useQuery({ queryKey: ["desktopAuth"], queryFn: () => call(desktop.authState()) });
  const key = ["workspace", issue.id];
  const ws = useQuery({ queryKey: key, queryFn: () => call(desktop.workspace(issue.id)) });
  const [message, setMessage] = useState("");
  const setWs = (w: Workspace) => queryClient.setQueryData(key, w);
  const setAuth = (a: AuthState) => queryClient.setQueryData(["desktopAuth"], a);

  const start = useMutation({
    mutationFn: () => call(desktop.startWork({ id: issue.id, number: issue.number, repository: issue.repository, title: issue.title })),
    onSuccess: setWs,
  });
  const push = useMutation({
    mutationFn: () => call(desktop.commitAndPush(issue.id, message || `Fix #${issue.number}: ${issue.title}`)),
    onSuccess: (w) => (setWs(w), setMessage("")),
  });
  const pr = useMutation({ mutationFn: () => call(desktop.openPullRequest(issue.id)), onSuccess: setWs });
  const reveal = useMutation({
    mutationFn: (where: "editor" | "folder") => call(where === "editor" ? desktop.openEditor(issue.id) : desktop.openFolder(issue.id)),
  });

  const failure = start.error ?? push.error ?? pr.error ?? reveal.error ?? ws.error ?? auth.error;
  const w = ws.data;
  const host = useAppRoot() ?? document.body;

  return createPortal(
    <dialog ref={dialog} className={s.dialog} aria-labelledby="work-title" onClose={onClose} data-ui="work-on-it">
      <div className={s.panel}>
        <p className={s.muted}>{issue.repository} #{issue.number}</p>
        <h2 id="work-title" className={s.title}>{issue.title}</h2>

        {auth.data && !auth.data.pushConnected ? (
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
          <>
            <section className={s.step}>
              <h3 className={s.stepTitle}>{t("work.ws.title")}</h3>
              <p className={s.mono}>{w.path}</p>
              <p className={s.muted}>{t("work.ws.branch", { branch: w.branch, fork: w.fork })}</p>
              <div className={s.row}>
                <button type="button" className={s.button} onClick={() => reveal.mutate("editor")}>{t("work.ws.editor")}</button>
                <button type="button" className={s.link} onClick={() => reveal.mutate("folder")}>{t("work.ws.folder")}</button>
                <button type="button" className={s.link} onClick={() => void ws.refetch()}>{t("work.ws.refresh")}</button>
              </div>
            </section>
            <section className={s.step}>
              <h3 className={s.stepTitle}>{t("work.push.title")}</h3>
              <p className={s.muted}>{w.changes.length === 0 ? t("work.push.clean") : t("work.push.changes", { count: w.changes.length })}</p>
              <input className={s.input} value={message} onChange={(e) => setMessage(e.target.value)}
                placeholder={`Fix #${issue.number}: ${issue.title}`} aria-label={t("work.push.message")} maxLength={500} />
              <button type="button" className={s.button} onClick={() => push.mutate()} disabled={push.isPending}>
                {push.isPending ? t("work.push.busy") : w.changes.length ? t("work.push.commit") : t("work.push.push")}
              </button>
            </section>
            <section className={s.step}>
              <h3 className={s.stepTitle}>{t("work.pr.title")}</h3>
              {w.pullRequestUrl ? (
                <button type="button" className={s.link} onClick={() => void desktop.openExternal(w.pullRequestUrl!)}>{w.pullRequestUrl}</button>
              ) : (
                <button type="button" className={s.button} onClick={() => pr.mutate()} disabled={pr.isPending}>
                  {pr.isPending ? t("work.pr.busy") : t("work.pr.open")}
                </button>
              )}
              <p className={s.muted}>{t("work.pr.hint")}</p>
            </section>
          </>
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
