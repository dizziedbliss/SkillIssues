package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/httpx"
)

func testApp(t *testing.T) (*App, *rsa.PrivateKey) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	app, err := New(Config{AppID: "12345", PrivateKey: key, HTTP: httpx.New(func() time.Duration { return 5 * time.Second }, nil)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app, key
}

func TestSignedJWTStructureAndSignature(t *testing.T) {
	app, key := testApp(t)

	tok, err := app.signedJWT()
	if err != nil {
		t.Fatalf("signedJWT: %v", err)
	}

	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "RS256" || header["typ"] != "JWT" {
		t.Fatalf("unexpected header: %v", header)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["iss"] != "12345" {
		t.Fatalf("expected iss=12345, got %v", payload["iss"])
	}
	exp, _ := payload["exp"].(float64)
	if time.Unix(int64(exp), 0).Before(time.Now()) {
		t.Fatal("token already expired")
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	unsigned := parts[0] + "." + parts[1]
	hashed := sha256.Sum256([]byte(unsigned))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hashed[:], sig); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
}

func TestInstallationTokenReusesCachedValue(t *testing.T) {
	app, _ := testApp(t)
	app.installationID = 1
	app.installationTok = "cached-token"
	app.installationExp = time.Now().Add(time.Hour)

	tok, err := app.InstallationToken(context.Background())
	if err != nil {
		t.Fatalf("InstallationToken: %v", err)
	}
	if tok != "cached-token" {
		t.Fatalf("expected cached token, got %q", tok)
	}
}
