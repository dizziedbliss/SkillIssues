import { useStats } from "../lib/useStats";
import { AnimatedNumber } from "./AnimatedNumber";
import s from "./LiveBar.module.css";

const format = new Intl.NumberFormat("en-US");

/**
 * The hero's bar: issues closed on GitHub since SkillIssues started counting
 * (green) against the open issues still waiting (red), live.
 */
export function LiveBar() {
  const { data } = useStats();
  const closed = data?.closed_since;
  const open = data?.open_issues;
  const total = (closed ?? 0) + (open ?? 0);
  const share = total > 0 ? (closed ?? 0) / total : 0;
  const label = data
    ? `${format.format(data.closed_since)} issues closed since we started counting; ${format.format(data.open_issues)} open issues.`
    : "Live count of closed and open issues, loading.";

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
        <span>issues closed</span>
        <span>open issues and counting</span>
      </figcaption>
    </figure>
  );
}
