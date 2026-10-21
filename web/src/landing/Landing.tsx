import { Link } from "react-router";
import tokens from "./tokens.module.css";
import s from "./Landing.module.css";
import { Header } from "./Header";
import { Footer } from "./Footer";
import { LiveBar } from "./LiveBar";
import { GlyphNoise } from "./GlyphNoise";
import { Logo } from "./Logo";
import { external } from "../lib/links";
import { useT } from "../i18n";
import { useDocumentTitle } from "../lib/title";

/**
 * The landing page, as the owner designed it: a zig-zag of sections, each in
 * the left or right part of a container with equal margins, the other side
 * left white, over a faint glyph-noise background. One column on narrow
 * screens. Its look is locked (tokens.module.css); themes never reach it.
 */
export function Landing() {
  useDocumentTitle(null);
  const t = useT();
  return (
    <div className={`${tokens.page} ${s.page}`}>
      <GlyphNoise />
      <Header />
      <main className={s.container}>
        <section className={s.hero} aria-labelledby="hero-title">
          <h1 id="hero-title" className={s.heading}>{t("landing.hero.title")}</h1>
          <p className={s.body}>{t("landing.hero.body")}</p>
          <LiveBar />
        </section>

        <section className={`${s.section} ${s.right}`} aria-labelledby="us-title">
          <h2 id="us-title" className={s.heading}>{t("landing.us.title")}</h2>
          <p className={s.body}>{t("landing.us.body1")}</p>
          <p className={s.body}>{t("landing.us.body2")}</p>
        </section>

        <section className={`${s.section} ${s.centred}`} aria-labelledby="made-title">
          <p className={s.made}>
            {t("landing.made.lead")} <Logo size="inline" />
          </p>
          <h2 id="made-title" className={s.heading}>{t("landing.made.title")}</h2>
          <p className={s.body}>{t("landing.made.body")}</p>
          <Link to="/app" className={s.button}>{t("landing.made.cta")}</Link>
        </section>

        <section className={`${s.section} ${s.left}`} aria-labelledby="dev-title">
          <p className={s.kicker}>{t("landing.dev.kicker")}</p>
          <h2 id="dev-title" className={s.heading}>{t("landing.dev.title")}</h2>
          <p className={s.body}>{t("landing.dev.body")}</p>
          <Link to="/app" className={s.button}>{t("landing.dev.cta")}</Link>
        </section>

        <section className={`${s.section} ${s.right}`} aria-labelledby="docs-title">
          <h2 id="docs-title" className={s.heading}>{t("landing.docs.title")}</h2>
          <p className={s.body}>{t("landing.docs.body")}</p>
        </section>

        <section className={`${s.section} ${s.left}`} aria-labelledby="maint-title">
          <p className={s.kicker}>{t("landing.maint.kicker")}</p>
          <h2 id="maint-title" className={s.heading}>{t("landing.maint.title")}</h2>
          <p className={s.body}>{t("landing.maint.body")}</p>
          <Link to="/maintainers" className={s.button}>{t("landing.maint.cta")}</Link>
        </section>

        <section className={`${s.section} ${s.right}`} aria-labelledby="desk-title">
          <h2 id="desk-title" className={s.heading}>{t("landing.desk.title")}</h2>
          <p className={s.body}>{t("landing.desk.body1")}</p>
          <p className={s.body}>{t("landing.desk.body2")}</p>
          <p className={s.ctaLine}>
            <Link to="/download" className={s.button}>{t("landing.desk.cta")}</Link>
            <span className={s.aside}>{t("landing.desk.aside")}</span>
          </p>
        </section>

        <section className={`${s.section} ${s.left}`} aria-labelledby="effort-title">
          <h2 id="effort-title" className={s.heading}>{t("landing.effort.title")}</h2>
          <p className={s.body}>{t("landing.effort.body")}</p>
        </section>

        <section className={`${s.section} ${s.left}`} aria-labelledby="trust-title">
          <p className={s.kicker}>{t("landing.trust.kicker")}</p>
          <h2 id="trust-title" className={s.heading}>{t("landing.trust.title")}</h2>
          <p className={s.body}>{t("landing.trust.body")}</p>
          <Link to="/security" className={s.button}>{t("landing.trust.cta")}</Link>
        </section>

        <section className={`${s.section} ${s.right}`} aria-labelledby="live-title">
          <h2 id="live-title" className={s.heading}>{t("landing.live.title")}</h2>
          <p className={s.body}>{t("landing.live.body")}</p>
          <a href={external.live} className={s.button}>{t("landing.live.cta")}</a>
        </section>

        <section className={`${s.closing} ${s.centred}`} aria-labelledby="end-title">
          <h2 id="end-title" className={s.heading}>{t("landing.end.title")}</h2>
          <p className={s.body}>{t("landing.end.body")}</p>
          <Link to="/app" className={s.button}>{t("landing.end.cta")}</Link>
        </section>
      </main>
      <Footer />
    </div>
  );
}
