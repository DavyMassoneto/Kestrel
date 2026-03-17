package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
	"golang.org/x/crypto/bcrypt"
)

// --- mock APIKeyFinder ---

type mockAPIKeyFinder struct {
	key *entity.APIKey
	err error
}

func (m *mockAPIKeyFinder) FindByPrefix(_ context.Context, _ string) (*entity.APIKey, error) {
	return m.key, m.err
}

// --- helpers ---

func makeKey(t *testing.T, rawKey string) *entity.APIKey {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	prefix := rawKey[:12]
	key, err := entity.NewAPIKey(vo.NewAPIKeyID(), "test-key", string(hash), prefix)
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	return key
}

const validRawKey = "omni-abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"

func TestAuthenticate_ValidKey(t *testing.T) {
	key := makeKey(t, validRawKey)
	finder := &mockAPIKeyFinder{key: key}
	uc := usecase.NewAuthenticateUseCase(finder)

	result, err := uc.Execute(context.Background(), validRawKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != key.ID() {
		t.Errorf("ID = %v; want %v", result.ID(), key.ID())
	}
	if result.LastUsedAt() == nil {
		t.Error("LastUsedAt should be set after authentication")
	}
}

func TestAuthenticate_PrefixNotFound(t *testing.T) {
	finder := &mockAPIKeyFinder{err: errors.New("API key not found")}
	uc := usecase.NewAuthenticateUseCase(finder)

	_, err := uc.Execute(context.Background(), validRawKey)
	if !errors.Is(err, errs.ErrInvalidAPIKey) {
		t.Errorf("err = %v; want %v", err, errs.ErrInvalidAPIKey)
	}
}

func TestAuthenticate_HashMismatch(t *testing.T) {
	// Key stored with a different raw key
	key := makeKey(t, validRawKey)
	finder := &mockAPIKeyFinder{key: key}
	uc := usecase.NewAuthenticateUseCase(finder)

	wrongKey := "omni-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
	_, err := uc.Execute(context.Background(), wrongKey)
	if !errors.Is(err, errs.ErrInvalidAPIKey) {
		t.Errorf("err = %v; want %v", err, errs.ErrInvalidAPIKey)
	}
}

func TestAuthenticate_KeyTooShort(t *testing.T) {
	finder := &mockAPIKeyFinder{}
	uc := usecase.NewAuthenticateUseCase(finder)

	_, err := uc.Execute(context.Background(), "short")
	if !errors.Is(err, errs.ErrInvalidAPIKey) {
		t.Errorf("err = %v; want %v", err, errs.ErrInvalidAPIKey)
	}
}

func TestAuthenticate_EmptyKey(t *testing.T) {
	finder := &mockAPIKeyFinder{}
	uc := usecase.NewAuthenticateUseCase(finder)

	_, err := uc.Execute(context.Background(), "")
	if !errors.Is(err, errs.ErrInvalidAPIKey) {
		t.Errorf("err = %v; want %v", err, errs.ErrInvalidAPIKey)
	}
}

func TestAuthenticate_FinderReturnsError(t *testing.T) {
	finder := &mockAPIKeyFinder{err: errors.New("db connection lost")}
	uc := usecase.NewAuthenticateUseCase(finder)

	_, err := uc.Execute(context.Background(), validRawKey)
	if !errors.Is(err, errs.ErrInvalidAPIKey) {
		t.Errorf("err = %v; want %v", err, errs.ErrInvalidAPIKey)
	}
}
