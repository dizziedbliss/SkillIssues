import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";
import { Link, Route, Routes, useSearchParams } from "react-router";
import { Footer } from "../landing/Footer";
import { useT, type MessageKey } from "../i18n";
import { ThemeToggleContext } from "../lib/themeToggle";
import { AppHeader } from "./AppHeader";
import { Explore } from "./explore/Explore";
import { IssuePage } from "./IssuePage";
import { SavedPage } from "./SavedPage";
import { ProfilePage } from "./profile/ProfilePage";
import { ContributionsPage } from "./profile/ContributionsPage";
import { PathPage } from "./path/PathPage";
import { PublicProfilePage } from "./people/PublicProfilePage";
import { LeaderboardPage } from "./people/LeaderboardPage";
import { Settings } from "./settings/Settings";
import { AppearanceProvider, useAppTheme } from "./theme";
import { useRouteSettled, useTransitionNavigate } from "./transition";
import { SearchContext } from "./search/context";
import { SearchOverlay } from "./search/SearchOverlay";
import { SearchPage } from "./search/SearchPage";
import "./transition.css";
import "./fonts.css";
import s from "./App.module.css";

/** The fixed codes the API's sign-in callback can return (?error=). */
const signInErrors: Record<string, MessageKey> = {
  access_denied: "auth.error.access_denied",
  invalid_callback: "auth.error.invalid_callback",
  oauth_failed: "auth.error.oauth_failed",
  sign_in_failed: "auth.error.sign_in_failed",
};

/**
 * The signed-in app: its themed root, header and footer around the page for
 * the route. The theme reaches only this root (theme.ts, APP_SCOPE); the landing page
 * and the docs never see it.
 */
export default function AppShell() {
  const t = useT();
  const root = useRef<HTMLDivElement>(null);
  const { appearance, toggle } = useAppTheme(root);
  const problem = appearance.problem;
  const [params, setParams] = useSearchParams();
  const go = useTransitionNavigate();
  useRouteSettled();

  const [searching, setSearching] = useState(false);
  const openSearch = useCallback(() => setSearching(true), []);
  const search = useMemo(() => ({ isOpen: searching, open: openSearch }), [searching, openSearch]);

  // "/" (outside a text field) and Ctrl/⌘K open search.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && (e.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName));
      if ((e.key === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing && !e.metaKey && !e.ctrlKey && !e.altKey)) {
        e.preventDefault();
        setSearching(true);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  // A plain click on a link to another app page moves with the theme's transition.
  function onClickCapture(e: MouseEvent<HTMLDivElement>) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (!a || !href || a.target || a.hasAttribute("download") || !/^\/app(\/|\?|$)/.test(href)) return;
    e.preventDefault();
    go(href);
  }

  // Back from GitHub: ?auth=ok is done with; ?error= is shown until dismissed.
  const callbackError = params.get("error");
  const dismiss = () =>
    setParams(
      (p) => {
        p.delete("error");
        p.delete("auth");
        return p;
      },
      { replace: true },
    );

  return (
    <ThemeToggleContext.Provider value={toggle}>
      <AppearanceProvider value={appearance}>
      <SearchContext.Provider value={search}>
        {/* data-app is APP_SCOPE: the theme applies here and nowhere else. */}
        <div ref={root} className={s.app} data-app="" onClickCapture={onClickCapture}>
          <AppHeader />
          {callbackError && (
            <p className={s.notice} role="alert">
              {t(signInErrors[callbackError] ?? "auth.error.other")}
              <button type="button" onClick={dismiss}>{t("auth.dismiss")}</button>
            </p>
          )}
          {problem && (
            <p className={s.problem} role="alert">
              {t("theme.failed", { reason: problem })}
            </p>
          )}
          <main className={s.main} data-ui="page">
            <Routes>
              <Route index element={<Explore />} />
              <Route path="issues/:id" element={<IssuePage />} />
              <Route path="search" element={<SearchPage />} />
              <Route path="profile" element={<ProfilePage />} />
              <Route path="contributions" element={<ContributionsPage />} />
              <Route path="saved" element={<SavedPage />} />
              <Route path="path" element={<PathPage />} />
              <Route path="u/:login" element={<PublicProfilePage />} />
              <Route path="leaderboard" element={<LeaderboardPage />} />
              <Route path="settings/*" element={<Settings />} />
              <Route path="*" element={<AppNotFound />} />
            </Routes>
          </main>
          <Footer />
          {searching && <SearchOverlay onClose={() => setSearching(false)} />}
        </div>
      </SearchContext.Provider>
      </AppearanceProvider>
    </ThemeToggleContext.Provider>
  );
}

function AppNotFound() {
  const t = useT();
  return (
    <section>
      <h1 className={s.title}>{t("app.notFound.title")}</h1>
      <p className={s.lede}>{t("app.notFound.body")}</p>
      <Link to="/app" className={s.link}>{t("app.notFound.cta")}</Link>
    </section>
  );
}
