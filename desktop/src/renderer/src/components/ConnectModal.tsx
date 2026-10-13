import { Modal } from "@skillissues/ui";
import { useApp } from "../state/AppState";
import { GitHubMark } from "../lib/icons";

// OAuth-only — the PAT/classic-token input (#github-token-modal in the old
// markup) is gone per the user's decision: Electron signs in through the
// system browser only.
export function ConnectModal() {
  const app = useApp();

  return (
    <Modal open={app.connectModalOpen} onClose={app.closeConnectModal} className="push-card" labelledBy="connect-modal-title">
      <button type="button" className="close" onClick={app.closeConnectModal} aria-label="Close">
        ×
      </button>
      <h2 id="connect-modal-title">Connect to GitHub</h2>
      <p>
        Connect your GitHub account to get personalized challenge recommendations, track XP, and verify merged pull
        requests.
      </p>

      {app.oauthWaiting ? (
        <div style={{ textAlign: "center", padding: "24px 0 12px" }}>
          <div className="oauth-spinner" />
          <h3 style={{ fontSize: "18px", margin: "0 0 8px" }}>Waiting for GitHub Authorization</h3>
          <p style={{ fontSize: "13.5px", color: "var(--text-dim)", maxWidth: "380px", margin: "0 auto 24px", lineHeight: 1.5 }}>
            We opened GitHub in your default web browser. Once you authorize SkillIssues, this window will update and
            sign you in automatically.
          </p>
          <button type="button" className="secondary" style={{ padding: "10px 28px" }} onClick={() => void app.cancelGitHubLogin()}>
            Cancel Authorization
          </button>
        </div>
      ) : (
        <>
          <button
            type="button"
            className="primary pref-save"
            style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: "8px", padding: "12px", fontWeight: 600 }}
            onClick={() => void app.startGitHubLogin()}
          >
            <GitHubMark size={18} />
            <span>Continue with GitHub (Browser OAuth)</span>
          </button>
          {app.oauthError && (
            <div className="result error" style={{ marginTop: "10px", width: "100%" }}>
              {app.oauthError}
            </div>
          )}
        </>
      )}
    </Modal>
  );
}
