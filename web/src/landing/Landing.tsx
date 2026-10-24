import { Link } from "react-router";
import tokens from "./tokens.module.css";
import s from "./Landing.module.css";
import { Header } from "./Header";
import { Footer } from "./Footer";
import { BacklogClock } from "./BacklogClock";
import { IssueFinder } from "./IssueFinder";
import { RepoFinder } from "./RepoFinder";
import { BacklogField } from "./BacklogField";
import { Logo } from "./Logo";
import { external } from "../lib/links";
import { useT, type MessageKey } from "../i18n";
import { useDocumentTitle } from "../lib/title";

/** Getting started's four steps (/docs/getting-started), in order. */
const STEPS: { title: MessageKey; body: MessageKey }[] = [
  { title: "landing.steps.find", body: "landing.steps.find.body" },
  { title: "landing.steps.say", body: "landing.steps.say.body" },
  { title: "landing.steps.fix", body: "landing.steps.fix.body" },
  { title: "landing.steps.pr", body: "landing.steps.pr.body" },
];

/** What XP is made of, as /docs/xp describes it. */
const XP: { title: MessageKey; body: MessageKey }[] = [
  { title: "landing.xp.levels", body: "landing.xp.levels.body" },
  { title: "landing.xp.badges", body: "landing.xp.badges.body" },
  { title: "landing.xp.streaks", body: "landing.xp.streaks.body" },
];

/**
 * The landing page, as the owner designed it: a zig-zag of sections, each in
 * the left or right part of a container with equal margins, the other side
 * left white, over the backlog field (BacklogField.tsx). One column on narrow
 * screens. Its look is locked (tokens.module.css); themes never reach it.
 * The hero is the backlog itself, live, and a way into it with no account.
 */
export function Landing() {
  useDocumentTitle(null);
  const t = useT();
  return (
    <div className={`${tokens.page} ${s.page}`}>
      <BacklogField />
      <Header />
      <main className={s.container}>
        <section className={s.hero} aria-labelledby="hero-title">
          <h1 id="hero-title" className={s.heading}>{t("landing.hero.title")}</h1>
          <p className={s.body}>{t("landing.hero.body")}</p>
          <BacklogClock />
        </section>

        <section className={s.section} aria-labelledby="finder-title">
          <IssueFinder />
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
        </section>

        <section className={s.section} aria-labelledby="steps-title">
          <div className={s.left}>
            <h2 id="steps-title" className={s.heading}>{t("landing.steps.title")}</h2>
          </div>
          <ol className={s.steps}>
            {STEPS.map((step, i) => (
              <li key={step.title} className={s.step}>
                <span className={s.stepNumber} aria-hidden="true">{i + 1}</span>
                <h3 className={s.stepTitle}>{t(step.title)}</h3>
                <p className={s.stepBody}>{t(step.body)}</p>
              </li>
            ))}
          </ol>
          <div className={s.right}>
            <p className={s.lead}>{t("landing.desk.title")}</p>
            <p className={s.body}>{t("landing.desk.body1")}</p>
            <p className={s.ctaLine}>
              <Link to="/download" className={s.button}>{t("landing.desk.cta")}</Link>
              <span className={s.aside}>{t("landing.desk.aside")}</span>
            </p>
            <p className={s.ctaLine}>
              <Link to="/docs/getting-started" className={s.link}>{t("landing.steps.guide")}</Link>
            </p>
          </div>
        </section>

        <section className={`${s.section} ${s.split}`} aria-label={t("landing.split.label")}>
          <div>
            <p className={s.kicker}>{t("landing.dev.kicker")}</p>
            <h2 className={s.heading}>{t("landing.dev.title")}</h2>
            <p className={s.body}>{t("landing.dev.body")}</p>
            <Link to="/app" className={s.button}>{t("landing.dev.cta")}</Link>
          </div>
          <div>
            <p className={s.kicker}>{t("landing.maint.kicker")}</p>
            <h2 className={s.heading}>{t("landing.maint.title")}</h2>
            <p className={s.body}>{t("landing.maint.body")}</p>
            <Link to="/maintainers" className={s.button}>{t("landing.maint.cta")}</Link>
          </div>
        </section>

        <section className={s.section} aria-labelledby="repo-title">
          <RepoFinder />
        </section>

        <section className={`${s.section} ${s.left}`} aria-labelledby="trust-title">
          <p className={s.kicker}>{t("landing.trust.kicker")}</p>
          <h2 id="trust-title" className={s.heading}>{t("landing.trust.title")}</h2>
          <p className={s.body}>{t("landing.trust.body1")}</p>
          <p className={s.body}>{t("landing.trust.body2")}</p>
          <Link to="/security" className={s.button}>{t("landing.trust.cta")}</Link>
        </section>

        <section className={`${s.section} ${s.right}`} aria-labelledby="xp-title">
          <p className={s.kicker}>{t("landing.xp.kicker")}</p>
          <h2 id="xp-title" className={s.heading}>{t("landing.xp.title")}</h2>
          <p className={s.body}>{t("landing.xp.body")}</p>
          <dl className={s.facts}>
            {XP.map((f) => (
              <div key={f.title} className={s.fact}>
                <dt>{t(f.title)}</dt>
                <dd>{t(f.body)}</dd>
              </div>
            ))}
          </dl>
          <p className={s.ctaLine}>
            <Link to="/docs/xp" className={s.button}>{t("landing.xp.cta")}</Link>
            <span className={s.aside}>{t("landing.xp.aside")}</span>
          </p>
        </section>

        <section className={`${s.section} ${s.centred}`} aria-labelledby="live-title">
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
