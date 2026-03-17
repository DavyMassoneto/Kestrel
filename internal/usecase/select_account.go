package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// AccountFinder finds available accounts (ISP — only what selection needs).
type AccountFinder interface {
	FindAvailable(ctx context.Context, excludeID *vo.AccountID) ([]*entity.Account, error)
}

// SelectAccountUseCase selects the best available account for a request.
type SelectAccountUseCase struct {
	finder AccountFinder
	clock  Clock
}

// NewSelectAccountUseCase creates a new SelectAccountUseCase.
func NewSelectAccountUseCase(finder AccountFinder, clock Clock) *SelectAccountUseCase {
	return &SelectAccountUseCase{finder: finder, clock: clock}
}

// Execute selects an account using priority + LRU with optional sticky routing.
func (uc *SelectAccountUseCase) Execute(ctx context.Context, preferredID *vo.AccountID, excludeID *vo.AccountID, now time.Time) (*entity.Account, error) {
	accounts, err := uc.finder.FindAvailable(ctx, excludeID)
	if err != nil {
		return nil, err
	}

	// Defense in depth: re-filter with entity's IsAvailable
	available := make([]*entity.Account, 0, len(accounts))
	for _, acc := range accounts {
		if acc.IsAvailable(now) {
			available = append(available, acc)
		}
	}

	if len(available) == 0 {
		return nil, errs.ErrAllAccountsExhausted
	}

	// Sticky routing: prefer the requested account if available
	if preferredID != nil {
		for _, acc := range available {
			if acc.ID() == *preferredID {
				return acc, nil
			}
		}
	}

	// Sort: priority ASC, then LastUsedAt ASC (nil = never used = first)
	sort.Slice(available, func(i, j int) bool {
		if available[i].Priority() != available[j].Priority() {
			return available[i].Priority() < available[j].Priority()
		}
		ti := available[i].LastUsedAt()
		tj := available[j].LastUsedAt()
		if ti == nil && tj == nil {
			return false
		}
		if ti == nil {
			return true
		}
		if tj == nil {
			return false
		}
		return ti.Before(*tj)
	})

	return available[0], nil
}
