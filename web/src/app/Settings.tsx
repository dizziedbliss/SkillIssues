import { useState } from "react";
import { Link } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { useT } from "../i18n";
import { useSession, useSignIn, useSignOut } from "./session";
import s from "./Settings.module.css";

/*
 * Settings. The account: sign out everywhere, and delete the account (the
 * Privacy policy points here). Support links close the page.
 */
export function Settings() {
  const t = useT();
  const queryClient = useQueryClient();
  const { user, isPending } = useSession();
  const signIn = useSignIn();
  const everywhere = useSignOut(true);
  const [confirming, setConfirming] = useState(false);
  const [deleted, setDeleted] = useState<string | null>(null);

  const remove = useMutation({
    mutationFn: () => api.deleteAccount(),
    onSuccess: (res) => {
      setConfirming(false);
      setDeleted(res.authorized_apps_url);
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== "session" });
      return queryClient.invalidateQueries({ queryKey: ["session"] });
    },
  });

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("settings.title")}</h1>

      {deleted && (
        <p className={s.notice} role="status">
          {t("settings.deleted.before")}{" "}
          <a href={deleted} target="_blank" rel="noreferrer">{t("settings.deleted.link")}</a>.
        </p>
      )}

      <section className={s.section} aria-labelledby="account-title">
        <h2 id="account-title" className={s.heading}>{t("settings.account")}</h2>
        {isPending ? null : user ? (
          <>
            <p className={s.who}>
              {user.avatar_url && <img src={user.avatar_url} alt="" width={32} height={32} />}
              {t("settings.signedInAs", { name: user.username })}
            </p>

            <div className={s.row}>
              <div>
                <h3 className={s.label}>{t("settings.everywhere")}</h3>
                <p className={s.body}>{t("settings.everywhere.body")}</p>
              </div>
              <button type="button" className={s.button} onClick={() => everywhere.mutate()} disabled={everywhere.isPending}>
                {everywhere.isPending ? t("settings.everywhere.pending") : t("settings.everywhere")}
              </button>
            </div>
            {everywhere.error && <p className={s.error} role="alert">{everywhere.error.message}</p>}

            <div className={s.row}>
              <div>
                <h3 className={s.label}>{t("settings.delete")}</h3>
                <p className={s.body}>{t("settings.delete.body")}</p>
              </div>
              {!confirming && (
                <button type="button" className={s.button} onClick={() => setConfirming(true)}>
                  {t("settings.delete")}
                </button>
              )}
            </div>
            {confirming && (
              <div className={s.confirm} role="group" aria-label={t("settings.delete")}>
                <button type="button" className={s.danger} onClick={() => remove.mutate()} disabled={remove.isPending}>
                  {remove.isPending ? t("settings.delete.pending") : t("settings.delete.confirm")}
                </button>
                <button type="button" className={s.quietButton} onClick={() => setConfirming(false)}>
                  {t("settings.delete.keep")}
                </button>
              </div>
            )}
            {remove.error && <p className={s.error} role="alert">{remove.error.message}</p>}
          </>
        ) : (
          <div className={s.row}>
            <p className={s.body}>{t("settings.signedOut")}</p>
            <button type="button" className={s.button} onClick={() => signIn.mutate()} disabled={signIn.isPending}>
              {signIn.isPending ? t("auth.opening") : t("auth.signIn")}
            </button>
          </div>
        )}
        {signIn.error && <p className={s.error} role="alert">{signIn.error.message}</p>}
      </section>

      <section className={s.section} aria-labelledby="support-title">
        <h2 id="support-title" className={s.heading}>{t("settings.support")}</h2>
        <ul className={s.links}>
          <li><Link to="/docs">{t("settings.support.docs")}</Link></li>
          <li><Link to="/faq">{t("settings.support.faq")}</Link></li>
          <li><Link to="/contact">{t("settings.support.contact")}</Link></li>
          <li><Link to="/docs/report-a-bug">{t("settings.support.bug")}</Link></li>
          <li><Link to="/privacy">{t("settings.support.privacy")}</Link></li>
          <li><Link to="/terms">{t("settings.support.terms")}</Link></li>
          <li><Link to="/security">{t("settings.support.security")}</Link></li>
        </ul>
      </section>
    </div>
  );
}
