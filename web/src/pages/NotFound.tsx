import { Link } from "react-router";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import { useT } from "../i18n";
import s from "./ContentPage.module.css";

export function NotFound() {
  const t = useT();
  return (
    <div className={tokens.page}>
      <Header />
      <main className={s.main}>
        <h1 className={s.title}>{t("page.notFound.title")}</h1>
        <p className={s.lede}>{t("page.notFound.body")}</p>
        <p className={s.linkLine}>
          <Link to="/app" className={s.button}>{t("page.notFound.cta")}</Link>
        </p>
      </main>
    </div>
  );
}
