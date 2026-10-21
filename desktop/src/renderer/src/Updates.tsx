import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { renderMarkdown } from "@skillissues/markdown";
import { useT } from "@web/i18n";
import type { UpdateState } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { useAppRoot } from "./appRoot";
import s from "./desktop.module.css";

/*
 * The app's updates, asked rather than done: the title bar says an update is
 * available, and this dialog shows what's in it and lets the person download
 * it, then restart into it (or let closing the app install it). After an
 * update, What's new shows the release notes once.
 */

export const updateKey = ["updateState"];

/** A dialog in the app's root, so the theme reaches it; closing it calls onClose. */
function Dialog({ label, onClose, children }: { label: string; onClose: () => void; children: React.ReactNode }) {
  const dialog = useRef<HTMLDialogElement>(null);
  // Only inside the app's root: rendered before it exists (What's new, at
  // start), it would open in <body> and then be recreated, closed, in the root.
  const host = useAppRoot();
  useEffect(() => {
    if (dialog.current && !dialog.current.open) dialog.current.showModal();
  }, [host]);
  if (!host) return null;
  return createPortal(
    <dialog ref={dialog} className={s.dialog} aria-label={label} onClose={onClose} data-ui="update-dialog">
      <div className={s.panel}>{children}</div>
    </dialog>,
    host,
  );
}

/** One version's release notes, rendered like an issue body (sanitised). */
function Notes({ version }: { version: string }) {
  const t = useT();
  const notes = useQuery({ queryKey: ["releaseNotes", version], queryFn: () => call(desktop.releaseNotes(version)), staleTime: Infinity });
  const html = useMemo(() => (notes.data?.body ? renderMarkdown(notes.data.body) : ""), [notes.data]);
  if (notes.isPending) return <p className={s.muted}>{t("update.notesLoading")}</p>;
  if (notes.error)
    return (
      <p className={s.error} role="alert">
        {notes.error.message}{" "}
        <button type="button" className={s.link} onClick={() => void notes.refetch()}>{t("update.retry")}</button>
      </p>
    );
  if (!html) return <p className={s.muted}>{t("update.noNotes")}</p>;
  return (
    <>
      <div className={s.notes} dangerouslySetInnerHTML={{ __html: html }} />
      {notes.data && (
        <button type="button" className={s.link} onClick={() => void desktop.openExternal(notes.data!.url)}>{t("update.onGitHub")}</button>
      )}
    </>
  );
}

/** The update, from offered to ready. */
export function UpdateDialog({ onClose }: { onClose: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const state = useQuery({ queryKey: updateKey, queryFn: () => call(desktop.updateState()), refetchInterval: 1000 });
  const setState = (u: UpdateState) => queryClient.setQueryData(updateKey, u);
  const download = useMutation({ mutationFn: () => call(desktop.downloadUpdate()), onSuccess: setState });
  const restart = useMutation({ mutationFn: () => call(desktop.installUpdate()) });
  const retry = useMutation({ mutationFn: () => call(desktop.checkUpdates()), onSuccess: setState });
  const u = state.data;
  const failure = download.error ?? restart.error ?? retry.error ?? state.error;
  const close = () => onClose();

  return (
    <Dialog label={t("update.title", { version: u?.version ?? "" })} onClose={close}>
      {u?.status === "available" && (
        <>
          <h2 className={s.title}>{t("update.title", { version: u.version ?? "" })}</h2>
          <p className={s.muted}>{u.manual ? t("update.bodyManual") : t("update.body")}</p>
          <section className={s.step}>
            <h3 className={s.stepTitle}>{t("update.whatsIn")}</h3>
            <Notes version={u.version!} />
          </section>
          <div className={s.row}>
            <button type="button" className={s.button} onClick={() => download.mutate()} disabled={download.isPending}>
              {u.manual ? t("update.downloadManual") : t("update.download")}
            </button>
            <button type="button" className={s.link} onClick={close}>{t("update.later")}</button>
          </div>
        </>
      )}
      {u?.status === "downloading" && (
        <>
          <h2 className={s.title}>{t("update.downloading", { version: u.version ?? "" })}</h2>
          <div className={s.meter} role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={u.percent ?? 0}>
            <span style={{ width: `${u.percent ?? 0}%` }} />
          </div>
          <p className={s.muted}>{t("update.downloadingBody")}</p>
          <button type="button" className={s.link} onClick={close}>{t("update.hide")}</button>
        </>
      )}
      {u?.status === "ready" && (
        <>
          <h2 className={s.title}>{t("update.ready", { version: u.version ?? "" })}</h2>
          <p className={s.muted}>{t("update.readyBody")}</p>
          <div className={s.row}>
            <button type="button" className={s.button} onClick={() => restart.mutate()} disabled={restart.isPending}>
              {t("update.restart")}
            </button>
            <button type="button" className={s.link} onClick={close}>{t("update.onClose")}</button>
          </div>
        </>
      )}
      {u?.status === "error" && (
        <>
          <h2 className={s.title}>{t("update.failed")}</h2>
          <p className={s.error} role="alert">{u.reason}</p>
          <div className={s.row}>
            <button type="button" className={s.button} onClick={() => retry.mutate()} disabled={retry.isPending}>{t("update.retry")}</button>
            <button type="button" className={s.link} onClick={close}>{t("work.close")}</button>
          </div>
        </>
      )}
      {u && !["available", "downloading", "ready", "error"].includes(u.status) && (
        <>
          <h2 className={s.title}>{t("update.upToDate")}</h2>
          <button type="button" className={s.link} onClick={close}>{t("work.close")}</button>
        </>
      )}
      {failure && <p className={s.error} role="alert">{failure.message}</p>}
    </Dialog>
  );
}

/** What's new: the first start after an update shows that version's notes, once. */
export function WhatsNew() {
  const t = useT();
  const version = useQuery({ queryKey: ["whatsNew"], queryFn: () => call(desktop.whatsNew()), staleTime: Infinity, retry: false });
  const [closed, setClosed] = useState(false);
  const v = version.data;
  if (!v || closed) return null;
  return (
    <Dialog label={t("whatsNew.title", { version: v })} onClose={() => setClosed(true)}>
      <h2 className={s.title}>{t("whatsNew.title", { version: v })}</h2>
      <p className={s.muted}>{t("whatsNew.body")}</p>
      <section className={s.step}>
        <Notes version={v} />
      </section>
      <CloseButton />
    </Dialog>
  );
}

/** Closes the dialog it's in. */
function CloseButton() {
  const t = useT();
  const ref = useRef<HTMLButtonElement>(null);
  return (
    <button ref={ref} type="button" className={s.button} onClick={() => ref.current?.closest("dialog")?.close()}>
      {t("whatsNew.close")}
    </button>
  );
}
