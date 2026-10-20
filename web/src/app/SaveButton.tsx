import type { Issue } from "@skillissues/client";
import { useT } from "../i18n";
import { BookmarkIcon } from "./icons";
import { useSaved, useToggleSaved } from "./saved";
import { useSession, useSignIn } from "./session";
import s from "./SaveButton.module.css";

/**
 * The bookmark. Signed out, it starts a sign-in; signed in, it saves or
 * unsaves, and says so if that failed. `label` shows the word beside it.
 */
export function SaveButton({ issue, label = false }: { issue: Issue; label?: boolean }) {
  const t = useT();
  const { user } = useSession();
  const signIn = useSignIn();
  const { ids } = useSaved();
  const toggle = useToggleSaved();
  const saved = ids.has(issue.id);
  const name = !user ? t("issue.saveSignIn") : saved ? t("issue.unsave") : t("issue.save");
  const error = toggle.error ?? signIn.error;

  return (
    <span className={s.wrap}>
      <button
        type="button"
        className={`${s.button} ${label ? s.labelled : ""}`}
        data-ui="bookmark"
        aria-pressed={user ? saved : undefined}
        aria-label={label ? undefined : name}
        title={name}
        disabled={toggle.isPending || signIn.isPending}
        onClick={() => (user ? toggle.mutate({ issue, save: !saved }) : signIn.mutate())}
      >
        <BookmarkIcon filled={saved} />
        {label && <span>{saved ? t("issue.saved") : t("issue.saveLong")}</span>}
      </button>
      {error && (
        <span className={s.error} role="alert">
          {t("issue.saveFailed", { reason: error.message })}
        </span>
      )}
    </span>
  );
}
