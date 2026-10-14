package config

import (
	"strings"
	"testing"
)

// In production, a binary refuses to start on a placeholder for a secret it
// requires, and does not care about secrets it does not require.
func TestLoadRequiresOnlyNamedSecretsInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgresql://u:p@localhost/db")
	t.Setenv("ADMIN_SYNC_TOKEN", "real-admin-token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")

	if _, err := Load(SecretAdminToken); err != nil {
		t.Fatalf("Load(SecretAdminToken) with only the admin token set: %v", err)
	}

	_, err := Load(SecretWebhook, SecretAdminToken, SecretTokenEncryption)
	if err == nil {
		t.Fatal("Load with placeholder webhook secret and encryption key: want error, got nil")
	}
	for _, name := range []string{"GITHUB_WEBHOOK_SECRET", "TOKEN_ENCRYPTION_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "ADMIN_SYNC_TOKEN") {
		t.Errorf("error %q names ADMIN_SYNC_TOKEN, which is set", err)
	}
}

func TestLoadAllowsPlaceholdersOutsideProduction(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("DATABASE_URL", "postgresql://u:p@localhost/db")
	t.Setenv("ADMIN_SYNC_TOKEN", "")
	if _, err := Load(SecretWebhook, SecretAdminToken, SecretTokenEncryption); err != nil {
		t.Fatalf("development Load: %v", err)
	}
}
