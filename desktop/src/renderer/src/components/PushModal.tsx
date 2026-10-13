import { Modal } from "@skillissues/ui";
import { useApp } from "../state/AppState";

export function PushModal() {
  const app = useApp();

  return (
    <Modal open={app.pushModalOpen} onClose={app.closePushModal} className="drawer-card push-card" labelledBy="push-modal-title">
      <button type="button" className="close" onClick={app.closePushModal} aria-label="Close">
        ×
      </button>
      <h2 id="push-modal-title">Push & Submit Pull Request</h2>
      <p>Review commit message, working branch, and pull request details before pushing to your GitHub fork.</p>

      <div className="destination">
        <label>WORKING BRANCH</label>
        <input className="workspace-path" value={app.pushBranch} onChange={(e) => app.setPushBranch(e.target.value)} placeholder="skillissues/work" />
      </div>
      <div className="destination">
        <label>COMMIT MESSAGE (Stages & commits uncommitted changes)</label>
        <input
          className="workspace-path"
          value={app.pushCommitMessage}
          onChange={(e) => app.setPushCommitMessage(e.target.value)}
          placeholder="e.g. Fix authentication error handling"
        />
      </div>
      <div className="destination">
        <label>PULL REQUEST TITLE</label>
        <input className="workspace-path" value={app.pushPrTitle} onChange={(e) => app.setPushPrTitle(e.target.value)} placeholder="Pull Request Title" />
      </div>

      <button type="button" className="primary pref-save" disabled={app.pushBusy} onClick={() => void app.confirmPush()}>
        {app.pushBusy ? "Pushing & Submitting PR..." : "Confirm, Push & Submit PR"} <span>↗</span>
      </button>
    </Modal>
  );
}
