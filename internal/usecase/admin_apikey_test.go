package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type mockAPIKeyStore struct {
	keys     map[string]*entity.APIKey
	createFn func(ctx context.Context, key *entity.APIKey) error
}

func newMockAPIKeyStore() *mockAPIKeyStore {
	return &mockAPIKeyStore{keys: make(map[string]*entity.APIKey)}
}

func (m *mockAPIKeyStore) FindByID(_ context.Context, id vo.APIKeyID) (*entity.APIKey, error) {
	key, ok := m.keys[id.String()]
	if !ok {
		return nil, errors.New("API key not found")
	}
	return key, nil
}

func (m *mockAPIKeyStore) FindAll(_ context.Context) ([]*entity.APIKey, error) {
	var result []*entity.APIKey
	for _, k := range m.keys {
		result = append(result, k)
	}
	return result, nil
}

func (m *mockAPIKeyStore) Create(ctx context.Context, key *entity.APIKey) error {
	if m.createFn != nil {
		return m.createFn(ctx, key)
	}
	m.keys[key.ID().String()] = key
	return nil
}

func (m *mockAPIKeyStore) Delete(_ context.Context, id vo.APIKeyID) error {
	if _, ok := m.keys[id.String()]; !ok {
		return errors.New("API key not found")
	}
	delete(m.keys, id.String())
	return nil
}

func TestCreateAPIKey_Success(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	input := CreateAPIKeyInput{
		Name:          "test-key",
		AllowedModels: []string{"claude-sonnet-4-20250514"},
	}

	key, rawKey, err := uc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if key.Name() != "test-key" {
		t.Errorf("Name = %q, want %q", key.Name(), "test-key")
	}
	if key.ID().String() == "" {
		t.Error("ID should not be empty")
	}
	if rawKey == "" {
		t.Error("rawKey should not be empty")
	}
	if !strings.HasPrefix(rawKey, "omni-") {
		t.Errorf("rawKey = %q, should start with 'omni-'", rawKey)
	}
	if key.KeyPrefix() == "" {
		t.Error("KeyPrefix should not be empty")
	}
	if !strings.HasPrefix(rawKey, key.KeyPrefix()) {
		t.Errorf("rawKey %q should start with prefix %q", rawKey, key.KeyPrefix())
	}
	if key.KeyHash() == "" {
		t.Error("KeyHash should not be empty")
	}
	// Hash should NOT be the raw key
	if key.KeyHash() == rawKey {
		t.Error("KeyHash should be a hash, not the raw key")
	}
	if !key.IsActive() {
		t.Error("key should be active")
	}

	models := key.AllowedModels()
	if len(models) != 1 || models[0] != "claude-sonnet-4-20250514" {
		t.Errorf("AllowedModels = %v, want [claude-sonnet-4-20250514]", models)
	}

	// Verify persisted
	if len(store.keys) != 1 {
		t.Errorf("store has %d keys, want 1", len(store.keys))
	}
}

func TestCreateAPIKey_EmptyName(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	input := CreateAPIKeyInput{Name: ""}

	_, _, err := uc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateAPIKey_NoAllowedModels(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	input := CreateAPIKeyInput{
		Name:          "all-models-key",
		AllowedModels: nil,
	}

	key, _, err := uc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Empty allowed models = all models allowed
	if len(key.AllowedModels()) != 0 {
		t.Errorf("AllowedModels = %v, want empty", key.AllowedModels())
	}
}

func TestCreateAPIKey_StoreFails(t *testing.T) {
	store := newMockAPIKeyStore()
	store.createFn = func(_ context.Context, _ *entity.APIKey) error {
		return errors.New("db error")
	}
	uc := NewAdminAPIKeyUseCase(store)

	input := CreateAPIKeyInput{Name: "test"}

	_, _, err := uc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRevokeAPIKey_Success(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	key, _ := entity.NewAPIKey(vo.NewAPIKeyID(), "test", "hash", "prefix")
	store.keys[key.ID().String()] = key

	err := uc.Revoke(context.Background(), key.ID())
	if err != nil {
		t.Fatalf("Revoke error: %v", err)
	}
	if len(store.keys) != 0 {
		t.Errorf("store has %d keys, want 0", len(store.keys))
	}
}

func TestRevokeAPIKey_NotFound(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	err := uc.Revoke(context.Background(), vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListAPIKeys_Success(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	key1, _ := entity.NewAPIKey(vo.NewAPIKeyID(), "key-1", "hash1", "prefix1")
	key2, _ := entity.NewAPIKey(vo.NewAPIKeyID(), "key-2", "hash2", "prefix2")
	store.keys[key1.ID().String()] = key1
	store.keys[key2.ID().String()] = key2

	result, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List returned %d keys, want 2", len(result))
	}
}

func TestListAPIKeys_Empty(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	result, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("List returned %d keys, want 0", len(result))
	}
}

func TestGenerateRawKey_Format(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	input := CreateAPIKeyInput{Name: "format-check"}
	_, rawKey, err := uc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// rawKey = "omni-" (5) + 64 hex chars = 69 chars total
	if len(rawKey) != 69 {
		t.Errorf("rawKey length = %d, want 69", len(rawKey))
	}
	if !strings.HasPrefix(rawKey, "omni-") {
		t.Errorf("rawKey should start with 'omni-'")
	}

	// Hex portion should be valid hex
	hexPart := rawKey[5:]
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("rawKey hex part contains invalid char %c", c)
			break
		}
	}
}

func TestGenerateRawKey_Uniqueness(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)

	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		input := CreateAPIKeyInput{Name: "unique-check"}
		_, rawKey, err := uc.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create error on iteration %d: %v", i, err)
		}
		if keys[rawKey] {
			t.Fatalf("duplicate rawKey generated: %s", rawKey)
		}
		keys[rawKey] = true
	}
}

func TestCreateAPIKey_GenerateKeyFails(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)
	uc.genRawKey = func() (string, error) {
		return "", errors.New("entropy exhausted")
	}

	_, _, err := uc.Create(context.Background(), CreateAPIKeyInput{Name: "test"})
	if err == nil {
		t.Fatal("expected error when key generation fails")
	}
	if !strings.Contains(err.Error(), "failed to generate key") {
		t.Errorf("error = %q, want to contain 'failed to generate key'", err.Error())
	}
}

func TestCreateAPIKey_HashFails(t *testing.T) {
	store := newMockAPIKeyStore()
	uc := NewAdminAPIKeyUseCase(store)
	uc.hashKey = func(_ string) (string, error) {
		return "", errors.New("hash error")
	}

	_, _, err := uc.Create(context.Background(), CreateAPIKeyInput{Name: "test"})
	if err == nil {
		t.Fatal("expected error when hash fails")
	}
	if !strings.Contains(err.Error(), "failed to hash key") {
		t.Errorf("error = %q, want to contain 'failed to hash key'", err.Error())
	}
}
