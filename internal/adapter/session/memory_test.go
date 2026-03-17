package session_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/session"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const defaultTTL = 30 * time.Minute

func makeModel(t *testing.T, raw string) vo.ModelName {
	t.Helper()
	m, err := vo.ParseModelName(raw)
	if err != nil {
		t.Fatalf("ParseModelName(%q): %v", raw, err)
	}
	return m
}

func TestGetOrCreate_NewSession(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	sess, err := store.GetOrCreate(context.Background(), apiKeyID, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("session should not be nil")
	}
	if sess.APIKeyID() != apiKeyID {
		t.Errorf("APIKeyID = %v; want %v", sess.APIKeyID(), apiKeyID)
	}
	if sess.Model().Raw != model.Raw {
		t.Errorf("Model = %q; want %q", sess.Model().Raw, model.Raw)
	}
}

func TestGetOrCreate_ExistingSession(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	first, err := store.GetOrCreate(context.Background(), apiKeyID, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	second, err := store.GetOrCreate(context.Background(), apiKeyID, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.ID() != second.ID() {
		t.Errorf("expected same session ID, got %v and %v", first.ID(), second.ID())
	}
}

func TestGetOrCreate_DifferentKeys(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	model := makeModel(t, "claude-sonnet-4-20250514")

	s1, _ := store.GetOrCreate(context.Background(), vo.NewAPIKeyID(), model)
	s2, _ := store.GetOrCreate(context.Background(), vo.NewAPIKeyID(), model)

	if s1.ID() == s2.ID() {
		t.Error("different API keys should create different sessions")
	}
}

func TestGetOrCreate_DifferentModels(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	m1 := makeModel(t, "claude-sonnet-4-20250514")
	m2 := makeModel(t, "claude-opus-4-20250514")

	s1, _ := store.GetOrCreate(context.Background(), apiKeyID, m1)
	s2, _ := store.GetOrCreate(context.Background(), apiKeyID, m2)

	if s1.ID() == s2.ID() {
		t.Error("different models should create different sessions")
	}
}

func TestGetOrCreate_ExpiredSession(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, 1*time.Millisecond)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	first, _ := store.GetOrCreate(context.Background(), apiKeyID, model)

	// Wait for session to expire
	time.Sleep(5 * time.Millisecond)

	second, err := store.GetOrCreate(context.Background(), apiKeyID, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.ID() == second.ID() {
		t.Error("expired session should be replaced with new one")
	}
}

func TestSave(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	sess, _ := store.GetOrCreate(context.Background(), apiKeyID, model)

	// Mutate session
	accountID := vo.NewAccountID()
	sess.BindAccount(accountID)
	sess.RecordRequest(time.Now())

	if err := store.Save(context.Background(), sess); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Retrieve and verify persistence
	retrieved, _ := store.GetOrCreate(context.Background(), apiKeyID, model)
	if retrieved.AccountID() == nil {
		t.Fatal("AccountID should be set after Save")
	}
	if *retrieved.AccountID() != accountID {
		t.Errorf("AccountID = %v; want %v", *retrieved.AccountID(), accountID)
	}
	if retrieved.RequestCount() != 1 {
		t.Errorf("RequestCount = %d; want 1", retrieved.RequestCount())
	}
}

func TestCleanup_RemovesExpired(t *testing.T) {
	store := session.NewMemorySessionStore(10*time.Millisecond, 1*time.Millisecond)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	first, _ := store.GetOrCreate(context.Background(), apiKeyID, model)

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	second, _ := store.GetOrCreate(context.Background(), apiKeyID, model)
	if first.ID() == second.ID() {
		t.Error("cleanup should have removed expired session")
	}
}

func TestCleanup_KeepsValid(t *testing.T) {
	store := session.NewMemorySessionStore(10*time.Millisecond, time.Hour)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	first, _ := store.GetOrCreate(context.Background(), apiKeyID, model)

	// Wait for cleanup to run (but session has long TTL)
	time.Sleep(50 * time.Millisecond)

	second, _ := store.GetOrCreate(context.Background(), apiKeyID, model)
	if first.ID() != second.ID() {
		t.Error("valid session should survive cleanup")
	}
}

func TestConcurrency(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	model := makeModel(t, "claude-sonnet-4-20250514")
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			apiKeyID := vo.NewAPIKeyID()
			sess, err := store.GetOrCreate(context.Background(), apiKeyID, model)
			if err != nil {
				t.Errorf("GetOrCreate error: %v", err)
				return
			}
			sess.RecordRequest(time.Now())
			if err := store.Save(context.Background(), sess); err != nil {
				t.Errorf("Save error: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestConcurrency_SameKey(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)

	// Seed the session
	store.GetOrCreate(context.Background(), apiKeyID, model)

	sessions := make([]*entity.Session, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			sess, err := store.GetOrCreate(context.Background(), apiKeyID, model)
			if err != nil {
				t.Errorf("GetOrCreate error: %v", err)
				return
			}
			sessions[idx] = sess
		}(i)
	}
	wg.Wait()

	// All should have gotten the same session
	firstID := sessions[0].ID()
	for i, s := range sessions {
		if s.ID() != firstID {
			t.Errorf("sessions[%d].ID = %v; want %v", i, s.ID(), firstID)
		}
	}
}

func TestGetOrCreate_InvalidTTL(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, 0)
	defer store.Stop()

	apiKeyID := vo.NewAPIKeyID()
	model := makeModel(t, "claude-sonnet-4-20250514")

	_, err := store.GetOrCreate(context.Background(), apiKeyID, model)
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}
}

func TestStop_Idempotent(t *testing.T) {
	store := session.NewMemorySessionStore(time.Hour, defaultTTL)
	store.Stop()
	store.Stop() // should not panic
}
