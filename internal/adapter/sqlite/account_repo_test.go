package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/sqlite"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// stubEncryptor is a test double that returns plaintext with a prefix.
type stubEncryptor struct{}

func (s *stubEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (s *stubEncryptor) Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) > 4 && ciphertext[:4] == "enc:" {
		return ciphertext[4:], nil
	}
	return ciphertext, nil
}

// failDecryptEncryptor succeeds on Encrypt but always fails on Decrypt.
type failDecryptEncryptor struct{}

func (f *failDecryptEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (f *failDecryptEncryptor) Decrypt(string) (string, error) {
	return "", fmt.Errorf("forced decrypt failure")
}

// failEncryptEncryptor always fails on Encrypt.
type failEncryptEncryptor struct{}

func (f *failEncryptEncryptor) Encrypt(string) (string, error) {
	return "", fmt.Errorf("forced encrypt failure")
}

func (f *failEncryptEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func setupRepo(t *testing.T) (*sqlite.AccountRepo, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	return repo, db.Writer()
}

func makeAccount(t *testing.T) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		"test-account",
		vo.NewSensitiveString("sk-ant-api03-test-key"),
		"https://api.anthropic.com",
		0,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func TestAccountRepo_CreateAndFindByID(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)

	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID().String() != acc.ID().String() {
		t.Errorf("ID = %q; want %q", found.ID(), acc.ID())
	}
	if found.Name() != acc.Name() {
		t.Errorf("Name = %q; want %q", found.Name(), acc.Name())
	}
	if found.BaseURL() != acc.BaseURL() {
		t.Errorf("BaseURL = %q; want %q", found.BaseURL(), acc.BaseURL())
	}
	if found.Status() != entity.StatusActive {
		t.Errorf("Status = %q; want %q", found.Status(), entity.StatusActive)
	}
	if found.Priority() != acc.Priority() {
		t.Errorf("Priority = %d; want %d", found.Priority(), acc.Priority())
	}
}

func TestAccountRepo_EncryptionRoundtrip(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// API key should be decrypted correctly
	if found.Credentials().APIKey.Value() != "sk-ant-api03-test-key" {
		t.Errorf("API key = %q; want %q", found.Credentials().APIKey.Value(), "sk-ant-api03-test-key")
	}
}

func TestAccountRepo_FindAll(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc1 := makeAccount(t)
	acc2, _ := entity.NewAccount(vo.NewAccountID(), "second", vo.NewSensitiveString("sk-2"), "https://api.anthropic.com", 1)

	repo.Create(ctx, acc1)
	repo.Create(ctx, acc2)

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("FindAll returned %d accounts; want 2", len(all))
	}
}

func TestAccountRepo_Save(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	// Rehydrate with updated name
	updated, _ := entity.RehydrateAccount(
		acc.ID(),
		"updated-name",
		vo.NewSensitiveString("sk-ant-api03-new-key"),
		"https://custom.api.com",
		entity.StatusActive,
		5,
		nil,
		nil,
		nil,
	)

	if err := repo.Save(ctx, updated); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, _ := repo.FindByID(ctx, acc.ID())
	if found.Name() != "updated-name" {
		t.Errorf("Name = %q; want %q", found.Name(), "updated-name")
	}
	if found.BaseURL() != "https://custom.api.com" {
		t.Errorf("BaseURL = %q; want %q", found.BaseURL(), "https://custom.api.com")
	}
	if found.Priority() != 5 {
		t.Errorf("Priority = %d; want 5", found.Priority())
	}
	if found.Credentials().APIKey.Value() != "sk-ant-api03-new-key" {
		t.Errorf("API key not updated")
	}
}

func TestAccountRepo_Delete(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	if err := repo.Delete(ctx, acc.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, acc.ID())
	if err == nil {
		t.Error("FindByID after Delete should return error")
	}
}

func TestAccountRepo_FindByID_NotFound(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	id := vo.NewAccountID()
	_, err := repo.FindByID(ctx, id)
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

func TestAccountRepo_FindAvailable(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	// Active account
	acc1 := makeAccount(t)
	repo.Create(ctx, acc1)

	// Disabled account
	acc2, _ := entity.NewAccount(vo.NewAccountID(), "disabled", vo.NewSensitiveString("sk-d"), "https://api.anthropic.com", 0)
	acc2.Disable("test reason")
	repo.Create(ctx, acc2)

	// Account in cooldown (future)
	acc3, _ := entity.NewAccount(vo.NewAccountID(), "cooldown", vo.NewSensitiveString("sk-c"), "https://api.anthropic.com", 0)
	acc3.ApplyCooldown(vo.ErrRateLimit, time.Now())
	repo.Create(ctx, acc3)

	available, err := repo.FindAvailable(ctx, nil)
	if err != nil {
		t.Fatalf("FindAvailable: %v", err)
	}

	// Only acc1 should be available (active, no cooldown)
	if len(available) != 1 {
		t.Fatalf("FindAvailable returned %d; want 1", len(available))
	}
	if available[0].ID().String() != acc1.ID().String() {
		t.Errorf("available[0].ID = %q; want %q", available[0].ID(), acc1.ID())
	}
}

func TestAccountRepo_FindAvailable_ExcludeID(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc1 := makeAccount(t)
	acc2, _ := entity.NewAccount(vo.NewAccountID(), "second", vo.NewSensitiveString("sk-2"), "https://api.anthropic.com", 0)
	repo.Create(ctx, acc1)
	repo.Create(ctx, acc2)

	excludeID := acc1.ID()
	available, err := repo.FindAvailable(ctx, &excludeID)
	if err != nil {
		t.Fatalf("FindAvailable: %v", err)
	}

	if len(available) != 1 {
		t.Fatalf("FindAvailable returned %d; want 1", len(available))
	}
	if available[0].ID().String() != acc2.ID().String() {
		t.Errorf("should exclude acc1, got %q", available[0].ID())
	}
}

func TestAccountRepo_UpdateStatus(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	// Apply cooldown
	now := time.Now()
	acc.ApplyCooldown(vo.ErrRateLimit, now)

	if err := repo.UpdateStatus(ctx, acc); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	found, _ := repo.FindByID(ctx, acc.ID())
	if found.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q; want %q", found.Status(), entity.StatusCooldown)
	}
	if found.BackoffLevel() != 1 {
		t.Errorf("BackoffLevel = %d; want 1", found.BackoffLevel())
	}
	if found.LastError() == nil {
		t.Error("LastError should not be nil")
	}
	ec := found.ErrorClassification()
	if ec == nil || *ec != vo.ErrRateLimit {
		t.Errorf("ErrorClassification = %v; want %q", ec, vo.ErrRateLimit)
	}
}

func TestAccountRepo_Save_WithAllFields(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	// Apply cooldown so account has all nullable fields populated
	now := time.Now()
	acc.ApplyCooldown(vo.ErrRateLimit, now)
	acc.RecordUsage(now)

	// Save with cooldown, lastUsedAt, lastError, errClassification all set
	if err := repo.Save(ctx, acc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q; want cooldown", found.Status())
	}
	if found.CooldownUntil() == nil {
		t.Error("CooldownUntil should not be nil")
	}
	if found.LastUsedAt() == nil {
		t.Error("LastUsedAt should not be nil")
	}
	if found.LastError() == nil {
		t.Error("LastError should not be nil")
	}
	if found.ErrorClassification() == nil {
		t.Error("ErrorClassification should not be nil")
	}
}

func TestAccountRepo_Create_WithCooldownAndUsage(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	now := time.Now()
	acc.ApplyCooldown(vo.ErrOverloaded, now)
	acc.RecordUsage(now)

	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.CooldownUntil() == nil {
		t.Error("CooldownUntil should be set")
	}
	if found.LastUsedAt() == nil {
		t.Error("LastUsedAt should be set")
	}
	if found.LastError() == nil {
		t.Error("LastError should be set")
	}
	ec := found.ErrorClassification()
	if ec == nil || *ec != vo.ErrOverloaded {
		t.Errorf("ErrorClassification = %v; want overloaded", ec)
	}
}

func TestAccountRepo_RecordSuccess(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	// Apply cooldown first
	acc.ApplyCooldown(vo.ErrRateLimit, time.Now())
	repo.UpdateStatus(ctx, acc)

	// Record success
	now := time.Now()
	if err := repo.RecordSuccess(ctx, acc.ID(), now); err != nil {
		t.Fatalf("RecordSuccess: %v", err)
	}

	found, _ := repo.FindByID(ctx, acc.ID())
	if found.Status() != entity.StatusActive {
		t.Errorf("Status = %q; want %q", found.Status(), entity.StatusActive)
	}
	if found.BackoffLevel() != 0 {
		t.Errorf("BackoffLevel = %d; want 0", found.BackoffLevel())
	}
	if found.LastError() != nil {
		t.Errorf("LastError should be nil, got %q", *found.LastError())
	}
	if found.LastUsedAt() == nil {
		t.Error("LastUsedAt should not be nil")
	}
}

// --- Corrupted data tests ---

func TestAccountRepo_FindByID_DecryptError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert via raw SQL
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, backoff_level, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"acc_aaaaaaaaaaaaaaaaaaaaa", "test", "some-encrypted-key", "https://api.anthropic.com",
		"active", 0, 0, "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Use failDecryptEncryptor
	repo := sqlite.NewAccountRepo(db, &failDecryptEncryptor{})
	ctx := context.Background()

	id, _ := vo.ParseAccountID("acc_aaaaaaaaaaaaaaaaaaaaa")
	_, err = repo.FindByID(ctx, id)
	if err == nil {
		t.Fatal("expected decrypt error")
	}
}

func TestAccountRepo_FindByID_InvalidID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert row with invalid ID directly via SQL
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, backoff_level, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad-id", "test", "enc:sk-key", "https://api.anthropic.com",
		"active", 0, 0, "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	ctx := context.Background()

	// Query by raw SQL to get the row with bad ID
	rows, err := db.Reader().QueryContext(ctx,
		`SELECT id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification
		 FROM accounts WHERE id = 'bad-id'`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	// Use FindAll which will attempt to rehydrate the bad ID
	_, err = repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected parse account id error")
	}
}

func TestAccountRepo_FindAll_InvalidCooldownUntil(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert with valid ID but invalid cooldown_until format (must also have error_classification for the branch)
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"acc_bbbbbbbbbbbbbbbbbbbbb", "test", "enc:sk-key", "https://api.anthropic.com",
		"cooldown", 0, "not-a-date", 1, nil, "rate_limit", "rate_limit",
		"2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	ctx := context.Background()

	_, err = repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected parse cooldown_until error")
	}
}

func TestAccountRepo_FindAll_InvalidLastUsedAt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert with valid ID but invalid last_used_at format
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, backoff_level, last_used_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"acc_ccccccccccccccccccccc", "test", "enc:sk-key", "https://api.anthropic.com",
		"active", 0, 0, "not-a-date",
		"2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	ctx := context.Background()

	_, err = repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected parse last_used_at error")
	}
}

func TestAccountRepo_Create_EncryptError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &failEncryptEncryptor{})
	ctx := context.Background()

	acc := makeAccount(t)
	err = repo.Create(ctx, acc)
	if err == nil {
		t.Fatal("expected encrypt error on Create")
	}
}

func TestAccountRepo_Save_EncryptError(t *testing.T) {
	// First create with normal encryptor, then save with failing one
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Create with normal encryptor
	goodRepo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	ctx := context.Background()
	acc := makeAccount(t)
	goodRepo.Create(ctx, acc)

	// Save with failing encryptor
	badRepo := sqlite.NewAccountRepo(db, &failEncryptEncryptor{})
	err = badRepo.Save(ctx, acc)
	if err == nil {
		t.Fatal("expected encrypt error on Save")
	}
}

func TestAccountRepo_FindAll_DecryptError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert valid row
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, backoff_level, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"acc_ddddddddddddddddddddd", "test", "enc:sk-key", "https://api.anthropic.com",
		"active", 0, 0, "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Use failDecryptEncryptor — exercises scanAccounts → rehydrate → decrypt error
	repo := sqlite.NewAccountRepo(db, &failDecryptEncryptor{})
	ctx := context.Background()

	_, err = repo.FindAll(ctx)
	if err == nil {
		t.Fatal("expected decrypt error on FindAll")
	}
}

func TestAccountRepo_Delete_ClosedDB(t *testing.T) {
	repo, writer := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	writer.Close()

	err := repo.Delete(ctx, acc.ID())
	if err == nil {
		t.Fatal("expected error for closed DB on Delete")
	}
}

func TestAccountRepo_RecordSuccess_ClosedDB(t *testing.T) {
	repo, writer := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	writer.Close()

	err := repo.RecordSuccess(ctx, acc.ID(), time.Now())
	if err == nil {
		t.Fatal("expected error for closed DB on RecordSuccess")
	}
}

func TestAccountRepo_FindAll_ClosedDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	db.Close()

	_, err = repo.FindAll(context.Background())
	if err == nil {
		t.Fatal("expected error for closed DB on FindAll")
	}
}

func TestAccountRepo_FindAvailable_ClosedDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	db.Close()

	_, err = repo.FindAvailable(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for closed DB on FindAvailable")
	}
}

func TestAccountRepo_UpdateStatus_ClosedDB(t *testing.T) {
	repo, writer := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)
	acc.ApplyCooldown(vo.ErrRateLimit, time.Now())

	writer.Close()

	err := repo.UpdateStatus(ctx, acc)
	if err == nil {
		t.Fatal("expected error for closed DB on UpdateStatus")
	}
}

func TestAccountRepo_Create_ClosedDB(t *testing.T) {
	repo, writer := setupRepo(t)
	ctx := context.Background()

	writer.Close()

	acc := makeAccount(t)
	err := repo.Create(ctx, acc)
	if err == nil {
		t.Fatal("expected error for closed DB on Create")
	}
}

func TestAccountRepo_Save_ClosedDB(t *testing.T) {
	repo, writer := setupRepo(t)
	ctx := context.Background()

	acc := makeAccount(t)
	repo.Create(ctx, acc)

	writer.Close()

	err := repo.Save(ctx, acc)
	if err == nil {
		t.Fatal("expected error for closed DB on Save")
	}
}

func TestAccountRepo_FindByID_ScanError(t *testing.T) {
	// Drop the accounts table and recreate with fewer columns
	// to trigger a scan error (not ErrNoRows).
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create a minimal table that has the same name but different columns
	_, err = db.Writer().Exec(`CREATE TABLE accounts (id TEXT PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Writer().Exec(`INSERT INTO accounts (id, name) VALUES ('acc_eeeeeeeeeeeeeeeeeeeee', 'test')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	id, _ := vo.ParseAccountID("acc_eeeeeeeeeeeeeeeeeeeee")
	_, err = repo.FindByID(context.Background(), id)
	if err == nil {
		t.Fatal("expected scan error for mismatched schema")
	}
}

func TestAccountRepo_FindAll_ScanRowError(t *testing.T) {
	// Force rows.Scan error inside scanAccounts by inserting a row
	// where priority contains non-numeric text (scan into int fails).
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// SQLite is dynamically typed — INSERT "abc" into INTEGER column succeeds,
	// but Go's rows.Scan into int fails.
	_, err = db.Writer().Exec(
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, backoff_level, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"acc_fffffffffffffffffffff", "test", "enc:sk-key", "https://api.anthropic.com",
		"active", "not-a-number", 0, "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := sqlite.NewAccountRepo(db, &stubEncryptor{})
	_, err = repo.FindAll(context.Background())
	if err == nil {
		t.Fatal("expected scan error for incompatible priority type")
	}
}
