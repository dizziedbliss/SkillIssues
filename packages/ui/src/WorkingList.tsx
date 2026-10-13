import type { ReactNode } from "react";
import type { ContributionDetail } from "@skillissues/api-client";
import { LanguageMark } from "./LanguageMark";
import { daysAgo } from "./format";

export interface WorkingListProps {
  items: ContributionDetail[];
  emptyMessage?: string;
  // Desktop's working card has VS Code/Show folder/Push & Submit PR
  // buttons that make no sense on web, which only deep-links to the
  // desktop app — so the per-card action buttons are the caller's, not
  // this component's, per the plan's "props/slots, not forks" rule.
  renderActions: (item: ContributionDetail) => ReactNode;
}

export function WorkingList({ items, emptyMessage, renderActions }: WorkingListProps) {
  if (!items.length) {
    return (
      <div className="empty-feed">
        <p>{emptyMessage ?? "Solve an issue to start tracking a local workspace."}</p>
      </div>
    );
  }

  return (
    <div className="issue-feed">
      {items.map((item) => (
        <article className="issue-card working-card" key={item.id}>
          <h2 className="issue-card-title">
            #{item.number} {item.title}
          </h2>
          <div className="issue-card-repo">{item.repository}</div>
          <div className="issue-card-bottom">
            <div className="pill-group-left">
              <span className="pill purple">Working</span>
              <span className="pill purple">Started {daysAgo(item.created_at)}</span>
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
      ))}
    </div>
  );
}
