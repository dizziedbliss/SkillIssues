import { SiteLink } from "./SiteLink";
import type { Issue } from "@skillissues/client";
import { useI18n, type MessageKey } from "../i18n";
import type { Live } from "../lib/issueBody";
import type { LinkedPulls } from "../lib/linkedPulls";
import { ago } from "../lib/time";
import s from "./BeginnerCheck.module.css";

/*
 * "Is this a good first issue?", from the issue as GitHub has it now. Each
 * line passes, warns, or says it couldn't tell; nothing is guessed. The
 * thresholds are the ones the guide explains (/docs/good-first-issues): keep
 * the two in step.
 */

/** A description shorter than this (template comments left out) says too little to start from. */
export const DESCRIBED_CHARS = 150;
/** No activity for longer than this, and the maintainers may have moved on. */
export const QUIET_DAYS = 180;
/** More comments than this, and there is a discussion to read first. */
export const BUSY_COMMENTS = 10;
/** Labels maintainers use for a place to start. */
const STARTER = /good.?first|beginner|easy|starter|first.?timers|newcomer|low.?hanging/i;

type State = "pass" | "warn" | "unknown";

export function BeginnerCheck({
  issue,
  body,
  live,
  pulls,
  claimedByOther,
}: {
  issue: Issue;
  body: string | null;
  live: Live | null;
  pulls: { data?: LinkedPulls; error: Error | null };
  /** Someone else on SkillIssues claimed it; null while unknown. */
  claimedByOther: boolean | null;
}) {
  const { t, locale } = useI18n();
  const labels = live?.labels ?? issue.labels;
  const updated = live?.updatedAt ?? issue.updated_at;
  const comments = live?.comments ?? issue.comments;
  const quietDays = updated ? (Date.now() - Date.parse(updated)) / 86_400_000 : null;
  const text = body === null ? null : body.replace(/<!--[\s\S]*?-->/g, "").trim();

  const checks: { state: State; text: string }[] = [
    labels.some((l) => STARTER.test(l)) || issue.difficulty === "BEGINNER"
      ? { state: "pass", text: t("check.labelled") }
      : { state: "warn", text: t("check.notLabelled") },
    text === null
      ? { state: "unknown", text: t("check.descriptionUnknown") }
      : text.length >= DESCRIBED_CHARS
        ? { state: "pass", text: t("check.described") }
        : { state: "warn", text: t("check.notDescribed") },
    live === null
      ? { state: "unknown", text: t("check.assigneeUnknown") }
      : live.assignees === 0
        ? { state: "pass", text: t("check.unassigned") }
        : { state: "warn", text: t("check.assigned") },
    claimedByOther === null
      ? { state: "unknown", text: t("check.claimUnknown") }
      : claimedByOther
        ? { state: "warn", text: t("check.claimed") }
        : { state: "pass", text: t("check.unclaimed") },
    pulls.error
      ? { state: "unknown", text: t("check.pullsUnknown") }
      : !pulls.data
        ? { state: "unknown", text: t("check.pullsLoading") }
        : pulls.data.open === 0
          ? { state: "pass", text: t("check.noPull") }
          : { state: "warn", text: t("check.openPull", { count: pulls.data.open }) },
    quietDays === null
      ? { state: "unknown", text: t("check.activityUnknown") }
      : quietDays <= QUIET_DAYS
        ? { state: "pass", text: t("check.active", { ago: ago(updated, locale) }) }
        : { state: "warn", text: t("check.quiet", { ago: ago(updated, locale) }) },
    comments <= BUSY_COMMENTS
      ? { state: "pass", text: t("check.calm") }
      : { state: "warn", text: t("check.busy", { count: comments }) },
  ];
  if (live?.closed) checks.unshift({ state: "warn", text: t("check.closed") });

  const warnings = checks.filter((c) => c.state === "warn").length;
  const verdict: MessageKey = live?.closed ? "check.verdictClosed" : warnings === 0 ? "check.verdictGood" : "check.verdictCheck";

  return (
    <section className={s.check} aria-labelledby="check-title" data-ui="beginner-check">
      <h2 id="check-title" className={s.title}>{t("check.title")}</h2>
      <p className={`${s.verdict} ${warnings === 0 && !live?.closed ? s.good : ""}`}>{t(verdict)}</p>
      <ul className={s.list}>
        {checks.map((c) => (
          <li key={c.text} className={s[c.state]}>
            <span className={s.mark} aria-hidden="true">{c.state === "pass" ? "+" : c.state === "warn" ? "!" : "?"}</span>
            <span>
              <span className="visually-hidden">{t(`check.state.${c.state}` as MessageKey)}: </span>
              {c.text}
            </span>
          </li>
        ))}
      </ul>
      <SiteLink to="/docs/good-first-issues" className={s.guide}>{t("check.guide")}</SiteLink>
    </section>
  );
}
