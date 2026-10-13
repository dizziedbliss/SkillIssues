import type { ReactNode } from "react";
import type { Issue } from "@skillissues/api-client";
import { LanguageMark } from "./LanguageMark";
import { daysAgo } from "./format";

export interface IssueCardProps {
  issue: Issue;
  saved: boolean;
  onSelect: () => void;
  onToggleSave: () => void;
  // Icons are injected rather than imported from a specific icon library —
  // web uses lucide-react, desktop had none at all — so this package
  // doesn't force a dependency neither client is guaranteed to want.
  bookmarkIcon?: ReactNode;
  commentIcon?: ReactNode;
}

export function IssueCard({
  issue,
  saved,
  onSelect,
  onToggleSave,
  bookmarkIcon,
  commentIcon,
}: IssueCardProps) {
  const author = issue.author || (issue.repository ? issue.repository.split("/")[0] : "open-source");
  const extraTechs = (issue.technologies || [])
    .filter((tech) => !issue.language || tech.toLowerCase() !== issue.language.toLowerCase())
    .slice(0, 2);

  return (
    <article
      className="issue-card"
      onClick={(e) => {
        if (!(e.target as HTMLElement).closest("button")) onSelect();
      }}
    >
      <h2 className="issue-card-title">
        #{issue.number} {issue.title}
      </h2>
      <div className="issue-card-repo">{issue.repository}</div>
      <div className="issue-card-bottom">
        <div className="pill-group-left">
          {issue.match_category && <span className="pill match">Matched in {issue.match_category}</span>}
          {issue.state === "CLOSED" && <span className="pill status-pill rejected">Closed</span>}
          {issue.lifecycle === "working" && <span className="pill purple">Working</span>}
          {issue.lifecycle === "submitted" && <span className="pill status-pill review">Submitted</span>}
          {issue.lifecycle === "merged" && <span className="pill status-pill accepted">Merged</span>}
          <span className="pill purple">{author}</span>
          <span className="pill purple">{daysAgo(issue.updated_at)}</span>
          <span className="pill purple">
            {commentIcon} {issue.comments || 0}
          </span>
        </div>
        <div className="pill-group-middle">
          <span className="pill light">
            <LanguageMark name={issue.language || "Open source"} />
            {issue.language || "Open source"}
          </span>
          {extraTechs.map((tech) => (
            <span className="pill light" key={tech}>
              {tech}
            </span>
          ))}
        </div>
        <div className="pill-group-right">
          <button type="button" className="view-issue-btn" onClick={onSelect}>
            View Issue
          </button>
          <button
            type="button"
            className={`save-icon-btn ${saved ? "saved" : ""}`}
            title={saved ? "Remove saved issue" : "Save issue"}
            onClick={(e) => {
              e.stopPropagation();
              onToggleSave();
            }}
          >
            {bookmarkIcon}
          </button>
        </div>
      </div>
    </article>
  );
}
