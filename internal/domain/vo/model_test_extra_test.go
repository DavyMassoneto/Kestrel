package vo_test

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestSupportedModelIDs(t *testing.T) {
	ids := vo.SupportedModelIDs()

	if len(ids) == 0 {
		t.Fatal("SupportedModelIDs should return non-empty list")
	}

	// Verify expected models are present
	expected := map[string]bool{
		"claude-sonnet-4-20250514":     false,
		"claude-opus-4-20250514":       false,
		"claude-haiku-4-20250514":      false,
		"claude-3-5-sonnet-20241022":   false,
		"claude-3-5-haiku-20241022":    false,
		"claude-3-opus-20240229":       false,
		"claude-3-sonnet-20240229":     false,
		"claude-3-haiku-20240307":      false,
	}

	for _, id := range ids {
		if _, ok := expected[id]; ok {
			expected[id] = true
		}
	}

	for model, found := range expected {
		if !found {
			t.Errorf("expected model %q not found in SupportedModelIDs()", model)
		}
	}

	if len(ids) != len(expected) {
		t.Errorf("SupportedModelIDs returned %d models; want %d", len(ids), len(expected))
	}
}
