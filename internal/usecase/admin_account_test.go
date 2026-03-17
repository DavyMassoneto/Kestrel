package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

var testNow = time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)

type mockAccountStore struct {
	accounts map[string]*entity.Account
	createFn func(ctx context.Context, account *entity.Account) error
	saveFn   func(ctx context.Context, account *entity.Account) error
	deleteFn func(ctx context.Context, id vo.AccountID) error
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{accounts: make(map[string]*entity.Account)}
}

func (m *mockAccountStore) FindByID(_ context.Context, id vo.AccountID) (*entity.Account, error) {
	acc, ok := m.accounts[id.String()]
	if !ok {
		return nil, errors.New("account not found")
	}
	return acc, nil
}

func (m *mockAccountStore) FindAll(_ context.Context) ([]*entity.Account, error) {
	var result []*entity.Account
	for _, acc := range m.accounts {
		result = append(result, acc)
	}
	return result, nil
}

func (m *mockAccountStore) Create(ctx context.Context, account *entity.Account) error {
	if m.createFn != nil {
		return m.createFn(ctx, account)
	}
	m.accounts[account.ID().String()] = account
	return nil
}

func (m *mockAccountStore) Save(ctx context.Context, account *entity.Account) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, account)
	}
	m.accounts[account.ID().String()] = account
	return nil
}

func (m *mockAccountStore) Delete(_ context.Context, id vo.AccountID) error {
	if m.deleteFn != nil {
		return m.deleteFn(context.Background(), id)
	}
	delete(m.accounts, id.String())
	return nil
}

func TestCreateAccount_Success(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	input := CreateAccountInput{
		Name:     "test-account",
		APIKey:   "sk-ant-api03-test",
		BaseURL:  "https://api.anthropic.com",
		Priority: 1,
	}

	acc, err := uc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if acc.Name() != "test-account" {
		t.Errorf("Name = %q, want %q", acc.Name(), "test-account")
	}
	if acc.BaseURL() != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q", acc.BaseURL())
	}
	if acc.Priority() != 1 {
		t.Errorf("Priority = %d, want 1", acc.Priority())
	}
	if acc.Status() != entity.StatusActive {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusActive)
	}
	if acc.ID().String() == "" {
		t.Error("ID should not be empty")
	}

	// Verify it was persisted
	if len(store.accounts) != 1 {
		t.Errorf("store has %d accounts, want 1", len(store.accounts))
	}
}

func TestCreateAccount_EmptyName(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	input := CreateAccountInput{
		Name:    "",
		APIKey:  "sk-ant-api03-test",
		BaseURL: "https://api.anthropic.com",
	}

	_, err := uc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateAccount_EmptyAPIKey(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	input := CreateAccountInput{
		Name:    "test",
		APIKey:  "",
		BaseURL: "https://api.anthropic.com",
	}

	_, err := uc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestCreateAccount_DefaultBaseURL(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	input := CreateAccountInput{
		Name:   "test",
		APIKey: "sk-ant-api03-test",
	}

	acc, err := uc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if acc.BaseURL() != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q, want default", acc.BaseURL())
	}
}

func TestCreateAccount_StoreFails(t *testing.T) {
	store := newMockAccountStore()
	store.createFn = func(_ context.Context, _ *entity.Account) error {
		return errors.New("db error")
	}
	uc := NewAdminAccountUseCase(store)

	input := CreateAccountInput{
		Name:    "test",
		APIKey:  "sk-ant-api03-test",
		BaseURL: "https://api.anthropic.com",
	}

	_, err := uc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateAccount_PartialName(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	// Create an account first
	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"original",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newName := "updated-name"
	input := UpdateAccountInput{Name: &newName}

	updated, err := uc.Update(context.Background(), acc.ID(), input)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Name() != "updated-name" {
		t.Errorf("Name = %q, want %q", updated.Name(), "updated-name")
	}
	// Other fields should remain unchanged
	if updated.BaseURL() != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q, should remain unchanged", updated.BaseURL())
	}
	if updated.Priority() != 1 {
		t.Errorf("Priority = %d, should remain 1", updated.Priority())
	}
}

func TestUpdateAccount_PartialPriority(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newPriority := 5
	input := UpdateAccountInput{Priority: &newPriority}

	updated, err := uc.Update(context.Background(), acc.ID(), input)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Priority() != 5 {
		t.Errorf("Priority = %d, want 5", updated.Priority())
	}
	if updated.Name() != "test" {
		t.Errorf("Name = %q, should remain unchanged", updated.Name())
	}
}

func TestUpdateAccount_NotFound(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	id := vo.NewAccountID()
	newName := "updated"
	input := UpdateAccountInput{Name: &newName}

	_, err := uc.Update(context.Background(), id, input)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestDeleteAccount_Success(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	err := uc.Delete(context.Background(), acc.ID())
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if len(store.accounts) != 0 {
		t.Errorf("store has %d accounts, want 0", len(store.accounts))
	}
}

func TestResetAccount_Success(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	// Put account in cooldown
	acc.ApplyCooldown(vo.ErrRateLimit, testNow)
	store.accounts[acc.ID().String()] = acc

	if acc.Status() != entity.StatusCooldown {
		t.Fatalf("precondition: Status = %q, want cooldown", acc.Status())
	}

	reset, err := uc.Reset(context.Background(), acc.ID())
	if err != nil {
		t.Fatalf("Reset error: %v", err)
	}
	if reset.Status() != entity.StatusActive {
		t.Errorf("Status = %q, want %q", reset.Status(), entity.StatusActive)
	}
	if reset.BackoffLevel() != 0 {
		t.Errorf("BackoffLevel = %d, want 0", reset.BackoffLevel())
	}
	if reset.LastError() != nil {
		t.Errorf("LastError should be nil, got %v", reset.LastError())
	}
}

func TestResetAccount_NotFound(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	_, err := uc.Reset(context.Background(), vo.NewAccountID())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestResetAccount_SaveFails(t *testing.T) {
	store := newMockAccountStore()
	store.saveFn = func(_ context.Context, _ *entity.Account) error {
		return errors.New("db error")
	}
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	acc.ApplyCooldown(vo.ErrRateLimit, testNow)
	store.accounts[acc.ID().String()] = acc

	_, err := uc.Reset(context.Background(), acc.ID())
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

func TestListAccounts_Success(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc1, _ := entity.NewAccount(vo.NewAccountID(), "acc-1", vo.NewSensitiveString("sk-1"), "https://api.anthropic.com", 1)
	acc2, _ := entity.NewAccount(vo.NewAccountID(), "acc-2", vo.NewSensitiveString("sk-2"), "https://api.anthropic.com", 2)
	store.accounts[acc1.ID().String()] = acc1
	store.accounts[acc2.ID().String()] = acc2

	result, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List returned %d accounts, want 2", len(result))
	}
}

func TestListAccounts_Empty(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	result, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("List returned %d accounts, want 0", len(result))
	}
}

func TestUpdateAccount_PartialAPIKey(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-old-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newKey := "sk-new-key"
	input := UpdateAccountInput{APIKey: &newKey}

	updated, err := uc.Update(context.Background(), acc.ID(), input)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Credentials().APIKey.Value() != "sk-new-key" {
		t.Errorf("APIKey = %q, want %q", updated.Credentials().APIKey.Value(), "sk-new-key")
	}
	if updated.Name() != "test" {
		t.Errorf("Name = %q, should remain unchanged", updated.Name())
	}
}

func TestUpdateAccount_PartialBaseURL(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newURL := "https://custom.api.example.com"
	input := UpdateAccountInput{BaseURL: &newURL}

	updated, err := uc.Update(context.Background(), acc.ID(), input)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.BaseURL() != "https://custom.api.example.com" {
		t.Errorf("BaseURL = %q, want %q", updated.BaseURL(), "https://custom.api.example.com")
	}
	if updated.Name() != "test" {
		t.Errorf("Name = %q, should remain unchanged", updated.Name())
	}
}

func TestUpdateAccount_SaveFails(t *testing.T) {
	store := newMockAccountStore()
	store.saveFn = func(_ context.Context, _ *entity.Account) error {
		return errors.New("db error")
	}
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newName := "updated"
	input := UpdateAccountInput{Name: &newName}

	_, err := uc.Update(context.Background(), acc.ID(), input)
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

func TestUpdateAccount_RehydrateValidationFails(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"test",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	emptyName := ""
	input := UpdateAccountInput{Name: &emptyName}

	_, err := uc.Update(context.Background(), acc.ID(), input)
	if err == nil {
		t.Fatal("expected error when rehydrate validation fails")
	}
}

func TestUpdateAccount_AllFields(t *testing.T) {
	store := newMockAccountStore()
	uc := NewAdminAccountUseCase(store)

	acc, _ := entity.NewAccount(
		vo.NewAccountID(),
		"original",
		vo.NewSensitiveString("sk-old"),
		"https://api.anthropic.com",
		1,
	)
	store.accounts[acc.ID().String()] = acc

	newName := "updated"
	newKey := "sk-new"
	newURL := "https://custom.api.com"
	newPriority := 10
	input := UpdateAccountInput{
		Name:     &newName,
		APIKey:   &newKey,
		BaseURL:  &newURL,
		Priority: &newPriority,
	}

	updated, err := uc.Update(context.Background(), acc.ID(), input)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Name() != "updated" {
		t.Errorf("Name = %q, want %q", updated.Name(), "updated")
	}
	if updated.Credentials().APIKey.Value() != "sk-new" {
		t.Errorf("APIKey = %q, want %q", updated.Credentials().APIKey.Value(), "sk-new")
	}
	if updated.BaseURL() != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want %q", updated.BaseURL(), "https://custom.api.com")
	}
	if updated.Priority() != 10 {
		t.Errorf("Priority = %d, want 10", updated.Priority())
	}
}
