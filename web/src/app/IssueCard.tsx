import { Link } from "react-router";
import type { Issue } from "@skillissues/client";
import { useI18n } from "../i18n";
import { ago } from "../lib/time";
import { CommentIcon } from "./icons";
import { SaveButton } from "./SaveButton";
import s from "./IssueCard.module.css";

/** Labels shown on a card before "+n". */
const LABELS = 4;

/**
 * One issue in a list, as designed: the title and number, the repository,
 * then its age, comments and labels on the left, the repository's languages,
 * View Issue and the bookmark on the right.
 */
export function IssueCard({ issue }: { issue: Issue }) {
  const { t, locale } = useI18n();
  const extra = issue.labels.length - LABELS;
  const updated = ago(issue.updated_at, locale);

  return (
    <article className={s.card} aria-labelledby={`issue-${issue.id}`}>
      <h2 id={`issue-${issue.id}`} className={s.title}>
        <Link to={`/app/issues/${issue.id}`}>
          {issue.title} <span className={s.number}>#{issue.number}</span>
        </Link>
      </h2>
      <p className={s.repo}>{issue.repository}</p>
      <div className={s.row}>
        <ul className={s.chips}>
          {issue.state === "CLOSED" && <li className={s.closed}>{t("issue.closedChip")}</li>}
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
            <ul className={s.tech}>
              {issue.tech.map((x) => (
                <li key={x} className={s.techChip}>{x}</li>
              ))}
            </ul>
          )}
          <Link to={`/app/issues/${issue.id}`} className={s.view} tabIndex={-1}>
            {t("issue.view")}
          </Link>
          <SaveButton issue={issue} />
        </div>
      </div>
    </article>
  );
}
