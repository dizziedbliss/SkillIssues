// Package githubapp implements the GitHub App credential model (plan D5):
// a JWT-derived installation token for server-side work (ingestion,
// on-demand reads), and a user-to-server token — minted via the same
// "Request user authorization during installation" OAuth-ish flow GitHub
// Apps support — for the contribution path (fork, push, PR), so that work
// is attributed to the user instead of the server.
//
// This package only talks to GitHub; it has no database access. Storing and
// refreshing tokens is the caller's job (see internal/auth).
package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds the App's own identity and credentials.
type Config struct {
	AppID        string
	ClientID     string
	ClientSecret string
	PrivateKey   *rsa.PrivateKey
	CallbackURL  string

	// InstallationID pins which installation mints the server's installation
	// token. Leave zero to auto-discover the (expected to be sole) installation
	// via GET /app/installations on first use.
	InstallationID int64
}

// LoadPrivateKeyFile reads and parses a PKCS1 RSA private key PEM file, as
// downloaded from the GitHub App's settings page.
func LoadPrivateKeyFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("githubapp: read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("githubapp: no PEM block found in private key file")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("githubapp: parse private key: %w", err)
	}
	return key, nil
}

// App mints and caches the two token types the credential model needs.
type App struct {
	cfg Config

	mu              sync.Mutex
	installationID  int64
	installationTok string
	installationExp time.Time
}

// New constructs an App. cfg.PrivateKey must be non-nil.
func New(cfg Config) (*App, error) {
	if cfg.PrivateKey == nil {
		return nil, errors.New("githubapp: private key is required")
	}
	if cfg.AppID == "" {
		return nil, errors.New("githubapp: app ID is required")
	}
	return &App{cfg: cfg, installationID: cfg.InstallationID}, nil
}

// ---------------------------------------------------------------------------
// App JWT (RS256, 9-minute lifetime — GitHub rejects anything over 10)
// ---------------------------------------------------------------------------

func b64url(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func (a *App) signedJWT() (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	payload := map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(), // clock drift tolerance
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.cfg.AppID,
	}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	unsigned := b64url(hb) + "." + b64url(pb)
	hashed := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.cfg.PrivateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("githubapp: sign jwt: %w", err)
	}
	return unsigned + "." + b64url(sig), nil
}

// ---------------------------------------------------------------------------
// Installation token (server identity: ingestion, on-demand reads)
// ---------------------------------------------------------------------------

// InstallationToken returns a cached installation access token, minting a
// fresh one if the cached token is within 5 minutes of its hour-long expiry.
func (a *App) InstallationToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.installationTok != "" && time.Until(a.installationExp) > 5*time.Minute {
		return a.installationTok, nil
	}

	if a.installationID == 0 {
		id, err := a.discoverInstallationID(ctx)
		if err != nil {
			return "", err
		}
		a.installationID = id
	}

	appJWT, err := a.signedJWT()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", a.installationID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("githubapp: mint installation token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("githubapp: mint installation token: HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("githubapp: parse installation token response: %w", err)
	}

	a.installationTok = result.Token
	a.installationExp = result.ExpiresAt
	return a.installationTok, nil
}

func (a *App) discoverInstallationID(ctx context.Context) (int64, error) {
	appJWT, err := a.signedJWT()
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/app/installations", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("githubapp: list installations: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("githubapp: list installations: HTTP %d: %s", resp.StatusCode, body)
	}

	var installations []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &installations); err != nil {
		return 0, fmt.Errorf("githubapp: parse installations response: %w", err)
	}
	if len(installations) == 0 {
		return 0, errors.New("githubapp: app has no installations")
	}
	return installations[0].ID, nil
}

// ---------------------------------------------------------------------------
// User-to-server token (contribution path: fork, push, PR)
// ---------------------------------------------------------------------------

// UserToken is the result of exchanging an authorization code or refresh
// token. AccessToken lasts ExpiresIn seconds (8 hours as of this writing);
// RefreshToken lasts RefreshExpiresIn seconds (~182 days) and must be used
// to mint a new AccessToken before it expires — GitHub does not auto-renew.
type UserToken struct {
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
}

// CallbackURL returns the configured OAuth callback URL, for callers that
// need to store it (e.g. alongside CSRF state) without reaching into Config.
func (a *App) CallbackURL() string { return a.cfg.CallbackURL }

// AuthorizeURL builds the GitHub authorize URL for the user-authorization
// flow. state should be a server-generated, single-use CSRF token the
// caller validates on callback.
func (a *App) AuthorizeURL(state string) string {
	params := url.Values{
		"client_id":    {a.cfg.ClientID},
		"redirect_uri": {a.cfg.CallbackURL},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// ExchangeCode exchanges an authorization code (from the callback) for a
// user-to-server token.
func (a *App) ExchangeCode(ctx context.Context, code string) (*UserToken, error) {
	return a.tokenRequest(ctx, url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {a.cfg.CallbackURL},
	})
}

// RefreshUserToken mints a new access token from a still-valid refresh token.
func (a *App) RefreshUserToken(ctx context.Context, refreshToken string) (*UserToken, error) {
	return a.tokenRequest(ctx, url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func (a *App) tokenRequest(ctx context.Context, form url.Values) (*UserToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("githubapp: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken           string `json:"access_token"`
		ExpiresIn             string `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn string `json:"refresh_token_expires_in"`
		Error                 string `json:"error"`
		ErrorDescription      string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("githubapp: parse token response: %w", err)
	}
	if result.Error != "" {
		if result.ErrorDescription != "" {
			return nil, fmt.Errorf("githubapp: %s (%s)", result.Error, result.ErrorDescription)
		}
		return nil, fmt.Errorf("githubapp: %s", result.Error)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("githubapp: token response had no access_token: %s", body)
	}

	now := time.Now()
	tok := &UserToken{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}
	if secs, err := strconv.Atoi(result.ExpiresIn); err == nil {
		tok.ExpiresAt = now.Add(time.Duration(secs) * time.Second)
	}
	if secs, err := strconv.Atoi(result.RefreshTokenExpiresIn); err == nil {
		tok.RefreshExpiresAt = now.Add(time.Duration(secs) * time.Second)
	}
	return tok, nil
}
