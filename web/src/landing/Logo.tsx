import s from "./Logo.module.css";

type Size = "header" | "inline" | "footer";

/**
 * The wordmark, as the owner designed it: "Skill" in the ink and "Issues" in
 * the signal red, Krona One, underlined, widely tracked.
 */
export function Logo({ size }: { size: Size }) {
  return (
    <span className={`${s.logo} ${s[size]}`}>
      <span className={s.skill}>Skill</span>
      <span className={s.issues}>Issues</span>
    </span>
  );
}
