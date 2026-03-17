package usecase

import (
	"context"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// AccountStatusWriter persists account status changes.
type AccountStatusWriter interface {
	UpdateStatus(ctx context.Context, account *entity.Account) error
	RecordSuccess(ctx context.Context, accountID vo.AccountID, now time.Time) error
}

// HandleFallbackUseCase decides how to handle a provider error.
type HandleFallbackUseCase struct {
	accountWriter AccountStatusWriter
	clock         Clock
}

// NewHandleFallbackUseCase creates a new HandleFallbackUseCase.
func NewHandleFallbackUseCase(writer AccountStatusWriter, clock Clock) *HandleFallbackUseCase {
	return &HandleFallbackUseCase{
		accountWriter: writer,
		clock:         clock,
	}
}

// Execute applies the appropriate action based on error classification.
func (uc *HandleFallbackUseCase) Execute(ctx context.Context, account *entity.Account, classification vo.ErrorClassification) (FallbackResult, error) {
	switch classification {
	case vo.ErrAuth:
		account.Disable("auth error")
		if err := uc.accountWriter.UpdateStatus(ctx, account); err != nil {
			return FallbackResult{}, err
		}

	case vo.ErrClient, vo.ErrUnknown:
		// No mutation — return directly

	default:
		// Transient errors: rate_limit, quota_exhausted, overloaded, server_error
		account.ApplyCooldown(classification, uc.clock.Now())
		if err := uc.accountWriter.UpdateStatus(ctx, account); err != nil {
			return FallbackResult{}, err
		}
	}

	return FallbackResult{
		ShouldFallback: classification.ShouldFallback(),
		Classification: classification,
	}, nil
}
