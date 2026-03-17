package usecase

import (
	"context"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const defaultBaseURL = "https://api.anthropic.com"

// AccountStore provides full CRUD for account administration.
type AccountStore interface {
	FindByID(ctx context.Context, id vo.AccountID) (*entity.Account, error)
	FindAll(ctx context.Context) ([]*entity.Account, error)
	Create(ctx context.Context, account *entity.Account) error
	Save(ctx context.Context, account *entity.Account) error
	Delete(ctx context.Context, id vo.AccountID) error
}

// CreateAccountInput contains the fields needed to create an account.
type CreateAccountInput struct {
	Name     string
	APIKey   string
	BaseURL  string
	Priority int
}

// UpdateAccountInput contains optional fields for partial account update.
type UpdateAccountInput struct {
	Name     *string
	APIKey   *string
	BaseURL  *string
	Priority *int
}

// AdminAccountUseCase handles account administration operations.
type AdminAccountUseCase struct {
	store AccountStore
}

// NewAdminAccountUseCase creates a new AdminAccountUseCase.
func NewAdminAccountUseCase(store AccountStore) *AdminAccountUseCase {
	return &AdminAccountUseCase{store: store}
}

// Create creates a new account from the given input.
func (uc *AdminAccountUseCase) Create(ctx context.Context, input CreateAccountInput) (*entity.Account, error) {
	baseURL := input.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		input.Name,
		vo.NewSensitiveString(input.APIKey),
		baseURL,
		input.Priority,
	)
	if err != nil {
		return nil, err
	}

	if err := uc.store.Create(ctx, acc); err != nil {
		return nil, err
	}

	return acc, nil
}

// Update applies partial updates to an existing account.
func (uc *AdminAccountUseCase) Update(ctx context.Context, id vo.AccountID, input UpdateAccountInput) (*entity.Account, error) {
	existing, err := uc.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := existing.Name()
	if input.Name != nil {
		name = *input.Name
	}

	apiKey := existing.Credentials().APIKey
	if input.APIKey != nil {
		apiKey = vo.NewSensitiveString(*input.APIKey)
	}

	baseURL := existing.BaseURL()
	if input.BaseURL != nil {
		baseURL = *input.BaseURL
	}

	priority := existing.Priority()
	if input.Priority != nil {
		priority = *input.Priority
	}

	updated, err := entity.RehydrateAccount(
		existing.ID(),
		name,
		apiKey,
		baseURL,
		existing.Status(),
		priority,
		nil, // cooldown preserved via status
		existing.LastUsedAt(),
		existing.LastError(),
	)
	if err != nil {
		return nil, err
	}

	if err := uc.store.Save(ctx, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete removes an account by ID.
func (uc *AdminAccountUseCase) Delete(ctx context.Context, id vo.AccountID) error {
	return uc.store.Delete(ctx, id)
}

// Reset clears cooldown and errors on an account.
func (uc *AdminAccountUseCase) Reset(ctx context.Context, id vo.AccountID) (*entity.Account, error) {
	acc, err := uc.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	acc.ClearError()

	if err := uc.store.Save(ctx, acc); err != nil {
		return nil, err
	}

	return acc, nil
}
