import { Modal } from "@skillissues/ui";
import { useApp } from "../state/AppState";

export function WorkspaceDrawer() {
  const app = useApp();
  const issue = app.selectedIssue;

  if (!issue) return null;

  return (
    <Modal open={app.drawerOpen} onClose={app.closeDrawer} className="drawer-card" labelledBy="drawer-title">
      <button type="button" className="close" onClick={app.closeDrawer} aria-label="Close">
        ×
      </button>

      <div className="drawer-nav">
        <button
          type="button"
          className={`drawer-tab ${app.drawerTab === "issue" ? "active" : ""}`}
          onClick={() => app.setDrawerTab("issue")}
        >
          📋 Issue Details
        </button>
        <button
          type="button"
          className={`drawer-tab ${app.drawerTab === "workspace" ? "active" : ""}`}
          onClick={() => app.setDrawerTab("workspace")}
        >
          💻 Workspace & Logs
        </button>
      </div>

      {app.drawerTab === "issue" ? (
        <div className="drawer-page">
          <span className={`difficulty ${(issue.difficulty || "").toLowerCase()}`}>
            {issue.difficulty || "BEGINNER"}
          </span>
          <h2 id="drawer-title">
            #{issue.number} {issue.title}
          </h2>
          <p>
            {issue.repository} · {issue.language || "Open source"}
          </p>
          <a href={issue.url} target="_blank" rel="noreferrer">
            Open issue ↗
          </a>
          {app.bodyLoading && !app.bodyHtml ? (
            <div className="drawer-body markdown-body loading">Loading issue description...</div>
          ) : (
            <div className="drawer-body markdown-body" dangerouslySetInnerHTML={{ __html: app.bodyHtml }} />
          )}
          <div className="drawer-skills">
            {(issue.required_skills || []).map((skill) => (
              <span className="tag" key={skill}>
                {skill}
              </span>
            ))}
          </div>
          {!app.hasWorkspace && (
            <button type="button" className="primary" style={{ marginTop: "18px" }} disabled={app.startChallengeBusy} onClick={() => void app.startChallenge()}>
              {app.startChallengeBusy ? "Working…" : "Solve issue (Fork & Clone)"} <span>↗</span>
            </button>
          )}
        </div>
      ) : (
        <div className="drawer-page">
          <div className="workspace-header-meta">
            <span className="difficulty">{issue.repository}</span>
            <h2 style={{ marginTop: "6px" }}>
              #{issue.number} {issue.title}
            </h2>
          </div>

          <div className="destination" style={{ marginTop: "16px" }}>
            <label>WORKSPACE ROOT FOLDER (Parent directory for all challenges)</label>
            <div className="workspace-parent-row">
              <div className="workspace-path" title={app.workspaceParentPath || ""}>
                {app.workspaceParentPath || "Not configured (will ask on first clone)"}
              </div>
              <button type="button" className="secondary change-btn" onClick={() => void app.changeWorkspaceParent()}>
                📁 Change Folder
              </button>
            </div>
          </div>

          <div className="destination">
            <label>CHALLENGE CLONE DESTINATION</label>
            <div className="workspace-path">{app.destination || "Choose or clone a repository first..."}</div>
          </div>

          {app.hasWorkspace && (
            <div>
              <div className="git-actions-section" style={{ marginTop: "14px" }}>
                <label>WORKING BRANCH</label>
                <div className="git-branch-row">
                  <input
                    className="workspace-path git-input"
                    value={app.branch}
                    onChange={(e) => app.setBranch(e.target.value)}
                    placeholder="skillissues/work"
                  />
                  <button type="button" className="secondary" disabled={app.branchBusy} onClick={() => void app.createBranchAction()}>
                    Create Branch
                  </button>
                </div>
              </div>
              <div className="git-actions-section" style={{ marginTop: "14px" }}>
                <label>WORKSPACE ACTIONS</label>
                <div className="git-buttons-row">
                  <button type="button" className="secondary" disabled={app.openVSCodeBusy} onClick={() => void app.openInVSCodeAction()}>
                    {app.openVSCodeBusy ? "Launching VS Code..." : "Open in VS Code ↗"}
                  </button>
                  <button type="button" className="secondary" disabled={app.openFolderBusy} onClick={() => void app.openWorkspaceFolder()}>
                    {app.openFolderBusy ? "Opening..." : "Show in Explorer ↗"}
                  </button>
                  <button type="button" className="primary" onClick={app.openPushModal}>
                    Push & Submit PR ↗
                  </button>
                  <button type="button" className="secondary" disabled={app.cloneBusy} onClick={() => void app.cloneAgain()}>
                    {app.cloneBusy ? "Cloning..." : "Re-clone ↗"}
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="logs-section">
            <div className="logs-header-row">
              <label>WORKSPACE & GIT TERMINAL LOGS</label>
              <button type="button" className="subtle-link" onClick={app.clearLogs}>
                Clear Log
              </button>
            </div>
            {app.logs.length > 0 && (
              <div className={`result logs-terminal${app.logs[app.logs.length - 1].error ? " error" : ""}`}>
                {app.logs.map((entry) => entry.text).join("\n\n")}
              </div>
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}
