package entity_test

import (
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func newTestAPIKey(t *testing.T) *entity.APIKey {
	t.Helper()
	k, err := entity.NewAPIKey(vo.NewAPIKeyID(), "test-key", "hashed-value", "omni_test")
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	return k
}

func TestNewAPIKey_Valid(t *testing.T) {
	id := vo.NewAPIKeyID()
	k, err := entity.NewAPIKey(id, "my-key", "hash123", "omni_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.ID() != id {
		t.Errorf("ID = %v; want %v", k.ID(), id)
	}
	if k.Name() != "my-key" {
		t.Errorf("Name = %q; want %q", k.Name(), "my-key")
	}
	if k.KeyHash() != "hash123" {
		t.Errorf("KeyHash = %q; want %q", k.KeyHash(), "hash123")
	}
	if k.KeyPrefix() != "omni_abc" {
		t.Errorf("KeyPrefix = %q; want %q", k.KeyPrefix(), "omni_abc")
	}
	if !k.IsActive() {
		t.Error("new key should be active")
	}
	if len(k.AllowedModels()) != 0 {
		t.Errorf("AllowedModels len = %d; want 0", len(k.AllowedModels()))
	}
	if k.LastUsedAt() != nil {
		t.Error("LastUsedAt should be nil")
	}
}

func TestNewAPIKey_EmptyName(t *testing.T) {
	_, err := entity.NewAPIKey(vo.NewAPIKeyID(), "", "hash", "prefix")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNewAPIKey_EmptyHash(t *testing.T) {
	_, err := entity.NewAPIKey(vo.NewAPIKeyID(), "name", "", "prefix")
	if err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestNewAPIKey_EmptyPrefix(t *testing.T) {
	_, err := entity.NewAPIKey(vo.NewAPIKeyID(), "name", "hash", "")
	if err == nil {
		t.Fatal("expected error for empty prefix")
	}
}

func TestAPIKey_Validate_Match(t *testing.T) {
	k := newTestAPIKey(t)
	match := k.Validate("raw-key", func(hash, raw string) bool {
		return hash == "hashed-value" && raw == "raw-key"
	})
	if !match {
		t.Error("Validate should return true for matching key")
	}
}

func TestAPIKey_Validate_NoMatch(t *testing.T) {
	k := newTestAPIKey(t)
	match := k.Validate("wrong-key", func(hash, raw string) bool {
		return false
	})
	if match {
		t.Error("Validate should return false for non-matching key")
	}
}

func TestAPIKey_IsModelAllowed_EmptyList(t *testing.T) {
	k := newTestAPIKey(t)
	if !k.IsModelAllowed("any-model") {
		t.Error("empty allowed list should allow any model")
	}
}

func TestAPIKey_IsModelAllowed_InList(t *testing.T) {
	k := newTestAPIKey(t)
	k.SetAllowedModels([]string{"claude-sonnet-4-20250514", "claude-opus-4-20250514"})

	if !k.IsModelAllowed("claude-sonnet-4-20250514") {
		t.Error("model in list should be allowed")
	}
}

func TestAPIKey_IsModelAllowed_NotInList(t *testing.T) {
	k := newTestAPIKey(t)
	k.SetAllowedModels([]string{"claude-sonnet-4-20250514"})

	if k.IsModelAllowed("claude-opus-4-20250514") {
		t.Error("model not in list should not be allowed")
	}
}

func TestAPIKey_RecordUsage(t *testing.T) {
	k := newTestAPIKey(t)
	now := time.Now()

	k.RecordUsage(now)

	if k.LastUsedAt() == nil {
		t.Fatal("LastUsedAt should not be nil")
	}
	if !k.LastUsedAt().Equal(now) {
		t.Errorf("LastUsedAt = %v; want %v", k.LastUsedAt(), now)
	}
}
