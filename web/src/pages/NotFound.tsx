import { Link } from "react-router";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import s from "./ContentPage.module.css";

export function NotFound() {
  return (
    <div className={tokens.page}>
      <Header />
      <main className={s.main}>
        <h1 className={s.title}>There’s no page here.</h1>
        <p className={s.lede}>The link may be old, or mistyped. There are plenty of open issues, though.</p>
        <p className={s.linkLine}>
          <Link to="/app" className={s.button}>Explore open issues</Link>
        </p>
      </main>
    </div>
  );
}
