import { useMemo, useState } from "react";
import type { ReactElement } from "react";
import type { ContributionDetail, Issue } from "@skillissues/api-client";
import {
  AttachPullRequestDialog,
  DifficultyTabs,
  IssueCard,
  IssueList,
  SectionTabs,
  SubmissionList,
  WorkingList,
} from "@skillissues/ui";
import { useApp } from "../state/AppState";
import { apiClient, desktop } from "../lib/bridge";
import {
  AllIcon,
  BookmarkIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CodeIcon,
  CommentIcon,
  DocIcon,
  ExternalLinkIcon,
  FolderIcon,
  UserIcon,
} from "../lib/icons";

interface SearchGroup {
  key: "repository" | "language" | "issue" | "author";
  title: string;
  icon: () => ReactElement;
  items: Issue[];
}

function groupByCategory(issues: Issue[]): SearchGroup[] {
  const repository: Issue[] = [];
  const language: Issue[] = [];
  const issue: Issue[] = [];
  const author: Issue[] = [];
  for (const item of issues) {
    const category = (item.match_category || "").toLowerCase();
    if (category === "repository") repository.push(item);
    else if (category === "language") language.push(item);
    else if (category === "author") author.push(item);
    else issue.push(item);
  }
  const groups: SearchGroup[] = [];
  if (repository.length) groups.push({ key: "repository", title: "Repositories", icon: FolderIcon, items: repository });
  if (language.length) groups.push({ key: "language", title: "Languages & Frameworks", icon: CodeIcon, items: language });
  if (issue.length) groups.push({ key: "issue", title: "Issues", icon: DocIcon, items: issue });
  if (author.length) groups.push({ key: "author", title: "Authors & Organizations", icon: UserIcon, items: author });
  return groups;
}

function WorkingActions({ item }: { item: ContributionDetail }) {
  const app = useApp();
  const [attachOpen, setAttachOpen] = useState(false);

  const openVSCode = async () => {
    try {
      const resolved = await desktop.resolveRepositoryWorkspace(item.repository);
      const result = await desktop.openInVSCode(resolved.path);
      app.toast.show(result.message || "Opened workspace in VS Code.", result.confirmed ? "success" : "info");
    } catch (err) {
      app.toast.show(err instanceof Error ? err.message : "Couldn't open VS Code.", "error");
    }
  };

  const openFolder = async () => {
    try {
      const resolved = await desktop.resolveRepositoryWorkspace(item.repository);
      const result = await desktop.openWorkspace(resolved.path);
      app.toast.show(result.confirmed ? "Opened workspace folder." : "Couldn't confirm the folder opened.", result.confirmed ? "success" : "info");
    } catch (err) {
      app.toast.show(err instanceof Error ? err.message : "Couldn't open the workspace folder.", "error");
    }
  };

  return (
    <>
      <a className="view-issue-btn" href={item.issue_url} target="_blank" rel="noreferrer">
        View issue ↗
      </a>
      <button type="button" className="view-issue-btn" onClick={() => void openVSCode()}>
        VS Code ↗
      </button>
      <button type="button" className="view-issue-btn" onClick={() => void openFolder()}>
        Show folder
      </button>
      <button type="button" className="view-issue-btn" onClick={() => void app.startPushForWorking(item)}>
        Push &amp; Submit PR
      </button>
      <button type="button" className="view-issue-btn" onClick={() => setAttachOpen(true)}>
        Link PR
      </button>
      <AttachPullRequestDialog
        open={attachOpen}
        repository={item.repository}
        onClose={() => setAttachOpen(false)}
        onAttach={async (prUrl, prNumber) => {
          await apiClient.attachPullRequest(item.id, prUrl, prNumber);
          app.toast.show("Pull request linked. It now appears under Submissions.", "success");
          void app.working.reload();
          void app.submissions.reload();
        }}
      />
    </>
  );
}

function SubmissionActions({ item }: { item: ContributionDetail }) {
  const app = useApp();

  const verify = async () => {
    try {
      const result = await apiClient.verifyContribution(item.id);
      app.toast.show(result.message, result.verified ? "success" : "error");
      void app.submissions.reload();
      void app.refreshProfile();
    } catch (err) {
      app.toast.show(err instanceof Error ? err.message : "Couldn't check the merge status.", "error");
    }
  };

  return (
    <>
      {item.state !== "PR_MERGED" && (
        <button type="button" className="view-issue-btn" onClick={() => void verify()}>
          Check merge status ↻
        </button>
      )}
      {item.pr_url && (
        <a className="view-issue-btn" href={item.pr_url} target="_blank" rel="noreferrer">
          View submission ↗
        </a>
      )}
      <a className="save-icon-btn" href={item.issue_url} target="_blank" rel="noreferrer" title="View issue on GitHub">
        <ExternalLinkIcon />
      </a>
    </>
  );
}

export function CatalogFeed() {
  const app = useApp();
  const { activeSection } = app;

  const searchActive = activeSection === "all" && Boolean(app.searchQuery.trim());
  const groups = useMemo(() => (searchActive ? groupByCategory(app.catalog.issues) : []), [searchActive, app.catalog.issues]);

  const subtitle =
    activeSection === "working"
      ? "Repositories you are actively solving"
      : activeSection === "submissions"
        ? "Pull requests under review and completed"
        : activeSection === "saved"
          ? "Challenges to do later"
          : app.catalog.stale
            ? "Personalized for your current level and preferences (showing cached results — offline or refresh pending)"
            : "Personalized for your current level and preferences";

  const pageLimit = app.searchQuery.trim() ? 50 : 20;

  return (
    <div className="feed-column">
      <SectionTabs
        items={[
          { key: "all", label: "SkillIssues", count: app.catalog.issues.length },
          { key: "saved", label: "Saved", count: app.saved.items.length },
          { key: "working", label: "Working", count: app.working.items.length },
          { key: "submissions", label: "Submissions", count: app.submissions.items.length },
        ]}
        active={activeSection}
        onChange={(key) => app.setActiveSection(key as typeof activeSection)}
      />

      {activeSection === "all" && <DifficultyTabs value={app.difficulty} onChange={app.setDifficulty} />}

      <div className="feed-heading">
        <h1>SkillIssues</h1>
        <span>{subtitle}</span>
      </div>

      <div className="issues">
        {activeSection === "working" && (
          <WorkingList items={app.working.items} renderActions={(item) => <WorkingActions item={item} />} />
        )}
        {activeSection === "submissions" && (
          <SubmissionList items={app.submissions.items} renderActions={(item) => <SubmissionActions item={item} />} />
        )}
        {activeSection === "saved" && (
          <IssueList
            issues={app.saved.items}
            loading={app.saved.loading && app.saved.items.length === 0}
            emptyMessage="Nothing saved yet — bookmark a challenge to do it later."
            isSaved={() => true}
            onSelect={app.openIssue}
            onToggleSave={app.toggleSave}
            bookmarkIcon={<BookmarkIcon saved />}
            commentIcon={<CommentIcon />}
          />
        )}
        {activeSection === "all" && !searchActive && (
          <IssueList
            issues={app.catalog.issues}
            loading={app.catalog.loading && app.catalog.issues.length === 0}
            emptyMessage="Nothing here yet."
            isSaved={app.isSaved}
            onSelect={app.openIssue}
            onToggleSave={app.toggleSave}
            bookmarkIcon={<BookmarkIcon saved={false} />}
            commentIcon={<CommentIcon />}
          />
        )}
        {activeSection === "all" && searchActive && (
          <SearchResults groups={groups} query={app.searchQuery} empty={app.catalog.issues.length === 0} />
        )}
      </div>

      {activeSection === "all" && (
        <div className="pagination">
          <button type="button" className="pagination-btn" disabled={app.page <= 1} onClick={() => app.setPage(app.page - 1)}>
            ← Previous
          </button>
          <span className="pagination-info">Page {app.page}</span>
          <button
            type="button"
            className="pagination-btn"
            disabled={app.catalog.issues.length < pageLimit}
            onClick={() => app.setPage(app.page + 1)}
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}

function SearchResults({ groups, query, empty }: { groups: SearchGroup[]; query: string; empty: boolean }) {
  const app = useApp();

  if (empty) {
    return (
      <div className="empty-feed">
        <p>No issues found matching &quot;{query}&quot;. Try searching for another language, repository, or topic.</p>
      </div>
    );
  }

  const activeGroups = app.searchCategory === "ALL" ? groups : groups.filter((g) => g.key === app.searchCategory);
  const totalCount = groups.reduce((sum, g) => sum + g.items.length, 0);

  return (
    <div className="search-results-container">
      {groups.length > 0 && (
        <div className="search-category-tabs">
          <button
            type="button"
            className={`search-category-tab ${app.searchCategory === "ALL" ? "active" : ""}`}
            onClick={() => app.setSearchCategory("ALL")}
          >
            <AllIcon />
            <span>All</span>
            <span className="search-category-badge">{totalCount}</span>
          </button>
          {groups.map((group) => (
            <button
              key={group.key}
              type="button"
              className={`search-category-tab ${app.searchCategory === group.key ? "active" : ""}`}
              onClick={() => app.setSearchCategory(group.key)}
            >
              <group.icon />
              <span>{group.title}</span>
              <span className="search-category-badge">{group.items.length}</span>
            </button>
          ))}
        </div>
      )}

      {activeGroups.map((group) => {
        const isExpanded = Boolean(app.expandedCategories[group.key]) || app.searchCategory !== "ALL";
        const visibleItems = isExpanded ? group.items : group.items.slice(0, 3);
        const hasMore = group.items.length > 3;

        return (
          <div className="search-category-section" key={group.key}>
            <div className="search-category-header">
              <div className="search-category-title">
                <span className="category-header-icon">
                  <group.icon />
                </span>
                <h3>{group.title}</h3>
                <span className="category-header-count">{group.items.length}</span>
              </div>
              {hasMore && app.searchCategory === "ALL" && (
                <button type="button" className="category-expand-btn" onClick={() => app.toggleCategoryExpanded(group.key)}>
                  {isExpanded ? (
                    <>
                      Show fewer <ChevronUpIcon />
                    </>
                  ) : (
                    <>
                      Show all {group.items.length} <ChevronDownIcon />
                    </>
                  )}
                </button>
              )}
            </div>

            <div className="issue-feed">
              {visibleItems.map((issue) => (
                <IssueCard
                  key={issue.id}
                  issue={issue}
                  saved={app.isSaved(issue)}
                  onSelect={() => app.openIssue(issue)}
                  onToggleSave={() => void app.toggleSave(issue)}
                  bookmarkIcon={<BookmarkIcon saved={app.isSaved(issue)} />}
                  commentIcon={<CommentIcon />}
                />
              ))}
            </div>

            {hasMore && app.searchCategory === "ALL" && (
              <div className="category-footer">
                <button type="button" className="category-toggle-more-btn" onClick={() => app.toggleCategoryExpanded(group.key)}>
                  {isExpanded ? (
                    <>
                      Show fewer results <ChevronUpIcon />
                    </>
                  ) : (
                    <>
                      Show {group.items.length - 3} more in {group.title} <ChevronDownIcon />
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
