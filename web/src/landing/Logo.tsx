import { useT } from "../i18n";
import s from "./Logo.module.css";

type Size = "header" | "inline" | "footer";

/**
 * The wordmark: Krona One at 9% tracking over its line in JetBrains Mono,
 * the two in a 24:8 ratio. Inline (in a sentence) it drops the line.
 */
export function Logo({ size }: { size: Size }) {
  const t = useT();
  return (
    <span className={`${s.logo} ${s[size]}`}>
      <span className={s.word}>SkillIssues</span>
      {size !== "inline" && <span className={s.line}>{t("logo.line")}</span>}
    </span>
  );
}
