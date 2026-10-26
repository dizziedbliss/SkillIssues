import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Issue } from "@skillissues/client";
import { useI18n } from "../i18n";
import { ago } from "../lib/time";
import { CommentIcon } from "./icons";
import { SaveButton } from "./SaveButton";
import { scopeURL } from "./explore/query";
import s from "./IssueCard.module.css";

/** Labels shown on a card before "+n". */
const LABELS = 4;

/**
 * One issue in a list: the title and number, the repository, then its age,
 * comments and labels on the left, the repository's languages and the bookmark
 * on the right. The whole card opens the issue (the title's link covers it);
 * the repository's name opens its issues.
 */
export function IssueCard({ issue, index }: { issue: Issue; index?: number }) {
  const { t, locale } = useI18n();
  const extra = issue.labels.length - LABELS;
  const updated = ago(issue.updated_at, locale);

  return (
    <article
      className={s.card}
      data-ui-state={issue.state === "CLOSED" ? "closed" : "open"}
      aria-labelledby={`issue-${issue.id}`}
      data-ui="card"
      data-ui-index={index}
      style={index === undefined ? undefined : ({ "--ui-index": index } as CSSProperties)}
    >
      <h2 id={`issue-${issue.id}`} className={s.title} data-ui="card-title">
        <Link to={`/app/issues/${issue.id}`}>
          {issue.title} <span className={s.number}>#{issue.number}</span>
        </Link>
      </h2>
      <p className={s.repo} data-ui="card-repo">
        <Link to={scopeURL({ repo: issue.repository })} className={s.repoLink} title={t("issue.moreRepo", { repo: issue.repository })}>
          {issue.repository}
        </Link>
      </p>
      {issue.for_you && issue.for_you.length > 0 && (
        <p className={s.why} data-ui="card-why">
          {t("foryou.label")} {issue.for_you.map((r) => t(`foryou.reason.${r}`)).join(", ")}
        </p>
      )}
      <div className={s.row}>
        <ul className={s.chips} data-ui="chips">
          {issue.state === "CLOSED" && <li className={s.closed}>{t("issue.closedChip")}</li>}
          {(issue.assignees ?? 0) > 0 && <li className={s.taken} title={t("issue.assignedHelp")}>{t("issue.assigned")}</li>}
          {issue.claimed && <li className={s.taken} title={t("issue.claimedHelp")}>{t("issue.claimed")}</li>}
          {issue.repository_health?.welcoming && (
            <li className={s.welcoming} title={t("health.welcomingHelp", { outside: issue.repository_health.outside, sampled: issue.repository_health.sampled })}>
              {t("health.welcoming")}
            </li>
          )}
          {issue.repository_health?.slow && (
            <li className={s.flag} title={t("health.slowHelp", { days: Math.round(issue.repository_health.median_merge_hours / 24) })}>
              {t("health.slow")}
            </li>
          )}
          {updated && (
            <li className={s.chip} title={t("issue.updated", { ago: updated })}>
              {updated}
            </li>
          )}
          <li className={s.chip} aria-label={t("issue.comments", { count: issue.comments })}>
            <CommentIcon />
            {issue.comments}
          </li>
          {issue.labels.slice(0, LABELS).map((l) => (
            <li key={l} className={s.chip}>{l}</li>
          ))}
          {extra > 0 && (
            <li className={s.chip} title={issue.labels.slice(LABELS).join(", ")}>
              {t("issue.moreLabels", { count: extra })}
            </li>
          )}
        </ul>
        <div className={s.end}>
          {issue.tech.length > 0 && (
            <p className={s.tech} data-ui="tech">{issue.tech.join(" · ")}</p>
          )}
          <SaveButton issue={issue} />
        </div>
      </div>
    </article>
  );
}
