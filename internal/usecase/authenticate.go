package usecase

import (
	"context"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyFinder finds API keys by prefix (ISP — only what auth needs).
type APIKeyFinder interface {
	FindByPrefix(ctx context.Context, prefix string) (*entity.APIKey, error)
}

// AuthenticateUseCase validates a raw API key and returns the matching entity.
type AuthenticateUseCase struct {
	finder APIKeyFinder
}

// NewAuthenticateUseCase creates a new AuthenticateUseCase.
func NewAuthenticateUseCase(finder APIKeyFinder) *AuthenticateUseCase {
	return &AuthenticateUseCase{finder: finder}
}

// Execute authenticates a raw API key.
// Extracts the prefix (first 12 chars), finds the key by prefix,
// validates the hash via bcrypt, and records usage.
func (uc *AuthenticateUseCase) Execute(ctx context.Context, rawKey string) (*entity.APIKey, error) {
	if len(rawKey) < keyPrefixLen {
		return nil, errs.ErrInvalidAPIKey
	}

	prefix := rawKey[:keyPrefixLen]

	apiKey, err := uc.finder.FindByPrefix(ctx, prefix)
	if err != nil {
		return nil, errs.ErrInvalidAPIKey
	}

	if !apiKey.Validate(rawKey, compareBcrypt) {
		return nil, errs.ErrInvalidAPIKey
	}

	apiKey.RecordUsage(time.Now())

	return apiKey, nil
}

// compareBcrypt compares a bcrypt hash with a raw string.
func compareBcrypt(hash, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}
