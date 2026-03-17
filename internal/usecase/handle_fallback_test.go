package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type mockAccountStatusWriter struct {
	updateStatusCalls []*entity.Account
	updateStatusErr   error
	recordSuccessCalls []vo.AccountID
	recordSuccessErr   error
}

func (m *mockAccountStatusWriter) UpdateStatus(_ context.Context, account *entity.Account) error {
	m.updateStatusCalls = append(m.updateStatusCalls, account)
	return m.updateStatusErr
}

func (m *mockAccountStatusWriter) RecordSuccess(_ context.Context, id vo.AccountID, _ time.Time) error {
	m.recordSuccessCalls = append(m.recordSuccessCalls, id)
	return m.recordSuccessErr
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time { return c.now }

var fallbackNow = time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)

func newTestAccount(t *testing.T) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		"test-account",
		vo.NewSensitiveString("sk-ant-key"),
		"https://api.anthropic.com",
		1,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func TestHandleFallback_RateLimit(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrRateLimit)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.ShouldFallback {
		t.Error("ShouldFallback should be true for rate_limit")
	}
	if result.Classification != vo.ErrRateLimit {
		t.Errorf("Classification = %q, want %q", result.Classification, vo.ErrRateLimit)
	}
	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusCooldown)
	}
	if len(writer.updateStatusCalls) != 1 {
		t.Fatalf("UpdateStatus called %d times, want 1", len(writer.updateStatusCalls))
	}
}

func TestHandleFallback_QuotaExhausted(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrQuotaExhausted)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.ShouldFallback {
		t.Error("ShouldFallback should be true for quota_exhausted")
	}
	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusCooldown)
	}
	if len(writer.updateStatusCalls) != 1 {
		t.Fatalf("UpdateStatus called %d times, want 1", len(writer.updateStatusCalls))
	}
}

func TestHandleFallback_Overloaded(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrOverloaded)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.ShouldFallback {
		t.Error("ShouldFallback should be true for overloaded")
	}
	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusCooldown)
	}
	if len(writer.updateStatusCalls) != 1 {
		t.Fatalf("UpdateStatus called %d times, want 1", len(writer.updateStatusCalls))
	}
}

func TestHandleFallback_ServerError(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrServer)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.ShouldFallback {
		t.Error("ShouldFallback should be true for server_error")
	}
	if acc.Status() != entity.StatusCooldown {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusCooldown)
	}
	if len(writer.updateStatusCalls) != 1 {
		t.Fatalf("UpdateStatus called %d times, want 1", len(writer.updateStatusCalls))
	}
}

func TestHandleFallback_Auth(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrAuth)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// ErrAuth: Disable, not ApplyCooldown
	if acc.Status() != entity.StatusDisabled {
		t.Errorf("Status = %q, want %q", acc.Status(), entity.StatusDisabled)
	}
	if len(writer.updateStatusCalls) != 1 {
		t.Fatalf("UpdateStatus called %d times, want 1", len(writer.updateStatusCalls))
	}
	// ShouldFallback follows domain VO
	if result.ShouldFallback != vo.ErrAuth.ShouldFallback() {
		t.Errorf("ShouldFallback = %v, want %v", result.ShouldFallback, vo.ErrAuth.ShouldFallback())
	}
	if result.Classification != vo.ErrAuth {
		t.Errorf("Classification = %q, want %q", result.Classification, vo.ErrAuth)
	}
}

func TestHandleFallback_Client(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrClient)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// No mutation for client errors
	if acc.Status() != entity.StatusActive {
		t.Errorf("Status = %q, should remain %q", acc.Status(), entity.StatusActive)
	}
	if len(writer.updateStatusCalls) != 0 {
		t.Errorf("UpdateStatus should not be called for client errors, called %d times", len(writer.updateStatusCalls))
	}
	if result.ShouldFallback {
		t.Error("ShouldFallback should be false for client_error")
	}
}

func TestHandleFallback_Unknown(t *testing.T) {
	writer := &mockAccountStatusWriter{}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	result, err := uc.Execute(context.Background(), acc, vo.ErrUnknown)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// No mutation for unknown errors
	if acc.Status() != entity.StatusActive {
		t.Errorf("Status = %q, should remain %q", acc.Status(), entity.StatusActive)
	}
	if len(writer.updateStatusCalls) != 0 {
		t.Errorf("UpdateStatus should not be called for unknown errors, called %d times", len(writer.updateStatusCalls))
	}
	if result.ShouldFallback {
		t.Error("ShouldFallback should be false for unknown")
	}
}

func TestHandleFallback_UpdateStatusError(t *testing.T) {
	writer := &mockAccountStatusWriter{
		updateStatusErr: errors.New("db error"),
	}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	_, err := uc.Execute(context.Background(), acc, vo.ErrRateLimit)
	if err == nil {
		t.Fatal("expected error when UpdateStatus fails")
	}
}

func TestHandleFallback_AuthUpdateStatusError(t *testing.T) {
	writer := &mockAccountStatusWriter{
		updateStatusErr: errors.New("db error"),
	}
	clock := &fixedClock{now: fallbackNow}
	uc := NewHandleFallbackUseCase(writer, clock)

	acc := newTestAccount(t)
	_, err := uc.Execute(context.Background(), acc, vo.ErrAuth)
	if err == nil {
		t.Fatal("expected error when UpdateStatus fails for auth")
	}
}
