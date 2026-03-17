package usecase

import (
	"context"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// AccountSelector abstracts account selection (implemented by SelectAccountUseCase).
type AccountSelector interface {
	Execute(ctx context.Context, preferredID *vo.AccountID, excludeID *vo.AccountID, now time.Time) (*entity.Account, error)
}

// FallbackHandler abstracts fallback handling (implemented by HandleFallbackUseCase).
type FallbackHandler interface {
	Execute(ctx context.Context, account *entity.Account, classification vo.ErrorClassification) (FallbackResult, error)
}

// FallbackResult encapsulates the result of a fallback attempt.
type FallbackResult struct {
	ShouldFallback bool
	Classification vo.ErrorClassification
}

// Clock abstracts the system clock for testability.
type Clock interface {
	Now() time.Time
}

// ClassifiedError is implemented by errors that carry a domain classification.
// Use cases extract the classification via errors.As without importing adapter types.
type ClassifiedError interface {
	error
	Classification() vo.ErrorClassification
}

// RetryAttempt records a fallback attempt for observability.
type RetryAttempt struct {
	AccountID      vo.AccountID
	Classification vo.ErrorClassification
	RetryIndex     int
}

// ProxyChatResult encapsulates response + retry metadata for observability.
type ProxyChatResult struct {
	Response *vo.ChatResponse
	Retries  []RetryAttempt
}

// ProxyStreamResult encapsulates channel + retry metadata.
type ProxyStreamResult struct {
	Events  <-chan vo.StreamEvent
	Retries []RetryAttempt
}
