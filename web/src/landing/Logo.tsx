import { LogoMark } from "./LogoMark";
import s from "./Logo.module.css";

type Size = "header" | "inline" | "footer";

/**
 * The logo, as the owner designed it: the mark (a ring and a red "!"), then
 * the wordmark, SkillIssues in Krona One in the ink, the "i" of Skill in the
 * signal red, underlined in the ink. Inline, in a sentence, it's the word alone.
 */
export function Logo({ size }: { size: Size }) {
  return (
    <span className={`${s.logo} ${s[size]}`}>
      {size !== "inline" && <LogoMark className={s.mark} />}
      <span className={s.word}>
        Sk<span className={s.i}>i</span>llIssues
      </span>
    </span>
  );
}
