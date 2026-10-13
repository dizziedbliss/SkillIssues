import type { Issue } from "@skillissues/api-client";
import { IssueCard, type IssueCardProps } from "./IssueCard";
import { SkeletonFeed } from "./Skeleton";

export interface IssueListProps {
  issues: Issue[];
  loading?: boolean;
  emptyMessage?: string;
  isSaved: (issue: Issue) => boolean;
  onSelect: (issue: Issue) => void;
  onToggleSave: (issue: Issue) => void;
  bookmarkIcon?: IssueCardProps["bookmarkIcon"];
  commentIcon?: IssueCardProps["commentIcon"];
}

export function IssueList({
  issues,
  loading,
  emptyMessage = "Nothing found here yet.",
  isSaved,
  onSelect,
  onToggleSave,
  bookmarkIcon,
  commentIcon,
}: IssueListProps) {
  if (loading) return <SkeletonFeed />;

  if (!issues.length) {
    return (
      <div className="empty-feed">
        <p>{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div className="issue-feed">
      {issues.map((issue) => (
        <IssueCard
          key={issue.id}
          issue={issue}
          saved={isSaved(issue)}
          onSelect={() => onSelect(issue)}
          onToggleSave={() => onToggleSave(issue)}
          bookmarkIcon={bookmarkIcon}
          commentIcon={commentIcon}
        />
      ))}
    </div>
  );
}
