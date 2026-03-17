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
