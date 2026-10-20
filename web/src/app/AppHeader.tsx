import { useEffect, useId, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import { Logo } from "../landing/Logo";
import { LOCALES, useI18n } from "../i18n";
import { useThemeToggle } from "../lib/themeToggle";
import { useSession, useSignIn, useSignOut } from "./session";
import { useSearchOverlay } from "./search/context";
import { useNotifications } from "./notifications";
import s from "./AppHeader.module.css";

/**
 * The app's header: the wordmark, the sections, and Profile, which opens the
 * account menu (or reads "Sign in" while signed out). Sticks to the top.
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
    <header className={s.header}>
      <div className={s.bar}>
        {/* Inside the app the wordmark is home: Explore. */}
        <Link to="/app" className={s.home} aria-label={t("app.home")}>
          <Logo size="header" />
        </Link>
        <nav className={s.nav} aria-label={t("app.nav")}>
          <Link to="/app" className={exploring ? s.active : undefined} aria-current={exploring ? "page" : undefined}>
            {t("app.nav.explore")}
          </Link>
          <Link
            to="/app/contributions"
            className={pathname.startsWith("/app/contributions") ? s.active : undefined}
            aria-current={pathname.startsWith("/app/contributions") ? "page" : undefined}
          >
            {t("app.nav.contribute")}
          </Link>
          <button
            type="button"
            className={`${s.navButton} ${search.isOpen || pathname.startsWith("/app/search") ? s.active : ""}`}
            aria-haspopup="dialog"
            aria-keyshortcuts="/ Control+K Meta+K"
            onClick={search.open}
          >
            {t("app.nav.search")}
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

/** Profile's menu: settings, light and dark, language, sign out. */
function ProfileMenu() {
  const { t, locale, setLocale } = useI18n();
  const theme = useThemeToggle();
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

  const inAccount = /^\/app\/(settings|profile|saved)/.test(pathname);
  const unread = useNotifications().data?.unread_count ?? 0;

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
        {t("app.nav.profile")}
        {unread > 0 && (
          <span className={s.dot}>
            <span className="visually-hidden">{t("app.nav.unread", { count: unread })}</span>
          </span>
        )}
      </button>
      {open && (
        <div id={id} className={s.menu}>
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
            <li>
              <Link to="/app/settings" className={s.item}>{t("app.menu.settings")}</Link>
            </li>
            <li>
              <button type="button" role="switch" aria-checked={theme.on} className={s.item} onClick={theme.toggle} title={theme.label(t)}>
                <span>{t("app.menu.appearance")}</span>
                <span className={s.switch} aria-hidden="true" />
              </button>
              <Link to="/app/settings#themes" className={`${s.item} ${s.subItem}`}>{t("app.menu.addThemes")}</Link>
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
