import type { ReactNode } from "react";
import type { ContributionDetail } from "@skillissues/api-client";
import { LanguageMark } from "./LanguageMark";
import { daysAgo, xpForDifficulty } from "./format";

export interface SubmissionListProps {
  items: ContributionDetail[];
  emptyMessage?: string;
  renderActions: (item: ContributionDetail) => ReactNode;
}

export function SubmissionList({ items, emptyMessage, renderActions }: SubmissionListProps) {
  if (!items.length) {
    return (
      <div className="empty-feed">
        <p>{emptyMessage ?? "Submit a pull request to see your contribution status here."}</p>
      </div>
    );
  }

  return (
    <div className="issue-feed">
      {items.map((item) => {
        const isMerged = item.state === "PR_MERGED";
        const isClosed = item.state === "PR_CLOSED";
        const xpEarned = xpForDifficulty(item.difficulty);

        const statusText = isMerged
          ? `Accepted (+${xpEarned} XP) 🎉`
          : isClosed
            ? "Rejected ✖"
            : "Review required ⏳";
        const statusClass = isMerged ? "accepted" : isClosed ? "rejected" : "review";

        return (
          <article className="issue-card submission-card" key={item.id}>
            <h2 className="issue-card-title">
              #{item.number} {item.title}
            </h2>
            <div className="issue-card-repo">{item.repository}</div>
            <div className="issue-card-bottom">
              <div className="pill-group-left">
                <span className={`pill status-pill ${statusClass}`}>{statusText}</span>
                <span className="pill purple">Started {daysAgo(item.created_at)}</span>
                {item.submitted_at && (
                  <span className="pill purple">Submitted {daysAgo(item.submitted_at)}</span>
                )}
              </div>
              <div className="pill-group-middle">
                <span className="pill light">
                  <LanguageMark name={item.language || "Code"} />
                  {item.language || "Code"}
                </span>
              </div>
              <div className="pill-group-right">{renderActions(item)}</div>
            </div>
          </article>
        );
      })}
    </div>
  );
}
