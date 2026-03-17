package vo_test

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestParseModelName_Valid(t *testing.T) {
	tests := []struct {
		raw      string
		resolved string
	}{
		{"claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"claude-opus-4-20250514", "claude-opus-4-20250514"},
		{"claude-haiku-4-20250514", "claude-haiku-4-20250514"},
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022"},
		{"claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022"},
		{"claude-3-opus-20240229", "claude-3-opus-20240229"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			m, err := vo.ParseModelName(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.Resolved != tt.resolved {
				t.Errorf("Resolved = %q; want %q", m.Resolved, tt.resolved)
			}
			if m.Raw != tt.raw {
				t.Errorf("Raw = %q; want %q", m.Raw, tt.raw)
			}
			if !m.IsValid() {
				t.Error("IsValid should return true")
			}
		})
	}
}

func TestParseModelName_Invalid(t *testing.T) {
	_, err := vo.ParseModelName("gpt-4")
	if err == nil {
		t.Fatal("expected error for unsupported model")
	}
}

func TestParseModelName_Empty(t *testing.T) {
	_, err := vo.ParseModelName("")
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestModelName_IsValid_Unsupported(t *testing.T) {
	// Create a ModelName with unsupported resolved name
	m := vo.ModelName{Raw: "unknown", Resolved: "unknown"}
	if m.IsValid() {
		t.Error("IsValid should return false for unsupported model")
	}
}
