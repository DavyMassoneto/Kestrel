package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"golang.org/x/crypto/bcrypt"
)

const (
	rawKeyPrefix    = "omni-"
	rawKeyRandomLen = 32 // 32 random bytes = 64 hex chars
	keyPrefixLen    = 12 // first 12 chars of raw key used as prefix
)

// APIKeyStore provides full CRUD for API key administration.
type APIKeyStore interface {
	FindByID(ctx context.Context, id vo.APIKeyID) (*entity.APIKey, error)
	FindAll(ctx context.Context) ([]*entity.APIKey, error)
	Create(ctx context.Context, key *entity.APIKey) error
	Delete(ctx context.Context, id vo.APIKeyID) error
}

// CreateAPIKeyInput contains the fields needed to create an API key.
type CreateAPIKeyInput struct {
	Name          string
	AllowedModels []string
}

// AdminAPIKeyUseCase handles API key administration operations.
type AdminAPIKeyUseCase struct {
	store APIKeyStore
}

// NewAdminAPIKeyUseCase creates a new AdminAPIKeyUseCase.
func NewAdminAPIKeyUseCase(store APIKeyStore) *AdminAPIKeyUseCase {
	return &AdminAPIKeyUseCase{store: store}
}

// Create generates a new API key, hashes it, and persists it.
// Returns the entity and the raw key (shown only once to the user).
func (uc *AdminAPIKeyUseCase) Create(ctx context.Context, input CreateAPIKeyInput) (*entity.APIKey, string, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash key: %w", err)
	}

	prefix := rawKey[:keyPrefixLen]

	key, err := entity.NewAPIKey(
		vo.NewAPIKeyID(),
		input.Name,
		string(hashBytes),
		prefix,
	)
	if err != nil {
		return nil, "", err
	}

	if len(input.AllowedModels) > 0 {
		key.SetAllowedModels(input.AllowedModels)
	}

	if err := uc.store.Create(ctx, key); err != nil {
		return nil, "", err
	}

	return key, rawKey, nil
}

// Revoke deletes an API key by ID.
func (uc *AdminAPIKeyUseCase) Revoke(ctx context.Context, id vo.APIKeyID) error {
	return uc.store.Delete(ctx, id)
}

func generateRawKey() (string, error) {
	b := make([]byte, rawKeyRandomLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return rawKeyPrefix + hex.EncodeToString(b), nil
}
