import { useEffect, useId, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { LogoLive } from "../landing/LogoLive";
import { LOCALES, useI18n } from "../i18n";
import { useSession, useSignIn, useSignOut } from "./session";
import { useSearchOverlay } from "./search/context";
import { useNotifications } from "./notifications";
import { platform } from "./platform";
import { exploreURL } from "./explore/where";
import { choiceName, useAppearance } from "./theme";
import s from "./AppHeader.module.css";

/**
 * The app's header: the wordmark, the places you work (Explore, Learning path,
 * the desktop's Working on, Search), and Profile, which opens the account menu
 * (or reads "Sign in" while signed out). Sticks to the top.
 */
export function AppHeader() {
  const { t } = useI18n();
  const { user, isPending } = useSession();
  const signIn = useSignIn();
  const search = useSearchOverlay();
  const { pathname } = useLocation();
  // Explore covers the catalog and the issues opened from it.
  const exploring = pathname === "/app" || pathname === "/app/" || pathname.startsWith("/app/issues/");

  return (
    <header className={s.header} data-ui="header">
      <div className={s.bar}>
        {/* Inside the app the wordmark is home: Explore. */}
        <Link to="/app" className={s.home} aria-label={t("app.home")}>
          <LogoLive size="header" />
        </Link>
        <nav className={s.nav} aria-label={t("app.nav")} data-ui="nav">
          {/* Back to Explore as it was left; the wordmark starts it afresh. */}
          <Link to={exploreURL()} className={exploring ? s.active : undefined} aria-current={exploring ? "page" : undefined}>
            {t("app.nav.explore")}
          </Link>
          <Link
            to="/app/path"
            className={pathname.startsWith("/app/path") ? s.active : undefined}
            aria-current={pathname.startsWith("/app/path") ? "page" : undefined}
          >
            {t("app.nav.path")}
          </Link>
          {platform().Workspaces && (
            <Link
              to="/app/work"
              className={pathname.startsWith("/app/work") ? s.active : undefined}
              aria-current={pathname.startsWith("/app/work") ? "page" : undefined}
            >
              {t("app.nav.work")}
            </Link>
          )}
          <button
            type="button"
            className={`${s.navButton} ${search.isOpen || pathname.startsWith("/app/search") ? s.active : ""}`}
            aria-haspopup="dialog"
            aria-keyshortcuts="/ Control+K Meta+K"
            onClick={search.open}
          >
            {t("app.nav.search")}
            <kbd className={s.kbd} aria-hidden="true">/</kbd>
          </button>
          {user ? (
            <ProfileMenu />
          ) : (
            <button type="button" className={s.navButton} onClick={() => signIn.mutate()} disabled={isPending || signIn.isPending}>
              {signIn.isPending ? t("auth.opening") : t("app.nav.signIn")}
            </button>
          )}
        </nav>
      </div>
      {signIn.error && (
        <p className={s.error} role="alert">
          {signIn.error.message}
        </p>
      )}
    </header>
  );
}

/**
 * Profile's menu: the things about you (profile, saved issues, your
 * contributions; reports for moderators), then settings, appearance and
 * language, then sign out. Places you go to work are in the nav, not here.
 */
function ProfileMenu() {
  const { t, locale, setLocale } = useI18n();
  const { user } = useSession();
  const appearance = useAppearance();
  const signOut = useSignOut();
  const [open, setOpen] = useState(false);
  const [languages, setLanguages] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const { pathname } = useLocation();
  const id = useId();

  // Leaving the page closes it.
  useEffect(() => setOpen(false), [pathname]);

  useEffect(() => {
    if (!open) {
      setLanguages(false);
      return;
    }
    const onDown = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const inAccount = /^\/app\/(settings|profile|saved|contributions|moderation)/.test(pathname);
  const unread = useNotifications().data?.unread_count ?? 0;
  // Moderators get Reports in the menu; the server decides who is one.
  const me = useQuery({ queryKey: ["me"], queryFn: ({ signal }) => api.me({ signal }), staleTime: 5 * 60_000 });

  return (
    <div ref={root} className={s.menuRoot}>
      <button
        ref={trigger}
        type="button"
        className={`${s.navButton} ${open || inAccount ? s.active : ""}`}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((o) => !o)}
      >
        {user?.avatar_url && <img className={s.avatar} src={user.avatar_url} alt="" width={22} height={22} />}
        {t("app.nav.profile")}
        <svg className={s.chevron} width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" aria-hidden="true">
          <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {unread > 0 && (
          <span className={s.dot}>
            <span className="visually-hidden">{t("app.nav.unread", { count: unread })}</span>
          </span>
        )}
      </button>
      {open && (
        <div id={id} className={s.menu} data-ui="menu">
          <ul>
            <li>
              <Link to="/app/profile" className={s.item}>{t("app.menu.profile")}</Link>
            </li>
            <li>
              <Link to="/app/saved" className={s.item}>
                <span>{t("app.menu.saved")}</span>
                {unread > 0 && <span className={s.count}>{unread}</span>}
              </Link>
            </li>
            <li>
              <Link to="/app/contributions" className={s.item}>{t("app.menu.contributions")}</Link>
            </li>
            {me.data?.moderator && (
              <li>
                <Link to="/app/moderation" className={s.item}>{t("mod.title")}</Link>
              </li>
            )}
            <li className={s.separator} role="separator" />
            <li>
              <Link to="/app/settings/profile" className={s.item}>{t("app.menu.settings")}</Link>
            </li>
            <li>
              <span className={`${s.item} ${s.heading}`}>{t("app.menu.appearance")}</span>
              {/* The last few themes, to switch between in one click. */}
              <ul className={s.recent}>
                {appearance.recent.map((c) => {
                  const name = choiceName(c);
                  const inUse = choiceName(appearance.current) === name;
                  return (
                    <li key={name}>
                      {inUse ? (
                        // The theme in use: its name, and a switch between its light and dark halves.
                        <span className={`${s.item} ${s.subItem}`} aria-current="true">
                          <span className={s.current}>
                            <span aria-hidden="true">✓</span> {name}
                          </span>
                          {appearance.hasModes && (
                            <button
                              type="button"
                              role="switch"
                              aria-checked={appearance.mode === "dark"}
                              aria-label={t("app.menu.dark")}
                              title={t(appearance.mode === "dark" ? "mode.toLight" : "mode.toDark")}
                              className={s.modeSwitch}
                              onClick={appearance.toggleMode}
                            >
                              <span aria-hidden="true">
                                {appearance.mode === "dark" ? t("app.menu.dark") : t("app.menu.light")}
                              </span>
                              <span className={s.switch} aria-hidden="true" />
                            </button>
                          )}
                        </span>
                      ) : (
                        <button
                          type="button"
                          className={`${s.item} ${s.subItem}`}
                          disabled={appearance.choosing}
                          onClick={() => appearance.choose(c)}
                        >
                          <span>{name}</span>
                        </button>
                      )}
                    </li>
                  );
                })}
              </ul>
              <Link to="/app/settings/themes" className={`${s.item} ${s.subItem} ${s.more}`}>
                <span>{t("app.menu.seeMore")}</span>
                <span aria-hidden="true">→</span>
              </Link>
            </li>
            <li>
              <button type="button" className={s.item} aria-expanded={languages} onClick={() => setLanguages((l) => !l)}>
                <span>{t("app.menu.language")}</span>
                <span className={s.value}>{LOCALES.find((l) => l.code === locale)?.name}</span>
              </button>
              {languages && (
                <fieldset className={s.languages}>
                  <legend className="visually-hidden">{t("language.choose")}</legend>
                  {LOCALES.map((l) => (
                    <label key={l.code} lang={l.code}>
                      <input type="radio" name={`${id}-locale`} checked={locale === l.code} onChange={() => setLocale(l.code)} />
                      {l.name}
                    </label>
                  ))}
                  <p>{t("language.more")}</p>
                </fieldset>
              )}
            </li>
            <li className={s.separator} role="separator" />
            <li>
              <button type="button" className={`${s.item} ${s.signOut}`} onClick={() => signOut.mutate()} disabled={signOut.isPending}>
                {signOut.isPending ? t("app.menu.signingOut") : t("app.menu.signOut")}
              </button>
            </li>
          </ul>
          {signOut.error && (
            <p className={s.menuError} role="alert">
              {signOut.error.message}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
