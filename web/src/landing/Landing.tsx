import type { ReactNode } from "react";
import { Link } from "react-router";
import tokens from "./tokens.module.css";
import s from "./Landing.module.css";
import { Header } from "./Header";
import { Footer } from "./Footer";
import { Hero } from "./Hero";
import { RepoFinder } from "./RepoFinder";
import { BacklogField } from "./BacklogField";
import { Logo } from "./Logo";
import { ActivityWidget, LevelWidget, StepsWidget, TrustWidget } from "./Widgets";
import { external } from "../lib/links";
import { useT, type MessageKey } from "../i18n";
import { useDocumentTitle } from "../lib/title";

/**
 * The comparison's rows: what each kind of tool gives you. Kinds, never names:
 * a claim about a named product would have to stay true as it changes, so the
 * others' cells are hedged ("often", "usually").
 */
const COMPARE: { row: MessageKey; lists: MessageKey; finders: MessageKey; us: MessageKey }[] = [
  { row: "landing.diff.get", lists: "landing.diff.get.lists", finders: "landing.diff.get.finders", us: "landing.diff.get.us" },
  { row: "landing.diff.size", lists: "landing.diff.size.lists", finders: "landing.diff.size.finders", us: "landing.diff.size.us" },
  { row: "landing.diff.level", lists: "landing.diff.level.lists", finders: "landing.diff.level.finders", us: "landing.diff.level.us" },
  { row: "landing.diff.pr", lists: "landing.diff.pr.lists", finders: "landing.diff.pr.finders", us: "landing.diff.pr.us" },
  { row: "landing.diff.progress", lists: "landing.diff.progress.lists", finders: "landing.diff.progress.finders", us: "landing.diff.progress.us" },
  { row: "landing.diff.cost", lists: "landing.diff.cost.lists", finders: "landing.diff.cost.finders", us: "landing.diff.cost.us" },
];

/**
 * The landing page, centred, over the backlog field: one real issue that has
 * been waiting (Hero.tsx), then
 * full-width bands, each showing the real thing: feature rows with live
 * miniatures, the repository finder, a comparison by kind of tool, why it's
 * free, and a closing that says why we made it. One column on narrow screens.
 * Its look is locked (tokens.module.css); themes never reach it.
 */
export function Landing() {
  useDocumentTitle(null);
  const t = useT();
  return (
    <div className={`${tokens.page} ${s.page}`}>
      <BacklogField />
      <Header />
      <main className={s.container}>
        <Hero />

        <section className={s.band} aria-labelledby="rows-title">
          <div className={s.head}>
            <h2 id="rows-title" className={s.title}>{t("landing.rows.title")}</h2>
          </div>
          <ol className={s.rows}>
            <Row title="landing.row.level.title" body="landing.row.level.body" cta="landing.row.level.cta" to="/docs/xp">
              <LevelWidget />
            </Row>
            <Row title="landing.row.desk.title" body="landing.row.desk.body" cta="landing.row.desk.cta" to="/download">
              <StepsWidget />
            </Row>
            <Row title="landing.row.trust.title" body="landing.row.trust.body" cta="landing.row.trust.cta" to="/security">
              <TrustWidget />
            </Row>
            <Row title="landing.row.live.title" body="landing.row.live.body" cta="landing.row.live.cta" href={external.live}>
              <ActivityWidget />
            </Row>
          </ol>
        </section>

        <section className={s.band} aria-labelledby="repo-title">
          <RepoFinder />
        </section>

        <section className={s.band} aria-labelledby="diff-title">
          <div className={s.head}>
            <p className={s.kicker}>{t("landing.diff.kicker")}</p>
            <h2 id="diff-title" className={s.title}>{t("landing.diff.title")}</h2>
            <p className={s.lede}>{t("landing.diff.body")}</p>
          </div>
          <div className={s.tableWrap}>
            <table className={s.compare}>
              <thead>
                <tr>
                  <td />
                  <th scope="col">{t("landing.diff.col.lists")}</th>
                  <th scope="col">{t("landing.diff.col.finders")}</th>
                  <th scope="col" className={s.us}>
                    <Logo size="inline" />
                  </th>
                </tr>
              </thead>
              <tbody>
                {COMPARE.map((r) => (
                  <tr key={r.row}>
                    <th scope="row">{t(r.row)}</th>
                    <td data-label={t("landing.diff.col.lists")}>{t(r.lists)}</td>
                    <td data-label={t("landing.diff.col.finders")}>{t(r.finders)}</td>
                    <td data-label="SkillIssues" className={s.us}>{t(r.us)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className={`${s.band} ${s.free}`} aria-labelledby="free-title">
          <div className={s.head}>
            <p className={s.kicker}>{t("landing.free.kicker")}</p>
            <h2 id="free-title" className={s.title}>{t("landing.free.title")}</h2>
            <p className={s.lede}>{t("landing.free.body1")}</p>
            <p className={s.lede}>{t("landing.free.body2")}</p>
            <p className={s.ctas}>
              <a href={external.sponsorsExplore} className={s.button}>{t("landing.free.cta")}</a>
              <a href={external.openCollective} className={s.ghost}>{t("landing.free.openCollective")}</a>
              <a href={external.thanksDev} className={s.ghost}>{t("landing.free.thanksDev")}</a>
            </p>
            <p className={s.aside}>{t("landing.free.aside")}</p>
          </div>
        </section>

        <section className={`${s.band} ${s.split}`} aria-label={t("landing.split.label")}>
          <div className={s.side}>
            <p className={s.kicker}>{t("landing.dev.kicker")}</p>
            <h3 className={s.rowTitle}>{t("landing.dev.title")}</h3>
            <p className={s.rowBody}>{t("landing.dev.body")}</p>
            <p className={s.links}>
              <Link to="/app" className={s.link}>{t("landing.dev.cta")}</Link>
            </p>
          </div>
          <div className={s.side}>
            <p className={s.kicker}>{t("landing.maint.kicker")}</p>
            <h3 className={s.rowTitle}>{t("landing.maint.title")}</h3>
            <p className={s.rowBody}>{t("landing.maint.body")}</p>
            <p className={s.links}>
              <Link to="/maintainers" className={s.link}>{t("landing.maint.cta")}</Link>
              <a href={external.maintainerFunding} className={s.link}>{t("landing.maint.funding")}</a>
            </p>
          </div>
        </section>

        <section className={`${s.band} ${s.closing}`} aria-labelledby="end-title">
          <div className={s.head}>
            <h2 id="end-title" className={s.title}>{t("landing.end.title")}</h2>
            <p className={s.lede}>{t("landing.end.body")}</p>
            <p className={s.made}>
              {t("landing.made.lead")} <Logo size="inline" />
            </p>
            <p className={s.lede}>{t("landing.made.title")}</p>
            <p className={s.ctas}>
              <a href="#find" className={s.button}>{t("landing.end.cta")}</a>
            </p>
          </div>
        </section>
      </main>
      <Footer />
    </div>
  );
}

/** One feature row: the tagline and its link on one side, the live miniature on the other. No box: hairlines between rows. */
function Row({
  title,
  body,
  cta,
  to,
  href,
  children,
}: {
  title: MessageKey;
  body: MessageKey;
  cta: MessageKey;
  to?: string;
  href?: string;
  children: ReactNode;
}) {
  const t = useT();
  return (
    <li className={s.row}>
      <div>
        <h3 className={s.rowTitle}>{t(title)}</h3>
        <p className={s.rowBody}>{t(body)}</p>
        <p className={s.links}>
          {to ? <Link to={to} className={s.link}>{t(cta)}</Link> : <a href={href} className={s.link}>{t(cta)}</a>}
        </p>
      </div>
      <div className={s.rowWidget}>{children}</div>
    </li>
  );
}
