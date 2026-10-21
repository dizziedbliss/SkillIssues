import s from "./Logo.module.css";

type Size = "header" | "inline" | "footer";

/**
 * The wordmark, as the owner designed it: SkillIssues in Krona One in the
 * ink, the "i" of Skill in the signal red, underlined in the ink. The mark
 * (the ring and "!") is the favicon and the app icon only.
 */
export function Logo({ size }: { size: Size }) {
  return (
    <span className={`${s.logo} ${s[size]}`}>
      <span className={s.word}>
        Sk<span className={s.i}>i</span>llIssues
      </span>
    </span>
  );
}
