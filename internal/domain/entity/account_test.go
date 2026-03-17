package entity_test

import (
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func newTestAccount(t *testing.T) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		"test-account",
		vo.NewSensitiveString("sk-ant-test-key"),
		"https://api.anthropic.com",
		1,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func TestNewAccount_Valid(t *testing.T) {
	id := vo.NewAccountID()
	acc, err := entity.NewAccount(
		id,
		"my-account",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID() != id {
		t.Errorf("ID = %v; want %v", acc.ID(), id)
	}
	if acc.Name() != "my-account" {
		t.Errorf("Name = %q; want %q", acc.Name(), "my-account")
	}
	if acc.BaseURL() != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q; want %q", acc.BaseURL(), "https://api.anthropic.com")
	}
	if acc.Status() != entity.StatusActive {
		t.Errorf("Status = %q; want %q", acc.Status(), entity.StatusActive)
	}
	if acc.Priority() != 0 {
		t.Errorf("Priority = %d; want 0", acc.Priority())
	}
	if acc.CooldownUntil() != nil {
		t.Error("CooldownUntil should be nil")
	}
	if acc.BackoffLevel() != 0 {
		t.Errorf("BackoffLevel = %d; want 0", acc.BackoffLevel())
	}
	if acc.LastUsedAt() != nil {
		t.Error("LastUsedAt should be nil")
	}
	if acc.LastError() != nil {
		t.Error("LastError should be nil")
	}
	if acc.ErrorClassification() != nil {
		t.Error("ErrorClassification should be nil")
	}
}

func TestNewAccount_EmptyName(t *testing.T) {
	_, err := entity.NewAccount(vo.NewAccountID(), "", vo.NewSensitiveString("key"), "https://api.anthropic.com", 1)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNewAccount_EmptyAPIKey(t *testing.T) {
	_, err := entity.NewAccount(vo.NewAccountID(), "acc", vo.NewSensitiveString(""), "https://api.anthropic.com", 1)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestNewAccount_EmptyBaseURL(t *testing.T) {
	_, err := entity.NewAccount(vo.NewAccountID(), "acc", vo.NewSensitiveString("key"), "", 1)
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
}

func TestAccount_Credentials(t *testing.T) {
	acc := newTestAccount(t)
	creds := acc.Credentials()

	if creds.APIKey.Value() != "sk-ant-test-key" {
		t.Errorf("APIKey = %q; want %q", creds.APIKey.Value(), "sk-ant-test-key")
	}
	if creds.BaseURL != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q; want %q", creds.BaseURL, "https://api.anthropic.com")
	}
}

func TestAccount_ApplyCooldown_RateLimit(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	err := acc.ApplyCooldown(vo.ErrRateLimit, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q; want %q", acc.Status(), entity.StatusCooldown)
	}
	if acc.CooldownUntil() == nil {
		t.Fatal("CooldownUntil should not be nil")
	}
	if acc.BackoffLevel() != 1 {
		t.Errorf("BackoffLevel = %d; want 1", acc.BackoffLevel())
	}
	ec := acc.ErrorClassification()
	if ec == nil || *ec != vo.ErrRateLimit {
		t.Errorf("ErrorClassification = %v; want %v", ec, vo.ErrRateLimit)
	}
}

func TestAccount_ApplyCooldown_Exponential(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	// Apply multiple rate limit cooldowns and verify backoff increases
	_ = acc.ApplyCooldown(vo.ErrRateLimit, now)
	if acc.BackoffLevel() != 1 {
		t.Errorf("BackoffLevel after 1st = %d; want 1", acc.BackoffLevel())
	}

	_ = acc.ApplyCooldown(vo.ErrRateLimit, now)
	if acc.BackoffLevel() != 2 {
		t.Errorf("BackoffLevel after 2nd = %d; want 2", acc.BackoffLevel())
	}

	_ = acc.ApplyCooldown(vo.ErrRateLimit, now)
	if acc.BackoffLevel() != 3 {
		t.Errorf("BackoffLevel after 3rd = %d; want 3", acc.BackoffLevel())
	}
}

func TestAccount_ApplyCooldown_QuotaExhausted(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	err := acc.ApplyCooldown(vo.ErrQuotaExhausted, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := now.Add(5 * time.Minute)
	if !acc.CooldownUntil().Equal(expected) {
		t.Errorf("CooldownUntil = %v; want %v", acc.CooldownUntil(), expected)
	}
}

func TestAccount_ApplyCooldown_ServerError(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	err := acc.ApplyCooldown(vo.ErrServer, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := now.Add(60 * time.Second)
	if !acc.CooldownUntil().Equal(expected) {
		t.Errorf("CooldownUntil = %v; want %v", acc.CooldownUntil(), expected)
	}
}

func TestAccount_ApplyCooldown_Overloaded(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	err := acc.ApplyCooldown(vo.ErrOverloaded, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := now.Add(30 * time.Second)
	if !acc.CooldownUntil().Equal(expected) {
		t.Errorf("CooldownUntil = %v; want %v", acc.CooldownUntil(), expected)
	}
}

func TestAccount_ApplyCooldown_RejectsAuth(t *testing.T) {
	acc := newTestAccount(t)
	err := acc.ApplyCooldown(vo.ErrAuth, time.Now())
	if err == nil {
		t.Fatal("expected error for auth classification")
	}
}

func TestAccount_ApplyCooldown_RejectsClient(t *testing.T) {
	acc := newTestAccount(t)
	err := acc.ApplyCooldown(vo.ErrClient, time.Now())
	if err == nil {
		t.Fatal("expected error for client classification")
	}
}

func TestAccount_ApplyCooldown_RejectsUnknown(t *testing.T) {
	acc := newTestAccount(t)
	err := acc.ApplyCooldown(vo.ErrUnknown, time.Now())
	if err == nil {
		t.Fatal("expected error for unknown classification")
	}
}

func TestAccount_ClearError(t *testing.T) {
	acc := newTestAccount(t)
	_ = acc.ApplyCooldown(vo.ErrRateLimit, time.Now())

	acc.ClearError()

	if acc.Status() != entity.StatusActive {
		t.Errorf("Status = %q; want %q", acc.Status(), entity.StatusActive)
	}
	if acc.CooldownUntil() != nil {
		t.Error("CooldownUntil should be nil after ClearError")
	}
	if acc.BackoffLevel() != 0 {
		t.Errorf("BackoffLevel = %d; want 0", acc.BackoffLevel())
	}
	if acc.LastError() != nil {
		t.Error("LastError should be nil after ClearError")
	}
	if acc.ErrorClassification() != nil {
		t.Error("ErrorClassification should be nil after ClearError")
	}
}

func TestAccount_Disable(t *testing.T) {
	acc := newTestAccount(t)
	acc.Disable("auth failure")

	if acc.Status() != entity.StatusDisabled {
		t.Errorf("Status = %q; want %q", acc.Status(), entity.StatusDisabled)
	}
	if acc.LastError() == nil || *acc.LastError() != "auth failure" {
		t.Errorf("LastError = %v; want %q", acc.LastError(), "auth failure")
	}
}

func TestAccount_IsAvailable_Active(t *testing.T) {
	acc := newTestAccount(t)
	if !acc.IsAvailable(time.Now()) {
		t.Error("active account should be available")
	}
}

func TestAccount_IsAvailable_Disabled(t *testing.T) {
	acc := newTestAccount(t)
	acc.Disable("test")

	if acc.IsAvailable(time.Now()) {
		t.Error("disabled account should not be available")
	}
}

func TestAccount_IsAvailable_CooldownActive(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()
	_ = acc.ApplyCooldown(vo.ErrRateLimit, now)

	if acc.IsAvailable(now) {
		t.Error("account in active cooldown should not be available")
	}
}

func TestAccount_IsAvailable_CooldownExpired(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()
	_ = acc.ApplyCooldown(vo.ErrRateLimit, now)

	future := now.Add(10 * time.Minute)
	if !acc.IsAvailable(future) {
		t.Error("account with expired cooldown should be available")
	}
}

func TestAccount_RecordUsage(t *testing.T) {
	acc := newTestAccount(t)
	now := time.Now()

	acc.RecordUsage(now)

	if acc.LastUsedAt() == nil {
		t.Fatal("LastUsedAt should not be nil after RecordUsage")
	}
	if !acc.LastUsedAt().Equal(now) {
		t.Errorf("LastUsedAt = %v; want %v", acc.LastUsedAt(), now)
	}
}

func TestAccount_ApplyCooldown_SetsLastError(t *testing.T) {
	acc := newTestAccount(t)
	_ = acc.ApplyCooldown(vo.ErrRateLimit, time.Now())

	if acc.LastError() == nil {
		t.Fatal("LastError should not be nil after ApplyCooldown")
	}
}

func TestRehydrateAccount_Valid(t *testing.T) {
	id := vo.NewAccountID()
	now := time.Now()
	cd := vo.NewCooldown(now.Add(30*time.Second), 2, vo.ErrRateLimit)
	lastErr := "rate limited"

	acc, err := entity.RehydrateAccount(
		id,
		"restored",
		vo.NewSensitiveString("sk-key"),
		"https://api.anthropic.com",
		entity.StatusCooldown,
		3,
		&cd,
		&now,
		&lastErr,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID() != id {
		t.Errorf("ID = %v; want %v", acc.ID(), id)
	}
	if acc.Name() != "restored" {
		t.Errorf("Name = %q; want %q", acc.Name(), "restored")
	}
	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q; want %q", acc.Status(), entity.StatusCooldown)
	}
	if acc.Priority() != 3 {
		t.Errorf("Priority = %d; want 3", acc.Priority())
	}
	if acc.BackoffLevel() != 2 {
		t.Errorf("BackoffLevel = %d; want 2", acc.BackoffLevel())
	}
	if acc.LastUsedAt() == nil || !acc.LastUsedAt().Equal(now) {
		t.Errorf("LastUsedAt = %v; want %v", acc.LastUsedAt(), now)
	}
	if acc.LastError() == nil || *acc.LastError() != lastErr {
		t.Errorf("LastError = %v; want %q", acc.LastError(), lastErr)
	}
}

func TestRehydrateAccount_EmptyName(t *testing.T) {
	_, err := entity.RehydrateAccount(
		vo.NewAccountID(), "", vo.NewSensitiveString("key"), "https://api.anthropic.com",
		entity.StatusActive, 1, nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRehydrateAccount_EmptyAPIKey(t *testing.T) {
	_, err := entity.RehydrateAccount(
		vo.NewAccountID(), "acc", vo.NewSensitiveString(""), "https://api.anthropic.com",
		entity.StatusActive, 1, nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestRehydrateAccount_EmptyBaseURL(t *testing.T) {
	_, err := entity.RehydrateAccount(
		vo.NewAccountID(), "acc", vo.NewSensitiveString("key"), "",
		entity.StatusActive, 1, nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
}
