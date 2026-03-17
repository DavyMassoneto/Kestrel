package sqlite_test

import (
	"context"
	"database/sql"
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
