import { useEffect, useRef, useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { ApiError, REPORT_REASONS, type ReportTarget } from "@skillissues/client";
import { api } from "../lib/api";
import { useT, type MessageKey } from "../i18n";
import { useSession, useSignIn } from "./session";
import s from "./ReportButton.module.css";

/**
 * Report an issue, a repository or a public theme: a dialog with the reasons
 * that apply and an optional note. Signed out, it asks you to sign in first.
 * Reports are private: they go to the people who run SkillIssues, and enough
 * of them take the target out of lists until someone looks.
 */
export function ReportButton({ target, label, className }: { target: ReportTarget; label: MessageKey; className?: string }) {
  const t = useT();
  const { user } = useSession();
  const signIn = useSignIn();
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [note, setNote] = useState("");

  useEffect(() => {
    const d = dialog.current;
    if (open && d && !d.open) d.showModal();
  }, [open]);

  const send = useMutation({ mutationFn: () => api.report(target, reason, note) });
  const problem =
    send.error instanceof ApiError
      ? send.error.status === 409
        ? t("report.already")
        : send.error.status === 429
          ? t("report.tooMany")
          : send.error.status === 404
            ? t("report.gone")
            : send.error.message
      : send.error?.message;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (reason) send.mutate();
  }

  function close() {
    dialog.current?.close();
  }

  return (
    <>
      <button type="button" className={className ?? s.trigger} onClick={() => (user ? setOpen(true) : signIn.mutate())} data-ui="report">
        {t(label)}
      </button>
      {open && (
        <dialog
          ref={dialog}
          className={s.dialog}
          aria-labelledby="report-title"
          onClose={() => {
            setOpen(false);
            send.reset();
            setReason("");
            setNote("");
          }}
          onClick={(e) => {
            if (e.target === dialog.current) close();
          }}
        >
          <div className={s.panel}>
            <h2 id="report-title" className={s.title}>{t(label)}</h2>
            {send.isSuccess ? (
              <>
                <p className={s.body} role="status">{t("report.thanks")}</p>
                <button type="button" className={s.primary} onClick={close}>{t("report.done")}</button>
              </>
            ) : (
              <form onSubmit={submit} className={s.form}>
                <p className={s.body}>{t("report.private")}</p>
                <fieldset className={s.reasons}>
                  <legend className="visually-hidden">{t("report.why")}</legend>
                  {REPORT_REASONS[target.type].map((r) => (
                    <label key={r} className={s.reason}>
                      <input type="radio" name="reason" value={r} checked={reason === r} onChange={() => setReason(r)} />
                      {t(`report.reason.${r}` as MessageKey)}
                    </label>
                  ))}
                </fieldset>
                <label className={s.noteField}>
                  <span>{t("report.note")}</span>
                  <textarea value={note} maxLength={500} rows={3} onChange={(e) => setNote(e.target.value)} />
                </label>
                {problem && <p className={s.error} role="alert">{problem}</p>}
                <div className={s.actions}>
                  <button type="button" className={s.quiet} onClick={close}>{t("report.cancel")}</button>
                  <button type="submit" className={s.primary} disabled={!reason || send.isPending}>
                    {send.isPending ? t("report.sending") : t("report.send")}
                  </button>
                </div>
              </form>
            )}
          </div>
        </dialog>
      )}
      {signIn.error && <span className={s.error} role="alert">{signIn.error.message}</span>}
    </>
  );
}
