package config_test

import (
	"os"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/infra/config"
)

func unsetAll(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "LOG_LEVEL", "LOG_FORMAT"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestLoad_Defaults(t *testing.T) {
	unsetAll(t)

	cfg, err := config.Load()
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
}

func TestLoad_OverrideViaEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "pretty")

	cfg, err := config.Load()
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
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT, got nil")
	}
}
