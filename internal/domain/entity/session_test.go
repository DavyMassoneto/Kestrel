package entity_test

import (
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func newTestSession(t *testing.T) *entity.Session {
	t.Helper()
	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	s, err := entity.NewSession(
		vo.NewSessionID(),
		vo.NewAPIKeyID(),
		model,
		30*time.Minute,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s
}

func TestNewSession_Valid(t *testing.T) {
	id := vo.NewSessionID()
	keyID := vo.NewAPIKeyID()
	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	now := time.Now()
	ttl := 30 * time.Minute

	s, err := entity.NewSession(id, keyID, model, ttl, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID() != id {
		t.Errorf("ID = %v; want %v", s.ID(), id)
	}
	if s.APIKeyID() != keyID {
		t.Errorf("APIKeyID = %v; want %v", s.APIKeyID(), keyID)
	}
	if s.AccountID() != nil {
		t.Error("AccountID should be nil initially")
	}
	if s.Model().Raw != model.Raw {
		t.Errorf("Model = %q; want %q", s.Model().Raw, model.Raw)
	}
	if s.RequestCount() != 0 {
		t.Errorf("RequestCount = %d; want 0", s.RequestCount())
	}
	if !s.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt = %v; want %v", s.CreatedAt(), now)
	}
	if !s.LastActiveAt().Equal(now) {
		t.Errorf("LastActiveAt = %v; want %v", s.LastActiveAt(), now)
	}
	if s.TTL() != ttl {
		t.Errorf("TTL = %v; want %v", s.TTL(), ttl)
	}
}

func TestNewSession_ZeroTTL(t *testing.T) {
	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	_, err := entity.NewSession(vo.NewSessionID(), vo.NewAPIKeyID(), model, 0, time.Now())
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}
}

func TestSession_BindAccount(t *testing.T) {
	s := newTestSession(t)
	accID := vo.NewAccountID()

	s.BindAccount(accID)

	if s.AccountID() == nil {
		t.Fatal("AccountID should not be nil after BindAccount")
	}
	if *s.AccountID() != accID {
		t.Errorf("AccountID = %v; want %v", *s.AccountID(), accID)
	}
}

func TestSession_UnbindAccount(t *testing.T) {
	s := newTestSession(t)
	s.BindAccount(vo.NewAccountID())
	s.UnbindAccount()

	if s.AccountID() != nil {
		t.Error("AccountID should be nil after UnbindAccount")
	}
}

func TestSession_RecordRequest(t *testing.T) {
	s := newTestSession(t)
	now := time.Now().Add(5 * time.Minute)

	s.RecordRequest(now)

	if s.RequestCount() != 1 {
		t.Errorf("RequestCount = %d; want 1", s.RequestCount())
	}
	if !s.LastActiveAt().Equal(now) {
		t.Errorf("LastActiveAt = %v; want %v", s.LastActiveAt(), now)
	}

	later := now.Add(1 * time.Minute)
	s.RecordRequest(later)

	if s.RequestCount() != 2 {
		t.Errorf("RequestCount = %d; want 2", s.RequestCount())
	}
}

func TestSession_IsExpired_False(t *testing.T) {
	s := newTestSession(t)
	if s.IsExpired(time.Now()) {
		t.Error("new session should not be expired")
	}
}

func TestSession_IsExpired_True(t *testing.T) {
	s := newTestSession(t)
	future := time.Now().Add(31 * time.Minute)

	if !s.IsExpired(future) {
		t.Error("session should be expired after TTL")
	}
}

func TestSession_IsExpired_AfterRecordRequest(t *testing.T) {
	now := time.Now()
	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	s, _ := entity.NewSession(vo.NewSessionID(), vo.NewAPIKeyID(), model, 10*time.Minute, now)

	// Record request 8 minutes later
	s.RecordRequest(now.Add(8 * time.Minute))

	// At 15 minutes from start, session should NOT be expired (last active was at 8 min, TTL is 10 min)
	if s.IsExpired(now.Add(15 * time.Minute)) {
		t.Error("session should not be expired — last active + TTL > now")
	}

	// At 19 minutes from start, session SHOULD be expired (last active at 8 + TTL 10 = 18 < 19)
	if !s.IsExpired(now.Add(19 * time.Minute)) {
		t.Error("session should be expired — last active + TTL < now")
	}
}
