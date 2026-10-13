package main

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/google/uuid"
)

// authGitHubBegin handles GET|POST /auth/github
func (h *handlers) authGitHubBegin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubClientID == "" {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{
			"enabled":       false,
			"authorize_url": "",
			"redirect_uri":  h.cfg.GitHubRedirectURI,
			"error":         "GITHUB_CLIENT_ID is not configured",
		})
		return
	}

	authorizeURL, state, err := h.ghOAuth.BeginOAuth(r.Context(), h.cfg.GitHubRedirectURI)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to begin OAuth flow: "+err.Error())
		return
	}

	isDesktop := r.URL.Query().Get("source") == "desktop"
	var ticket string
	if isDesktop {
		ticket = uuid.NewString()
		h.stateTickets.Store(state, ticketEntry{data: ticket, createdAt: time.Now()})
	}

	// If browser directly navigated or explicitly requested redirect
	if r.URL.Query().Get("redirect") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, authorizeURL, http.StatusFound)
		return
	}

	resp := map[string]any{
		"enabled":       true,
		"authorize_url": authorizeURL,
		"redirect_uri":  h.cfg.GitHubRedirectURI,
	}
	if ticket != "" {
		resp["ticket"] = ticket
	}
	middleware.RespondJSON(w, http.StatusOK, resp)
}

// authGitHubCallback handles GET /auth/github/callback
func (h *handlers) authGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	frontendURL := h.cfg.FrontendURLPrimary

	var ticket string
	if stateVal, ok := h.stateTickets.LoadAndDelete(state); ok {
		if entry, ok := stateVal.(ticketEntry); ok && !entry.expired() {
			ticket = entry.data.(string)
		}
	}

	redirectWithError := func(code string, detail string) {
		if ticket != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SkillIssues — Authentication Error</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-app: #f7f6fb;
      --text-primary: #1e1338;
      --text-muted: #6b5f88;
      --border-card: #e5e0f2;
      --accent-primary: #241442;
      --accent-primary-hover: #371f63;
    }
    * { box-sizing: border-box; }
    body {
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg-app);
      color: var(--text-primary);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 24px;
      -webkit-font-smoothing: antialiased;
    }
    .card {
      background: #ffffff;
      border: 1px solid var(--border-card);
      border-radius: 24px;
      padding: 44px 36px;
      text-align: center;
      max-width: 440px;
      width: 100%%;
      box-shadow: 0 20px 40px -12px rgba(36, 20, 66, 0.08), 0 2px 6px rgba(36, 20, 66, 0.04);
    }
    .badge-icon {
      width: 60px;
      height: 60px;
      border-radius: 18px;
      background: #fef2f2;
      color: #dc2626;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 20px;
    }
    h2 {
      font-size: 22px;
      font-weight: 800;
      color: var(--text-primary);
      margin: 0 0 12px;
      letter-spacing: -0.02em;
    }
    .err-pill {
      display: inline-block;
      background: #fef2f2;
      color: #b91c1c;
      border: 1px solid #fecaca;
      padding: 8px 16px;
      border-radius: 10px;
      font-size: 13.5px;
      font-weight: 600;
      margin-bottom: 20px;
      word-break: break-word;
      max-width: 100%%;
    }
    p {
      font-size: 14px;
      color: var(--text-muted);
      line-height: 1.6;
      margin: 0;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge-icon">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
      </svg>
    </div>
    <h2>Authentication Failed</h2>
    <div class="err-pill">%s: %s</div>
    <p>You can close this tab and return to the application to try again.</p>
  </div>
</body>
</html>`, html.EscapeString(code), html.EscapeString(detail))
			return
		}
		sep := "?"
		if strings.Contains(frontendURL, "?") {
			sep = "&"
		}
		target := fmt.Sprintf("%s%serror=%s", frontendURL, sep, url.QueryEscape(code))
		if detail != "" {
			target += fmt.Sprintf("&error_description=%s", url.QueryEscape(detail))
		}
		http.Redirect(w, r, target, http.StatusFound)
	}

	if errParam != "" {
		redirectWithError(errParam, errDesc)
		return
	}

	if code == "" || state == "" {
		redirectWithError("missing_code_or_state", "Authorization code or state parameter missing from callback")
		return
	}

	ghToken, ghUser, err := h.ghOAuth.ExchangeOAuthCode(r.Context(), code, state)
	if err != nil {
		slog.Error("failed to exchange oauth code", "error", err)
		redirectWithError("oauth_failed", err.Error())
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, ghToken)
	if err != nil {
		slog.Error("failed to upsert user after oauth", "error", err)
		redirectWithError("user_creation_failed", err.Error())
		return
	}

	rawToken, err := h.authSvc.CreateSession(r.Context(), userID, "github_oauth")
	if err != nil {
		slog.Error("failed to create session after oauth", "error", err)
		redirectWithError("session_failed", err.Error())
		return
	}

	h.setSessionCookie(w, rawToken)

	// If this flow originated from Electron Desktop, record ticket & present auto-redirect landing page
	if ticket != "" {
		h.desktopTickets.Store(ticket, ticketEntry{
			createdAt: time.Now(),
			data: map[string]any{
				"session_token": rawToken,
				"user": map[string]any{
					"id":         userID,
					"username":   ghUser.Username,
					"avatar_url": ghUser.AvatarURL,
				},
			},
		})

		deepLink := fmt.Sprintf("skillissues://auth?token=%s&ticket=%s", url.QueryEscape(rawToken), url.QueryEscape(ticket))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SkillIssues — Authorized</title>
  <meta http-equiv="refresh" content="0;url=%s" />
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=Noto+Sans:wght@700;800&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-app: #f7f6fb;
      --text-primary: #1e1338;
      --text-muted: #6b5f88;
      --border-card: #e5e0f2;
      --accent-primary: #241442;
      --accent-primary-hover: #371f63;
      --accent-blue: #7c3aed;
    }
    * { box-sizing: border-box; }
    body {
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg-app);
      color: var(--text-primary);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 24px;
      -webkit-font-smoothing: antialiased;
    }
    .card {
      background: #ffffff;
      border: 1px solid var(--border-card);
      border-radius: 24px;
      padding: 44px 36px;
      text-align: center;
      max-width: 440px;
      width: 100%%;
      box-shadow: 0 20px 40px -12px rgba(36, 20, 66, 0.08), 0 2px 6px rgba(36, 20, 66, 0.04);
    }
    .badge-icon {
      width: 60px;
      height: 60px;
      border-radius: 18px;
      background: #ede9f6;
      color: var(--accent-blue);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 20px;
    }
    h1 {
      font-size: 22px;
      font-weight: 800;
      color: var(--text-primary);
      margin: 0 0 10px;
      letter-spacing: -0.02em;
    }
    p {
      font-size: 14px;
      color: var(--text-muted);
      line-height: 1.6;
      margin: 0 0 28px;
    }
    a.btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      background: var(--accent-primary);
      color: #ffffff;
      text-decoration: none;
      padding: 12px 28px;
      border-radius: 12px;
      font-weight: 600;
      font-size: 14px;
      box-shadow: 0 4px 14px rgba(36, 20, 66, 0.15);
      transition: all 0.15s ease;
    }
    a.btn:hover {
      background: var(--accent-primary-hover);
      transform: translateY(-1px);
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge-icon">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M20 6 9 17l-5-5"/>
      </svg>
    </div>
    <h1>Authorization Successful!</h1>
    <p>You have connected your GitHub account. Redirecting you back to SkillIssues Desktop...</p>
    <a class="btn" href="%s">Open SkillIssues Desktop <span>↗</span></a>
  </div>
  <script>
    setTimeout(function() { window.location.href = %q; }, 100);
  </script>
</body>
</html>`, deepLink, deepLink, deepLink)
		return
	}

	// Web flow: the session cookie was already set above; redirect back to the
	// frontend without putting the token in the URL (it would leak via browser
	// history and the Referer header). ?auth=ok tells the SPA an OAuth round
	// trip just finished, so it can report a real error instead of silently
	// rendering logged-out if the cookie didn't actually take.
	sep := "?"
	if strings.Contains(frontendURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, frontendURL+sep+"auth=ok", http.StatusFound)
}

// authGitHubPoll handles GET /auth/github/poll?ticket=...
// Used by Electron desktop to verify when browser-based OAuth authorization completes.
func (h *handlers) authGitHubPoll(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "ticket is required")
		return
	}
	val, ok := h.desktopTickets.Load(ticket)
	if !ok {
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	entry, ok := val.(ticketEntry)
	if !ok || entry.expired() {
		h.desktopTickets.Delete(ticket)
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	// Consume the ticket: once the desktop app has retrieved the session, the
	// ticket must not be claimable again by anyone who later learns it.
	h.desktopTickets.Delete(ticket)
	data := entry.data.(map[string]any)
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"session_token": data["session_token"],
		"user":          data["user"],
	})
}
