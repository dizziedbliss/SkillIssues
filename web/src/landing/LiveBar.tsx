import { useStats } from "../lib/useStats";
import { AnimatedNumber } from "./AnimatedNumber";
import { useI18n } from "../i18n";
import s from "./LiveBar.module.css";

/**
 * The hero's bar: issues closed on GitHub since SkillIssues started counting
 * (green) against the open issues still waiting (red), live.
 */
export function LiveBar() {
  const { data } = useStats();
  const { t, locale } = useI18n();
  const format = new Intl.NumberFormat(locale);
  const closed = data?.closed_since;
  const open = data?.open_issues;
  const total = (closed ?? 0) + (open ?? 0);
  const share = total > 0 ? (closed ?? 0) / total : 0;
  const label = data
    ? t("live.label", { closed: format.format(data.closed_since), open: format.format(data.open_issues) })
    : t("live.loading");

  return (
    <figure className={s.bar} aria-label={label}>
      <span className={s.closedCount}>
        <AnimatedNumber value={closed} placeholder="0" />
      </span>
      <span className={s.track} aria-hidden="true">
        {/* A closed issue or two still shows: the sliver is the point. */}
        <span className={s.closed} style={{ width: closed ? `max(3px, ${share * 100}%)` : 0 }} />
      </span>
      <span className={s.openCount}>
        <AnimatedNumber value={open} />
      </span>
      <figcaption className={s.captions} aria-hidden="true">
        <span>{t("live.closed")}</span>
        <span>{t("live.open")}</span>
      </figcaption>
    </figure>
  );
}
