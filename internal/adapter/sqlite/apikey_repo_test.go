package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func setupAPIKeyRepo(t *testing.T) *APIKeyRepo {
	t.Helper()
	db := setupTestDB(t)
	return NewAPIKeyRepo(db)
}

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTestAPIKey(t *testing.T, name, hash, prefix string, models []string) *entity.APIKey {
	t.Helper()
	key, err := entity.NewAPIKey(vo.NewAPIKeyID(), name, hash, prefix)
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	if len(models) > 0 {
		key.SetAllowedModels(models)
	}
	return key
}

func TestAPIKeyRepo_Create_And_FindByID(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "test-key", "$2a$10$hash", "omni-abc123", nil)

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, key.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID().String() != key.ID().String() {
		t.Errorf("ID = %q, want %q", found.ID(), key.ID())
	}
	if found.Name() != "test-key" {
		t.Errorf("Name = %q, want %q", found.Name(), "test-key")
	}
	if found.KeyHash() != "$2a$10$hash" {
		t.Errorf("KeyHash = %q", found.KeyHash())
	}
	if found.KeyPrefix() != "omni-abc123" {
		t.Errorf("KeyPrefix = %q", found.KeyPrefix())
	}
	if !found.IsActive() {
		t.Error("should be active")
	}
}

func TestAPIKeyRepo_Create_WithAllowedModels(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	models := []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514"}
	key := createTestAPIKey(t, "models-key", "hash", "prefix12", models)

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, key.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	foundModels := found.AllowedModels()
	if len(foundModels) != 2 {
		t.Fatalf("AllowedModels len = %d, want 2", len(foundModels))
	}
	if foundModels[0] != "claude-sonnet-4-20250514" {
		t.Errorf("AllowedModels[0] = %q", foundModels[0])
	}
	if foundModels[1] != "claude-opus-4-20250514" {
		t.Errorf("AllowedModels[1] = %q", foundModels[1])
	}
}

func TestAPIKeyRepo_FindByID_NotFound(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestAPIKeyRepo_FindAll(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key1 := createTestAPIKey(t, "key1", "hash1", "prefix1_____", nil)
	key2 := createTestAPIKey(t, "key2", "hash2", "prefix2_____", nil)

	repo.Create(ctx, key1)
	repo.Create(ctx, key2)

	keys, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("FindAll len = %d, want 2", len(keys))
	}
}

func TestAPIKeyRepo_FindAll_Empty(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	keys, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("FindAll len = %d, want 0", len(keys))
	}
}

func TestAPIKeyRepo_Delete(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "to-delete", "hash", "prefix______", nil)
	repo.Create(ctx, key)

	if err := repo.Delete(ctx, key.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, key.ID())
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAPIKeyRepo_Delete_NotFound(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error for delete not found")
	}
}

func TestAPIKeyRepo_FindByPrefix(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "find-prefix", "hash", "omni-test123", nil)
	repo.Create(ctx, key)

	found, err := repo.FindByPrefix(ctx, "omni-test123")
	if err != nil {
		t.Fatalf("FindByPrefix: %v", err)
	}

	if found.ID().String() != key.ID().String() {
		t.Errorf("ID = %q, want %q", found.ID(), key.ID())
	}
	if found.Name() != "find-prefix" {
		t.Errorf("Name = %q", found.Name())
	}
}

func TestAPIKeyRepo_FindByPrefix_NotFound(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	_, err := repo.FindByPrefix(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for prefix not found")
	}
}

func TestAPIKeyRepo_FindByPrefix_InactiveKeyNotReturned(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "inactive-key", "hash", "omni-inactiv", nil)
	repo.Create(ctx, key)

	// Deactivate by deleting (our Delete does hard delete, so simulate inactive via direct SQL)
	repo.db.Writer().Exec("UPDATE api_keys SET is_active = 0 WHERE id = ?", key.ID().String())

	_, err := repo.FindByPrefix(ctx, "omni-inactiv")
	if err == nil {
		t.Fatal("expected error — inactive key should not be returned")
	}
}

func TestAPIKeyRepo_AllowedModels_EmptyRoundtrip(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "no-models", "hash", "prefix______", nil)
	repo.Create(ctx, key)

	found, err := repo.FindByID(ctx, key.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	models := found.AllowedModels()
	if len(models) != 0 {
		t.Errorf("AllowedModels = %v, want empty", models)
	}
}
