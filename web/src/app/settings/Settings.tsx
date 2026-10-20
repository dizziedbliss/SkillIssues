import { useState, type ReactNode } from "react";
import { Link, Navigate, NavLink, Route, Routes } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useT, type MessageKey } from "../../i18n";
import { useSession, useSignOut } from "../session";
import { useProfile } from "../profile/data";
import { SignInPrompt } from "../profile/parts";
import { ProfileForm } from "./ProfileForm";
import { Themes } from "./Themes";
import { ThemeEditorPage } from "./ThemeEditor";
import s from "./Settings.module.css";

const VIEWS: { path: string; title: MessageKey; signedIn: boolean }[] = [
  { path: "profile", title: "settings.profile", signedIn: true },
  { path: "themes", title: "settings.themes", signedIn: true },
  { path: "desktop", title: "settings.desktop", signedIn: false },
  { path: "account", title: "settings.account", signedIn: true },
  { path: "support", title: "settings.support", signedIn: false },
];

/*
 * Settings, one view at a time beside a menu of them (/app/settings/<view>):
 * your profile, themes (and a theme's editor), the desktop app, the account
 * (sign out everywhere, delete it; the Privacy policy points there) and
 * support. On a phone the menu becomes a row of tabs above the view.
 */
export function Settings() {
  const t = useT();
  const { user, isPending } = useSession();
  if (isPending) return null;

  const needsAccount = (el: ReactNode) => (user ? el : <SignInPrompt />);

  return (
    <div className={s.layout}>
      <nav className={s.sidebar} aria-label={t("settings.title")} data-ui="settings-nav">
        <h1 className={s.sideTitle}>{t("settings.title")}</h1>
        <ul>
          {VIEWS.filter((v) => user || !v.signedIn).map((v) => (
            <li key={v.path}>
              <NavLink to={`/app/settings/${v.path}`} className={({ isActive }) => (isActive ? s.current : undefined)}>
                {t(v.title)}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <div className={s.view}>
        <Routes>
          <Route index element={<Navigate to={user ? "profile" : "support"} replace />} />
          <Route path="profile" element={needsAccount(<ProfileView />)} />
          <Route path="themes" element={needsAccount(<View title="settings.themes"><Themes /></View>)} />
          <Route path="themes/:id" element={needsAccount(<ThemeEditorPage />)} />
          <Route path="desktop" element={<View title="settings.desktop"><Desktop /></View>} />
          <Route path="account" element={needsAccount(<View title="settings.account"><Account username={user?.username ?? ""} avatar={user?.avatar_url ?? ""} /></View>)} />
          <Route path="support" element={<View title="settings.support"><Support /></View>} />
          <Route path="*" element={<Navigate to="/app/settings" replace />} />
        </Routes>
      </div>
    </div>
  );
}

function View({ title, children }: { title: MessageKey; children: ReactNode }) {
  const t = useT();
  return (
    <section className={s.section} aria-labelledby="view-title">
      <h2 id="view-title" className={s.heading}>{t(title)}</h2>
      {children}
    </section>
  );
}

function ProfileView() {
  const t = useT();
  const profile = useProfile();
  return (
    <View title="settings.profile">
      <p className={s.body}>{t("settings.profile.lede")}</p>
      {profile.error && <p className={s.error} role="alert">{profile.error.message}</p>}
      {profile.data && <ProfileForm key={profile.data.id} profile={profile.data} />}
    </View>
  );
}

function Desktop() {
  const t = useT();
  return (
    <>
      <p className={s.body}>{t("settings.desktop.body")}</p>
      <p>
        <Link to="/download" className={s.button}>{t("settings.desktop.get")}</Link>
      </p>
    </>
  );
}

function Support() {
  const t = useT();
  return (
    <ul className={s.links}>
      <li><Link to="/docs">{t("settings.support.docs")}</Link></li>
      <li><Link to="/faq">{t("settings.support.faq")}</Link></li>
      <li><Link to="/contact">{t("settings.support.contact")}</Link></li>
      <li><Link to="/docs/report-a-bug">{t("settings.support.bug")}</Link></li>
      <li><Link to="/privacy">{t("settings.support.privacy")}</Link></li>
      <li><Link to="/terms">{t("settings.support.terms")}</Link></li>
      <li><Link to="/security">{t("settings.support.security")}</Link></li>
    </ul>
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
