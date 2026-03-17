package logger_test

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/infra/logger"
)

func TestNew_JSONFormat(t *testing.T) {
	log := logger.New("info", "json")
	if log == nil {
		t.Fatal("logger should not be nil")
	}
}

func TestNew_PrettyFormat(t *testing.T) {
	log := logger.New("debug", "pretty")
	if log == nil {
		t.Fatal("logger should not be nil")
	}
}

func TestNew_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			log := logger.New(level, "json")
			if log == nil {
				t.Fatalf("logger for level %q should not be nil", level)
			}
		})
	}
}

func TestNew_DefaultFormat(t *testing.T) {
	log := logger.New("info", "")
	if log == nil {
		t.Fatal("logger with empty format should default to JSON and not be nil")
	}
}
