import { useStats } from "../lib/useStats";
import { AnimatedNumber } from "./AnimatedNumber";
import { LiveBar } from "./LiveBar";
import { useI18n } from "../i18n";
import s from "./BacklogClock.module.css";

/**
 * The hero's clock: every open issue in the catalog as one big ticking
 * number, what arrived and closed today under it, and the closed-against-open
 * bar. All of it from GET /stats, polled every 10s.
 */
export function BacklogClock() {
  const { data } = useStats();
  const { t, locale } = useI18n();
  const format = new Intl.NumberFormat(locale);
  return (
    <div className={s.clock}>
      <p className={s.kicker}>
        <span className={s.dot} aria-hidden="true" />
        {t("landing.clock.kicker")}
      </p>
      <p className={s.count}>
        <AnimatedNumber value={data?.open_issues} placeholder="0" />
      </p>
      <p className={s.caption}>
        {data
          ? t("landing.clock.caption", { repositories: format.format(data.repositories_indexed) })
          : t("live.loading")}
      </p>
      {data && (
        <p className={s.today}>
          <span className={s.arrived}>{t("landing.clock.arrived", { n: format.format(data.arrived_today) })}</span>
          <span className={s.closed}>{t("landing.clock.closed", { n: format.format(data.closed_today) })}</span>
        </p>
      )}
      <LiveBar />
    </div>
  );
}
