package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	enc, err := c.Encrypt("ghp_secret_token")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == "ghp_secret_token" {
		t.Fatal("ciphertext must not equal plaintext")
	}

	plain, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plain != "ghp_secret_token" {
		t.Fatalf("round trip mismatch: got %q", plain)
	}
}

func TestEmptyInputPassesThrough(t *testing.T) {
	key := make([]byte, 32)
	c, _ := New(key)

	if enc, _ := c.Encrypt(""); enc != "" {
		t.Fatalf("expected empty ciphertext, got %q", enc)
	}
	if plain, _ := c.Decrypt(""); plain != "" {
		t.Fatalf("expected empty plaintext, got %q", plain)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1
	c1, _ := New(key1)
	c2, _ := New(key2)

	enc, _ := c1.Encrypt("token")
	if _, err := c2.Decrypt(enc); err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

func TestNewRejectsWrongKeyLength(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}
