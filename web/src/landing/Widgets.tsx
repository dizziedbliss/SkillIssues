import { useStats } from "../lib/useStats";
import { useI18n, useT, type MessageKey } from "../i18n";
import { ago } from "../lib/time";
import s from "./Widgets.module.css";

/*
 * The small live pieces inside the landing page's feature cards. Each shows
 * the real thing in miniature: how levels read (an example profile), the
 * desktop app's steps, what sign-in can and can't do, and the catalog's
 * latest activity from GET /stats.
 */

/** Where level n starts, as /docs/xp states it: each level takes 100 XP more than the one before (level 2 at 100, 5 at 1,000, 10 at 4,500). */
const levelStart = (n: number) => 50 * n * (n - 1);
const levelOf = (xp: number) => {
  let n = 1;
  while (levelStart(n + 1) <= xp) n++;
  return n;
};

/** An example profile, labelled as one: the numbers only illustrate how levels, language levels, badges and streaks read. */
const EXAMPLE = {
  xp: 2540,
  languages: [
    { name: "TypeScript", xp: 1240 },
    { name: "Go", xp: 420 },
    { name: "Python", xp: 180 },
  ],
  badges: ["badge.first_merge", "badge.backlog_hero", "badge.streak_4"] as MessageKey[],
  streakWeeks: 4,
};

export function LevelWidget() {
  const { t, locale } = useI18n();
  const format = new Intl.NumberFormat(locale);
  const level = levelOf(EXAMPLE.xp);
  const from = levelStart(level);
  const to = levelStart(level + 1);
  const share = (EXAMPLE.xp - from) / (to - from);
  return (
    <figure className={s.level}>
      <figcaption className={s.example}>{t("landing.levelw.example")}</figcaption>
      <div className={s.levelHead}>
        <span className={s.levelNumber}>{t("xp.level", { level })}</span>
        <span className={s.levelXp}>{t("landing.levelw.xp", { n: format.format(EXAMPLE.xp) })}</span>
      </div>
      <div className={s.progress} role="img" aria-label={t("landing.levelw.toNext", { n: format.format(to - EXAMPLE.xp), level: level + 1 })}>
        <span style={{ width: `${share * 100}%` }} />
      </div>
      <p className={s.toNext}>{t("landing.levelw.toNext", { n: format.format(to - EXAMPLE.xp), level: level + 1 })}</p>
      <ul className={s.langs}>
        {EXAMPLE.languages.map((l) => (
          <li key={l.name}>
            <span>{l.name}</span>
            <span className={s.plus}>{t("xp.level", { level: levelOf(l.xp) })}</span>
          </li>
        ))}
      </ul>
      <ul className={s.badges}>
        {EXAMPLE.badges.map((b) => (
          <li key={b}>{t(b)}</li>
        ))}
        <li className={s.streak}>{t("xp.streak", { weeks: EXAMPLE.streakWeeks })}</li>
      </ul>
    </figure>
  );
}

const STEPS: MessageKey[] = [
  "landing.stepw.fork",
  "landing.stepw.branch",
  "landing.stepw.commit",
  "landing.stepw.push",
  "landing.stepw.pr",
];

/** The desktop app's steps, lit one after another (still under reduced motion). */
export function StepsWidget() {
  const t = useT();
  return (
    <ol className={s.steps}>
      {STEPS.map((k) => (
        <li key={k} className={s.step}>{t(k)}</li>
      ))}
    </ol>
  );
}

const TRUST: { label: MessageKey; value: MessageKey }[] = [
  { label: "landing.trustw.scopes", value: "landing.trustw.none" },
  { label: "landing.trustw.token", value: "landing.trustw.never" },
  { label: "landing.trustw.repos", value: "landing.trustw.none" },
];

/** What signing in gives SkillIssues, as /security describes it. */
export function TrustWidget() {
  const t = useT();
  return (
    <dl className={s.trust}>
      {TRUST.map((r) => (
        <div key={r.label} className={s.trustRow}>
          <dt>{t(r.label)}</dt>
          <dd>
            <span className={s.ok} aria-hidden="true" />
            {t(r.value)}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/** How many of the latest activity rows to show. */
const ACTIVITY_SHOWN = 4;

/** The catalog's latest changes, as GET /stats reports them. */
export function ActivityWidget() {
  const { data, isError } = useStats();
  const { t, locale } = useI18n();
  const rows = data?.activity.slice(0, ACTIVITY_SHOWN) ?? [];
  if (isError) return <p className={s.empty}>{t("landing.livew.error")}</p>;
  if (!data) return <p className={s.empty}>{t("live.loading")}</p>;
  if (rows.length === 0) return <p className={s.empty}>{t("landing.livew.empty")}</p>;
  return (
    <ul className={s.activity} aria-label={t("landing.livew.label")}>
      {rows.map((a) => (
        <li key={`${a.at}-${a.repository}`} className={s.activityRow}>
          <span className={s.repo}>{a.repository}</span>
          {a.arrived > 0 && <span className={s.arrived}>+{a.arrived}</span>}
          {a.closed > 0 && <span className={s.closed}>−{a.closed}</span>}
          <span className={s.when}>{ago(a.at, locale)}</span>
        </li>
      ))}
    </ul>
  );
}
