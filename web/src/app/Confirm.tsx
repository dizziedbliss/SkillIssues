import { useEffect, useRef, useState, type ReactNode } from "react";
import { useT } from "../i18n";
import s from "./Confirm.module.css";

/*
 * "Are you sure?" before anything that can't simply be undone: giving up a
 * path, releasing a claim, deleting a theme. One dialog for the whole app,
 * asked as a promise: `if (await confirm({...})) doIt()`.
 */

export interface ConfirmOptions {
  title: string;
  body: ReactNode;
  /** The confirming button says what happens: "Give up the path", not "OK". */
  confirm: string;
}

export function useConfirm(): [ReactNode, (o: ConfirmOptions) => Promise<boolean>] {
  const [asking, setAsking] = useState<{ o: ConfirmOptions; resolve: (yes: boolean) => void } | null>(null);
  const element = asking && (
    <ConfirmDialog
      {...asking.o}
      onDone={(yes) => {
        asking.resolve(yes);
        setAsking(null);
      }}
    />
  );
  return [element, (o) => new Promise((resolve) => setAsking({ o, resolve }))];
}

function ConfirmDialog({ title, body, confirm, onDone }: ConfirmOptions & { onDone: (yes: boolean) => void }) {
  const t = useT();
  const dialog = useRef<HTMLDialogElement>(null);
  const answer = useRef(false);
  useEffect(() => {
    if (dialog.current && !dialog.current.open) dialog.current.showModal();
  }, []);
  const close = (yes: boolean) => {
    answer.current = yes;
    dialog.current?.close();
  };
  return (
    // Escape and Cancel both close it as a no; only the confirming button says yes.
    <dialog ref={dialog} className={s.dialog} aria-labelledby="confirm-title" onClose={() => onDone(answer.current)} data-ui="confirm">
      <h2 id="confirm-title" className={s.title}>{title}</h2>
      <div className={s.body}>{body}</div>
      <div className={s.actions}>
        <button type="button" className={s.cancel} onClick={() => close(false)} autoFocus>
          {t("confirm.cancel")}
        </button>
        <button type="button" className={s.confirm} onClick={() => close(true)}>
          {confirm}
        </button>
      </div>
    </dialog>
  );
}
