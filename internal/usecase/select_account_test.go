package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
)

// --- mock AccountFinder ---

type mockAccountFinder struct {
	accounts []*entity.Account
	err      error
}

func (m *mockAccountFinder) FindAvailable(_ context.Context, _ *vo.AccountID) ([]*entity.Account, error) {
	return m.accounts, m.err
}

// --- mock Clock ---

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time { return c.now }

// --- helpers ---

func makeAccount(t *testing.T, name string, priority int, lastUsedAt *time.Time) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		name,
		vo.NewSensitiveString("sk-test"),
		"https://api.anthropic.com",
		priority,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	if lastUsedAt != nil {
		acc.RecordUsage(*lastUsedAt)
	}
	return acc
}

func TestSelectAccount_NoAccounts(t *testing.T) {
	finder := &mockAccountFinder{accounts: []*entity.Account{}}
	clock := &fixedClock{now: time.Now()}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	_, err := uc.Execute(context.Background(), nil, nil, clock.Now())
	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Errorf("err = %v; want %v", err, errs.ErrAllAccountsExhausted)
	}
}

func TestSelectAccount_FinderError(t *testing.T) {
	finder := &mockAccountFinder{err: errors.New("db error")}
	clock := &fixedClock{now: time.Now()}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	_, err := uc.Execute(context.Background(), nil, nil, clock.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectAccount_SingleAccount(t *testing.T) {
	acc := makeAccount(t, "single", 0, nil)
	finder := &mockAccountFinder{accounts: []*entity.Account{acc}}
	clock := &fixedClock{now: time.Now()}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, clock.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != acc.ID() {
		t.Errorf("ID = %v; want %v", result.ID(), acc.ID())
	}
}

func TestSelectAccount_PriorityOrdering(t *testing.T) {
	low := makeAccount(t, "low", 10, nil)
	high := makeAccount(t, "high", 1, nil)
	finder := &mockAccountFinder{accounts: []*entity.Account{low, high}}
	clock := &fixedClock{now: time.Now()}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, clock.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != high.ID() {
		t.Errorf("expected high priority account, got %v", result.Name())
	}
}

func TestSelectAccount_LRU_SamePriority(t *testing.T) {
	now := time.Now()
	older := now.Add(-10 * time.Minute)
	newer := now.Add(-1 * time.Minute)

	accOld := makeAccount(t, "old", 0, &older)
	accNew := makeAccount(t, "new", 0, &newer)
	finder := &mockAccountFinder{accounts: []*entity.Account{accNew, accOld}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != accOld.ID() {
		t.Errorf("expected LRU (oldest) account, got %v", result.Name())
	}
}

func TestSelectAccount_LRU_NilLastUsedAtFirst(t *testing.T) {
	now := time.Now()
	used := now.Add(-1 * time.Minute)

	neverUsed := makeAccount(t, "never-used", 0, nil)
	usedAcc := makeAccount(t, "used", 0, &used)
	finder := &mockAccountFinder{accounts: []*entity.Account{usedAcc, neverUsed}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != neverUsed.ID() {
		t.Errorf("expected never-used account (nil LastUsedAt), got %v", result.Name())
	}
}

func TestSelectAccount_LRU_BothNilLastUsedAt(t *testing.T) {
	now := time.Now()
	acc1 := makeAccount(t, "a", 0, nil)
	acc2 := makeAccount(t, "b", 0, nil)
	finder := &mockAccountFinder{accounts: []*entity.Account{acc1, acc2}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Either is fine since both are equal — just verify no panic and one is returned
	if result.ID() != acc1.ID() && result.ID() != acc2.ID() {
		t.Error("expected one of the two accounts")
	}
}

func TestSelectAccount_LRU_MixedNilAndUsed(t *testing.T) {
	now := time.Now()
	older := now.Add(-10 * time.Minute)
	newer := now.Add(-1 * time.Minute)

	// 3 accounts with same priority: older, newer, never-used
	// Sort must compare all pairs, forcing ti!=nil,tj==nil comparisons
	accOlder := makeAccount(t, "older", 0, &older)
	accNewer := makeAccount(t, "newer", 0, &newer)
	accNever := makeAccount(t, "never", 0, nil)
	// accNever first — sort may call less(1,0) where j=0(nil), covering tj==nil branch
	finder := &mockAccountFinder{accounts: []*entity.Account{accNever, accNewer, accOlder}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != accNever.ID() {
		t.Errorf("expected never-used account first, got %v", result.Name())
	}
}

func TestSelectAccount_StickyRouting(t *testing.T) {
	preferred := makeAccount(t, "preferred", 10, nil)
	better := makeAccount(t, "better", 1, nil)
	prefID := preferred.ID()
	finder := &mockAccountFinder{accounts: []*entity.Account{better, preferred}}
	clock := &fixedClock{now: time.Now()}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), &prefID, nil, clock.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != preferred.ID() {
		t.Errorf("expected sticky-routed preferred account, got %v", result.Name())
	}
}

func TestSelectAccount_StickyRouting_PreferredNotAvailable(t *testing.T) {
	now := time.Now()
	// preferred is in cooldown (not in the available list)
	fallback := makeAccount(t, "fallback", 0, nil)
	prefID := vo.NewAccountID() // ID that doesn't match any account
	finder := &mockAccountFinder{accounts: []*entity.Account{fallback}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), &prefID, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != fallback.ID() {
		t.Errorf("expected fallback account, got %v", result.Name())
	}
}

func TestSelectAccount_RefilterIsAvailable(t *testing.T) {
	now := time.Now()

	// Account that was returned by finder but is actually in cooldown
	cooldownAcc := makeAccount(t, "cooldown", 0, nil)
	cooldownAcc.ApplyCooldown(vo.ErrRateLimit, now) // cooldown active

	available := makeAccount(t, "available", 1, nil)
	finder := &mockAccountFinder{accounts: []*entity.Account{cooldownAcc, available}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	result, err := uc.Execute(context.Background(), nil, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID() != available.ID() {
		t.Errorf("expected available account (cooldown filtered), got %v", result.Name())
	}
}

func TestSelectAccount_AllFilteredByIsAvailable(t *testing.T) {
	now := time.Now()

	cooldownAcc := makeAccount(t, "cooldown", 0, nil)
	cooldownAcc.ApplyCooldown(vo.ErrRateLimit, now)

	finder := &mockAccountFinder{accounts: []*entity.Account{cooldownAcc}}
	clock := &fixedClock{now: now}
	uc := usecase.NewSelectAccountUseCase(finder, clock)

	_, err := uc.Execute(context.Background(), nil, nil, now)
	if !errors.Is(err, errs.ErrAllAccountsExhausted) {
		t.Errorf("err = %v; want %v", err, errs.ErrAllAccountsExhausted)
	}
}
