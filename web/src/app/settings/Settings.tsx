import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useT, type MessageKey } from "../../i18n";
import { useSession, useSignOut } from "../session";
import { useProfile } from "../profile/data";
import { SignInPrompt } from "../profile/parts";
import { ProfileForm } from "./ProfileForm";
import { Themes } from "./Themes";
import s from "./Settings.module.css";

const SECTIONS: { id: string; title: MessageKey }[] = [
  { id: "profile", title: "settings.profile" },
  { id: "themes", title: "settings.themes" },
  { id: "desktop", title: "settings.desktop" },
  { id: "account", title: "settings.account" },
  { id: "support", title: "settings.support" },
];

/*
 * Settings: your profile, themes, the desktop app, the account (sign out
 * everywhere, delete it; the Privacy policy points here) and support. A link
 * to a section (#themes, from Appearance's "Add more") scrolls to it.
 */
export function Settings() {
  const t = useT();
  const { hash } = useLocation();
  const { user, isPending } = useSession();
  const profile = useProfile();

  useEffect(() => {
    if (!hash) return;
    document.getElementById(hash.slice(1))?.scrollIntoView({ block: "start" });
  }, [hash, user, profile.isSuccess]);

  if (isPending) return null;

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("settings.title")}</h1>
      {user && (
        <nav className={s.toc} aria-label={t("settings.nav")}>
          {SECTIONS.map((x) => (
            <a key={x.id} href={`#${x.id}`}>{t(x.title)}</a>
          ))}
        </nav>
      )}

      {!user ? (
        <div className={s.section}>
          <SignInPrompt />
        </div>
      ) : (
        <>
          <section id="profile" className={s.section} aria-labelledby="profile-title">
            <h2 id="profile-title" className={s.heading}>{t("settings.profile")}</h2>
            <p className={s.body}>{t("settings.profile.lede")}</p>
            {profile.error && <p className={s.error} role="alert">{profile.error.message}</p>}
            {profile.data && <ProfileForm key={profile.data.id} profile={profile.data} />}
          </section>

          <section id="themes" className={s.section} aria-labelledby="themes-title">
            <h2 id="themes-title" className={s.heading}>{t("settings.themes")}</h2>
            <Themes />
          </section>

          <section id="desktop" className={s.section} aria-labelledby="desktop-title">
            <h2 id="desktop-title" className={s.heading}>{t("settings.desktop")}</h2>
            <p className={s.body}>{t("settings.desktop.body")}</p>
            <p>
              <Link to="/download" className={s.button}>{t("settings.desktop.get")}</Link>
            </p>
          </section>

          <section id="account" className={s.section} aria-labelledby="account-title">
            <h2 id="account-title" className={s.heading}>{t("settings.account")}</h2>
            <Account username={user.username} avatar={user.avatar_url} />
          </section>
        </>
      )}

      <section id="support" className={s.section} aria-labelledby="support-title">
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

/** Who's signed in, sign out everywhere, and deleting the account. */
function Account({ username, avatar }: { username: string; avatar: string }) {
  const t = useT();
  const queryClient = useQueryClient();
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

  if (deleted) {
    return (
      <p className={s.notice} role="status">
        {t("settings.deleted.before")} <a href={deleted} target="_blank" rel="noreferrer">{t("settings.deleted.link")}</a>.
      </p>
    );
  }

  return (
    <>
      <p className={s.who}>
        {avatar && <img src={avatar} alt="" width={32} height={32} />}
        {t("settings.signedInAs", { name: username })}
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
  );
}
