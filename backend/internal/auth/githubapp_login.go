// GitHub App user-authorization login (plan D5). Separate from the OAuth
// App flow in github.go — this mints a user-to-server token (fork/push/PR
// attributed to the user) rather than a classic, non-expiring OAuth token.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/githubapp"
	"github.com/jackc/pgx/v5"
)

// GitHubAppLogin drives the GitHub App's user-authorization flow.
type GitHubAppLogin struct {
	app   *githubapp.App
	db    *Service
	oauth *GitHubOAuth // reused only for its GraphQL "viewer" lookup
}

// NewGitHubAppLogin constructs a GitHubAppLogin.
func NewGitHubAppLogin(app *githubapp.App, authSvc *Service, oauth *GitHubOAuth) *GitHubAppLogin {
	return &GitHubAppLogin{app: app, db: authSvc, oauth: oauth}
}

// BeginAuth generates a CSRF state, stores it (reusing oauth_states; GitHub
// Apps don't use PKCE, so code_verifier is stored empty), and returns the
// authorize URL to redirect the user to.
func (g *GitHubAppLogin) BeginAuth(ctx context.Context) (authorizeURL, state string, err error) {
	stateBuf := make([]byte, 32)
	if _, err := rand.Read(stateBuf); err != nil {
		return "", "", fmt.Errorf("github app auth: generate state: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(stateBuf)

	if err := g.db.db.Exec(ctx,
		`INSERT INTO oauth_states (state, code_verifier, redirect_uri, expires_at)
		 VALUES ($1, '', $2, $3)`,
		state, g.app.CallbackURL(), time.Now().Add(10*time.Minute),
	); err != nil {
		return "", "", fmt.Errorf("github app auth: store state: %w", err)
	}

	return g.app.AuthorizeURL(state), state, nil
}

// ExchangeCode validates the CSRF state, exchanges the code for a
// user-to-server token, fetches the GitHub user, and upserts the user with
// that token (access + refresh + expiry). Returns the local user ID.
func (g *GitHubAppLogin) ExchangeCode(ctx context.Context, code, state string) (int, *GitHubUser, error) {
	var ok bool
	err := g.db.db.QueryRow(ctx,
		`DELETE FROM oauth_states WHERE state = $1 AND expires_at > now() RETURNING true`,
		[]any{state},
		func(row pgx.Row) error { return row.Scan(&ok) },
	)
	if err != nil || !ok {
		return 0, nil, fmt.Errorf("github app auth: invalid or expired state: %w", ErrUnauthorized)
	}

	tok, err := g.app.ExchangeCode(ctx, code)
	if err != nil {
		return 0, nil, fmt.Errorf("github app auth: exchange code: %w", err)
	}

	ghUser, err := g.oauth.GetAuthenticatedUser(ctx, tok.AccessToken)
	if err != nil {
		return 0, nil, fmt.Errorf("github app auth: get user: %w", err)
	}

	userID, err := g.db.UpsertUserWithAppToken(ctx, ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL, tok)
	if err != nil {
		return 0, nil, fmt.Errorf("github app auth: upsert user: %w", err)
	}

	return userID, ghUser, nil
}
