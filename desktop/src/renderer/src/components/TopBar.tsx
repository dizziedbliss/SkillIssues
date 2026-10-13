import { useApp } from "../state/AppState";
import { LogoutIcon } from "../lib/icons";

export function TopBar() {
  const app = useApp();
  const { session } = app;

  return (
    <header className="topbar">
      <div className="wordmark">
        <img src="logo.png" alt="SkillIssues Logo" className="brand-logo" />
        <div className="wordmark-text">
          SkillIssues
          <span>You Have the Skills. We Have the Issues.</span>
        </div>
      </div>
      <div className="auth-actions">
        {session.authenticated ? (
          <>
            <span>@{session.user?.username || "connected"}</span>
            <button
              type="button"
              className="auth-button subtle"
              disabled={app.loggingOut}
              onClick={() => void app.logout()}
            >
              <LogoutIcon />
              {app.loggingOut ? "Signing out…" : "Log out"}
            </button>
          </>
        ) : (
          <button type="button" className="auth-button" onClick={app.openConnectModal}>
            Sign in with GitHub
          </button>
        )}
      </div>
    </header>
  );
}
