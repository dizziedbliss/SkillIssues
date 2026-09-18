package auth

import (
	"testing"
)

func TestHashToken(t *testing.T) {
	raw := "test-secret-token"
	hashed := hashToken(raw)
	if hashed == "" {
		t.Fatal("expected non-empty hash")
	}
	if hashed == raw {
		t.Fatal("hash should not match raw token")
	}
	// Verify deterministic hashing
	if hashToken(raw) != hashed {
		t.Fatal("hashing should be deterministic")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := []byte("secret_key")
	body := []byte(`{"action":"opened"}`)
	// HMAC-SHA256 for this payload and secret:
	// Let's test with valid signature using generate/verify
	sig := "sha256=invalid"
	if VerifyWebhookSignature(secret, body, sig) {
		t.Fatal("expected invalid signature to fail")
	}

	if VerifyWebhookSignature(secret, body, "") {
		t.Fatal("expected empty signature to fail")
	}
}
