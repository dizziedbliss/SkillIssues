import { useEffect, useState } from "react";
import { useStats } from "../lib/useStats";
import { useReducedMotion } from "../lib/useReducedMotion";
import { useI18n } from "../i18n";
import { AnimatedNumber } from "./AnimatedNumber";
import { Logo } from "./Logo";
import s from "./LogoLive.module.css";

/** How long the logo shows, then the live count, before they swap again. */
const LOGO_MS = 12_000;
const LIVE_MS = 6_000;

/**
 * The header's logo, which every so often gives its place to the live count:
 * issues closed since SkillIssues started counting (green) against the open
 * ones still waiting (red), the hero's bar in small. Both share one slot, so
 * nothing moves around them. With reduced motion it stays the logo.
 */
export function LogoLive({ size }: { size: "header" }) {
  const { t, locale } = useI18n();
  const reduced = useReducedMotion();
  const { data } = useStats();
  const [live, setLive] = useState(false);

  useEffect(() => {
    if (reduced || !data) {
      setLive(false);
      return;
    }
    const id = setTimeout(() => setLive((v) => !v), live ? LIVE_MS : LOGO_MS);
    return () => clearTimeout(id);
  }, [live, reduced, data]);

  const closed = data?.closed_since;
  const open = data?.open_issues;
  const total = (closed ?? 0) + (open ?? 0);
  const share = total > 0 ? (closed ?? 0) / total : 0;
  const format = new Intl.NumberFormat(locale);

  return (
    <span className={s.slot} data-live={live ? "on" : "off"} data-ui="logo-live">
      <span className={s.logo}>
        <Logo size={size} />
      </span>
      {/* The link around it is named; this is a picture of the numbers. */}
      <span className={s.live} aria-hidden="true"
        title={data ? t("live.label", { closed: format.format(data.closed_since), open: format.format(data.open_issues) }) : undefined}>
        <span className={s.counts}>
          <span className={s.closedCount}><AnimatedNumber value={closed} placeholder="0" /></span>
          <span className={s.track}>
            <span className={s.closed} style={{ width: closed ? `max(2px, ${share * 100}%)` : 0 }} />
          </span>
          <span className={s.openCount}><AnimatedNumber value={open} /></span>
        </span>
        <span className={s.captions}>
          <span>{t("live.closedShort")}</span>
          <span>{t("live.openShort")}</span>
        </span>
      </span>
    </span>
  );
}
