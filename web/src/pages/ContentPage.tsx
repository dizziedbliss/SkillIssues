import { Link } from "react-router";
import tokens from "../landing/tokens.module.css";
import { Header } from "../landing/Header";
import { Footer } from "../landing/Footer";
import type { Block, Page } from "./content";
import s from "./ContentPage.module.css";

function render(b: Block, i: number) {
  if (typeof b === "string") return <p key={i} className={s.p}>{b}</p>;
  if ("h" in b) return <h2 key={i} className={s.h2}>{b.h}</h2>;
  if ("list" in b) {
    return (
      <ul key={i} className={s.list}>
        {b.list.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    );
  }
  const { label, to, href } = b.link;
  return (
    <p key={i} className={s.linkLine}>
      {to ? <Link to={to} className={s.button}>{label}</Link> : <a href={href} className={s.button}>{label}</a>}
    </p>
  );
}

/** A written page of the public site, in the landing page's locked style. */
export function ContentPage({ page }: { page: Page }) {
  return (
    <div className={tokens.page}>
      <Header />
      <main className={s.main}>
        <article>
          <h1 className={s.title}>{page.title}</h1>
          <p className={s.lede}>{page.lede}</p>
          {page.draft && (
            <p className={s.draft} role="note">
              Draft: this page is being reviewed and may change before it is final.
            </p>
          )}
          <div className={s.body}>{page.body.map(render)}</div>
        </article>
      </main>
      <Footer />
    </div>
  );
}
