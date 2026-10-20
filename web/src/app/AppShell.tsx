import { useRef } from "react";
import { Link, Route, Routes, useSearchParams } from "react-router";
import { Footer } from "../landing/Footer";
import { useT, type MessageKey } from "../i18n";
import { ThemeToggleContext } from "../lib/themeToggle";
import { AppHeader } from "./AppHeader";
import { Explore } from "./explore/Explore";
import { IssuePage } from "./IssuePage";
import { Settings } from "./Settings";
import { useAppTheme } from "./theme";
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
  const { toggle, problem } = useAppTheme(root);
  const [params, setParams] = useSearchParams();

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
      {/* data-app is APP_SCOPE: the theme applies here and nowhere else. */}
      <div ref={root} className={s.app} data-app="">
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
        <main className={s.main}>
          <Routes>
            <Route index element={<Explore />} />
            <Route path="issues/:id" element={<IssuePage />} />
            <Route path="settings" element={<Settings />} />
            <Route path="*" element={<AppNotFound />} />
          </Routes>
        </main>
        <Footer />
      </div>
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
