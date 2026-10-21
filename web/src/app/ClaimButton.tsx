import { useI18n } from "../i18n";
import { useClaimState, useToggleClaim } from "./claims";
import { useSession, useSignIn } from "./session";
import s from "./ClaimButton.module.css";

/**
 * "I'm working on this": a claim other SkillIssues users see, for a few days
 * or while your pull request for it is open. It lives on SkillIssues only, so
 * it asks you to say so on GitHub too.
 */
export function ClaimButton({ issueID, url, className }: { issueID: number; url: string; className?: string }) {
  const { t, locale } = useI18n();
  const { user } = useSession();
  const signIn = useSignIn();
  const state = useClaimState(issueID);
  const toggle = useToggleClaim(issueID);
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });
  const yours = state.data?.yours ?? null;
  const error = toggle.error ?? signIn.error ?? state.error;

  if (state.data?.claimed && !yours) {
    return <p className={s.note} data-ui="claim" data-ui-state="taken">{t("claim.taken")}</p>;
  }
  return (
    <span className={s.wrap} data-ui="claim" data-ui-state={yours ? "yours" : "free"}>
      {yours ? (
        <>
          <span className={s.note}>
            {yours.extended ? t("claim.yoursExtended") : t("claim.yours", { date: date.format(new Date(yours.expires_at)) })}{" "}
            <a href={url} target="_blank" rel="noreferrer">{t("claim.tellGitHub")}</a>
          </span>
          <button type="button" className={s.link} onClick={() => toggle.mutate(false)} disabled={toggle.isPending}>
            {t("claim.release")}
          </button>
        </>
      ) : (
        <button
          type="button"
          className={className}
          title={t("claim.help")}
          onClick={() => (user ? toggle.mutate(true) : signIn.mutate())}
          disabled={toggle.isPending || signIn.isPending || state.isPending}
        >
          {user ? t("claim.claim") : t("claim.signIn")}
        </button>
      )}
      {error && <span className={s.error} role="alert">{error.message}</span>}
    </span>
  );
}
