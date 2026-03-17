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

// --- Error path tests via corrupted data and closed DB ---

func TestAPIKeyRepo_FindByID_InvalidID(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Insert a row with an invalid ID format via raw SQL
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES ('bad-id', 'hash', 'prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z')`)

	_, err := repo.FindByID(ctx, vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error for nonexistent valid ID")
	}

	// Query by raw SQL prefix to hit buildKey with invalid ID
	row := repo.db.Reader().QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys WHERE id = 'bad-id'`)
	_, err = repo.scanKey(row)
	if err == nil {
		t.Fatal("expected error for invalid ID format in scanKey")
	}
}

func TestAPIKeyRepo_FindByID_EmptyName(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Insert row with empty name — will fail NewAPIKey validation in buildKey
	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES (?, 'hash', 'prefix', '', 1, NULL, '2026-01-01T00:00:00Z')`, validID)

	row := repo.db.Reader().QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys WHERE id = ?`, validID)
	_, err := repo.scanKey(row)
	if err == nil {
		t.Fatal("expected error for empty name in buildKey")
	}
}

func TestAPIKeyRepo_FindByID_CorruptedModelsJSON(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES (?, 'hash', 'prefix', 'test', 1, 'not-valid-json', '2026-01-01T00:00:00Z')`, validID)

	row := repo.db.Reader().QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys WHERE id = ?`, validID)
	_, err := repo.scanKey(row)
	if err == nil {
		t.Fatal("expected error for corrupted allowed_models JSON")
	}
}

func TestAPIKeyRepo_FindByID_InvalidLastUsedAt(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at, last_used_at)
		VALUES (?, 'hash', 'prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z', 'not-a-date')`, validID)

	parsedID, parseErr := vo.ParseAPIKeyID(validID)
	if parseErr != nil {
		t.Fatalf("ParseAPIKeyID: %v", parseErr)
	}

	// lastUsedAt parse failure is silently ignored, so key is returned successfully
	found, err := repo.FindByID(ctx, parsedID)
	if err != nil {
		t.Fatalf("FindByID should succeed even with invalid lastUsedAt: %v", err)
	}
	if found.LastUsedAt() != nil {
		t.Error("LastUsedAt should be nil when date format is invalid")
	}
}

func TestAPIKeyRepo_FindAll_CorruptedRow(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Insert a corrupted row (invalid ID format)
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES ('bad-id', 'hash', 'prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z')`)

	_, err := repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected error for corrupted row in FindAll")
	}
}

func TestAPIKeyRepo_FindAll_CorruptedModelsJSON(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES (?, 'hash', 'prefix', 'test', 1, '{invalid', '2026-01-01T00:00:00Z')`, validID)

	_, err := repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected error for corrupted JSON in FindAll")
	}
}

func TestAPIKeyRepo_Create_Duplicate(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "dup-key", "hash", "prefix______", nil)
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Second insert with same ID should fail
	err := repo.Create(ctx, key)
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}
}

func TestAPIKeyRepo_FindAll_DBClosed(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := NewAPIKeyRepo(db)
	db.Close()

	_, err = repo.FindAll(context.Background())
	if err == nil {
		t.Fatal("expected error for closed DB in FindAll")
	}
}

func TestAPIKeyRepo_Create_DBClosed(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := NewAPIKeyRepo(db)
	db.Close()

	key := createTestAPIKey(t, "closed-db", "hash", "prefix______", nil)
	err = repo.Create(context.Background(), key)
	if err == nil {
		t.Fatal("expected error for closed DB in Create")
	}
}

func TestAPIKeyRepo_Delete_DBClosed(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := NewAPIKeyRepo(db)
	db.Close()

	err = repo.Delete(context.Background(), vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error for closed DB in Delete")
	}
}

func TestAPIKeyRepo_FindByPrefix_ScanError(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Insert with invalid ID to trigger scan→buildKey error path
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES ('bad-id', 'hash', 'bad-prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z')`)

	_, err := repo.FindByPrefix(ctx, "bad-prefix")
	if err == nil {
		t.Fatal("expected error for invalid ID in FindByPrefix")
	}
}

func TestAPIKeyRepo_ScanKey_NonErrNoRows(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Drop table to cause scan error that isn't ErrNoRows
	repo.db.Writer().ExecContext(ctx, `DROP TABLE api_keys`)

	_, err := repo.FindByID(ctx, vo.NewAPIKeyID())
	if err == nil {
		t.Fatal("expected error when table is dropped")
	}
}

func TestAPIKeyRepo_FindAll_BuildKeyError(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Insert a row with empty name that passes scan but fails NewAPIKey validation in buildKey
	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES (?, 'hash', 'prefix', '', 1, NULL, '2026-01-01T00:00:00Z')`, validID)

	_, err := repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected error for empty name in FindAll buildKey path")
	}
}

func TestAPIKeyRepo_BuildKey_EmptyHash(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at)
		VALUES (?, '', 'prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z')`, validID)

	parsedID, _ := vo.ParseAPIKeyID(validID)
	_, err := repo.FindByID(ctx, parsedID)
	if err == nil {
		t.Fatal("expected error for empty key_hash in buildKey")
	}
}

func TestAPIKeyRepo_FindAll_ScanError(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	// Replace api_keys with a view that returns incompatible column count
	// causing rows.Scan to fail
	repo.db.Writer().ExecContext(ctx, `DROP TABLE api_keys`)
	repo.db.Writer().ExecContext(ctx,
		`CREATE TABLE api_keys (id TEXT, key_hash TEXT, key_prefix TEXT, name TEXT, is_active INTEGER, allowed_models TEXT, last_used_at TEXT, created_at TEXT, extra1 TEXT)`)
	// Insert row — the SELECT asks for 7 columns from a 9-col table, which works
	// But we use a VIEW to inject wrong types
	repo.db.Writer().ExecContext(ctx, `DROP TABLE api_keys`)
	repo.db.Writer().ExecContext(ctx,
		`CREATE TABLE api_keys_raw (id TEXT, key_hash TEXT, key_prefix TEXT, name TEXT, is_active TEXT, allowed_models TEXT, last_used_at TEXT, created_at TEXT)`)
	repo.db.Writer().ExecContext(ctx,
		`CREATE VIEW api_keys AS SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at FROM api_keys_raw`)
	// Insert with is_active as a non-boolean blob that SQLite will not coerce to bool during scan
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys_raw VALUES ('x', 'h', 'p', 'n', X'DEADBEEF', NULL, NULL, '2026-01-01')`)

	_, err := repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected error for scan type mismatch in FindAll")
	}
}

func TestAPIKeyRepo_FindByID_ValidLastUsedAt(t *testing.T) {
	repo := setupAPIKeyRepo(t)
	ctx := context.Background()

	validID := vo.NewAPIKeyID().String()
	repo.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at, last_used_at)
		VALUES (?, 'hash', 'prefix', 'test', 1, NULL, '2026-01-01T00:00:00Z', '2026-03-15T10:30:00Z')`, validID)

	parsedID, _ := vo.ParseAPIKeyID(validID)
	found, err := repo.FindByID(ctx, parsedID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.LastUsedAt() == nil {
		t.Error("LastUsedAt should not be nil for valid date")
	}
}
