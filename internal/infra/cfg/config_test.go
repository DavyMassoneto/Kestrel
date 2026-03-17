package cfg_test

import (
	"os"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/infra/cfg"
)

func unsetAll(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "LOG_LEVEL", "LOG_FORMAT", "DB_PATH", "CLAUDE_BASE_URL"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key")
	t.Setenv("ADMIN_KEY", "test-admin-key")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-api03-test")
}

func TestLoad_Defaults(t *testing.T) {
	unsetAll(t)
	setRequired(t)

	cfg, err := cfg.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d; want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q; want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q; want %q", cfg.LogFormat, "json")
	}
	if cfg.ClaudeBaseURL != "https://api.anthropic.com" {
		t.Errorf("ClaudeBaseURL = %q; want default", cfg.ClaudeBaseURL)
	}
	if cfg.DBPath != "kestrel.db" {
		t.Errorf("DBPath = %q; want %q", cfg.DBPath, "kestrel.db")
	}
}

func TestLoad_OverrideViaEnv(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "pretty")
	t.Setenv("CLAUDE_BASE_URL", "https://custom.api.com")
	t.Setenv("DB_PATH", "/tmp/test.db")

	cfg, err := cfg.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d; want 9090", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q; want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "pretty" {
		t.Errorf("LogFormat = %q; want %q", cfg.LogFormat, "pretty")
	}
	if cfg.ClaudeBaseURL != "https://custom.api.com" {
		t.Errorf("ClaudeBaseURL = %q; want custom", cfg.ClaudeBaseURL)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q; want /tmp/test.db", cfg.DBPath)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "not-a-number")

	_, err := cfg.Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT, got nil")
	}
}

func TestConfig_OAuthDefaults(t *testing.T) {
	unsetAll(t)
	setRequired(t)

	c, err := cfg.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.OAuthClientID != "" {
		t.Errorf("OAuthClientID = %q; want empty", c.OAuthClientID)
	}
	if c.OAuthRedirectURI != "http://localhost:8080/api/oauth/callback" {
		t.Errorf("OAuthRedirectURI = %q; want default", c.OAuthRedirectURI)
	}
	if c.OAuthAuthURL != "https://console.anthropic.com/oauth/authorize" {
		t.Errorf("OAuthAuthURL = %q; want default", c.OAuthAuthURL)
	}
	if c.OAuthTokenURL != "https://console.anthropic.com/oauth/token" {
		t.Errorf("OAuthTokenURL = %q; want default", c.OAuthTokenURL)
	}
	if !strings.Contains(c.OAuthScope, "org:create_api_key") {
		t.Errorf("OAuthScope = %q; want default containing org:create_api_key", c.OAuthScope)
	}
}

func TestConfig_OAuthFromEnv(t *testing.T) {
	unsetAll(t)
	setRequired(t)
	t.Setenv("OAUTH_CLIENT_ID", "my-client")
	t.Setenv("OAUTH_REDIRECT_URI", "https://myapp.com/callback")
	t.Setenv("OAUTH_AUTH_URL", "https://custom-auth.com/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://custom-auth.com/token")

	c, err := cfg.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.OAuthClientID != "my-client" {
		t.Errorf("OAuthClientID = %q; want my-client", c.OAuthClientID)
	}
	if c.OAuthRedirectURI != "https://myapp.com/callback" {
		t.Errorf("OAuthRedirectURI = %q; want custom", c.OAuthRedirectURI)
	}
	if c.OAuthAuthURL != "https://custom-auth.com/authorize" {
		t.Errorf("OAuthAuthURL = %q; want custom", c.OAuthAuthURL)
	}
	if c.OAuthTokenURL != "https://custom-auth.com/token" {
		t.Errorf("OAuthTokenURL = %q; want custom", c.OAuthTokenURL)
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	unsetAll(t)
	// Don't set required fields
	os.Unsetenv("ENCRYPTION_KEY")
	os.Unsetenv("ADMIN_KEY")
	os.Unsetenv("CLAUDE_API_KEY")

	_, err := cfg.Load()
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}
