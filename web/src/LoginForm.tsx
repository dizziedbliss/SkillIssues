/**
 * @file GitHub OAuth / GitHub App sign-in form.
 *
 * OAuth (GitHub App) is the only supported login path — there is no
 * fallback to a Personal Access Token form. If the server has no
 * GITHUB_CLIENT_ID configured, `/auth/github` returns `enabled: false` and
 * this shows that as a real error, never a token input.
 */
import { useState } from "react";
import { X } from "lucide-react";
import { ApiError } from "@skillissues/api-client";
import { apiClient } from "./apiClient";

export function LoginForm({
  onClose,
  initialError,
}: {
  onClose?: () => void;
  initialError?: string;
}) {
  const [oauthLoading, setOAuthLoading] = useState(false);
  const [err, setErr] = useState(initialError || "");

  async function handleOAuthLogin() {
    setOAuthLoading(true);
    setErr("");
    try {
      const data = await apiClient.getGithubAuthorize();

      if (!data.enabled || !data.authorize_url) {
        setErr(data.error || "GitHub OAuth is not enabled on this server.");
        setOAuthLoading(false);
        return;
      }

      window.location.href = data.authorize_url;
    } catch (caught) {
      setErr(
        caught instanceof ApiError
          ? caught.message
          : "Failed to initiate GitHub OAuth — check your connection and try again.",
      );
      setOAuthLoading(false);
    }
  }

  return (
    <main className="center login-screen">
      <div className="login-card">
        {onClose && (
          <button
            type="button"
            className="login-modal-close"
            onClick={onClose}
            title="Close login modal"
          >
            <X size={18} />
          </button>
        )}
        <div className="wordmark">
          <img src="/logo.png" alt="SkillIssues Logo" className="brand-logo" />
          <div className="wordmark-text">
            SkillIssues<span>You Have the Skills. We Have the Issues.</span>
          </div>
        </div>
        <h1 className="login-title">Sign in to SkillIssues</h1>
        <p className="login-desc">
          Connect with your GitHub account to get personalized issue recommendations,
          track XP progression, and verify merged pull requests.
        </p>

        {err && (
          <div className="auth-alert-box">
            <span>{err}</span>
          </div>
        )}

        <button
          type="button"
          className="oauth-github-btn"
          onClick={() => void handleOAuthLogin()}
          disabled={oauthLoading}
        >
          <svg className="github-icon" height="20" width="20" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
          </svg>
          <span>{oauthLoading ? "Connecting to GitHub..." : "Continue with GitHub"}</span>
        </button>
      </div>
    </main>
  );
}
