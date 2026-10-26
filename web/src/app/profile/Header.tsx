import type { ReactNode } from "react";
import type { XPSummary } from "@skillissues/client";
import { useI18n } from "../../i18n";
import { SiteLink } from "../SiteLink";
import s from "./Header.module.css";

/**
 * The top of a profile, yours or someone's public one: who they are on the
 * left, their level large on the right with the bar to the next, and the
 * numbers behind it in one line. Everything comes from the XP ledger's
 * summary, so both pages show the same thing. own adds the streak nudge and
 * the link to how XP works.
 */
export function ProfileHeader({
  avatar,
  name,
  login,
  joined,
  summary,
  own,
  above,
  children,
}: {
  avatar: string;
  name: string;
  login: string;
  joined: string;
  summary: XPSummary | undefined;
  own: boolean;
  /** A quiet line over the name: the greeting on your own profile. */
  above?: ReactNode;
  /** Under the name: the bio and what you're looking for. */
  children?: ReactNode;
}) {
  const { t, locale } = useI18n();
  const n = new Intl.NumberFormat(locale);
  return (
    <header className={s.header} data-ui="profile-header">
      <div className={s.who}>
        {avatar ? <img className={s.avatar} src={avatar} alt="" width={96} height={96} /> : <span className={s.avatar} aria-hidden="true" />}
        <div className={s.identity}>
          {above && <p className={s.above}>{above}</p>}
          <h1 className={s.name}>{name}</h1>
          <p className={s.login}>
            <a href={`https://github.com/${login}`} target="_blank" rel="noreferrer">@{login}</a>
            {joined && <span>{t("profile.joined", { date: joined })}</span>}
          </p>
          {children}
        </div>
      </div>

      <section className={s.level} aria-labelledby="profile-level" data-ui="level" aria-busy={!summary}>
        {summary && <Level summary={summary} own={own} />}
      </section>

      {summary && (
        <ul className={s.numbers}>
          <li data-ui="stat">{t("profile.merges", { count: summary.merges, n: n.format(summary.merges) })}</li>
          <li data-ui="stat">{t("profile.languagesCount", { count: summary.languages.length })}</li>
          <li data-ui="stat" className={summary.streak.current > 0 ? s.lit : undefined}>
            {summary.streak.current > 0 ? t("xp.streak", { weeks: summary.streak.current }) : t("xp.streakNone")}
          </li>
          {summary.streak.best > 1 && <li data-ui="stat">{t("xp.streakBest", { weeks: summary.streak.best })}</li>}
        </ul>
      )}
    </header>
  );
}

function Level({ summary, own }: { summary: XPSummary; own: boolean }) {
  const { t, locale } = useI18n();
  const n = new Intl.NumberFormat(locale);
  const { level, total, streak } = summary;
  const span = Math.max(1, level.next - level.start);
  const into = Math.min(span, Math.max(0, total - level.start));
  const toNext = t("xp.toNext", { left: n.format(level.next - total), level: level.level + 1 });
  return (
    <>
      <p className={s.kicker}>{t("xp.title")}</p>
      <h2 id="profile-level" className={s.levelNumber}>{t("xp.level", { level: level.level })}</h2>
      <div className={s.bar} role="progressbar" aria-valuemin={level.start} aria-valuemax={level.next} aria-valuenow={total} aria-label={toNext}>
        <span style={{ width: `${(into / span) * 100}%` }} />
      </div>
      <p className={s.toNext}>
        <span>{t("xp.total", { xp: n.format(total) })}</span>
        <span>{toNext}</span>
      </p>
      {own && streak.current > 0 && !streak.this_week && <p className={s.nudge}>{t("xp.streakKeep")}</p>}
      {own && <SiteLink to="/docs/xp" className={s.how}>{t("xp.how")}</SiteLink>}
    </>
  );
}
