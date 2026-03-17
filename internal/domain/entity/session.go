package entity

import (
	"fmt"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// Session represents a routing session with behavior.
type Session struct {
	id           vo.SessionID
	apiKeyID     vo.APIKeyID
	accountID    *vo.AccountID
	model        vo.ModelName
	requestCount int
	createdAt    time.Time
	lastActiveAt time.Time
	ttl          time.Duration
}

// NewSession creates a validated session. Returns error if required fields are missing.
func NewSession(id vo.SessionID, apiKeyID vo.APIKeyID, model vo.ModelName, ttl time.Duration, now time.Time) (*Session, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("session TTL must be positive")
	}
	return &Session{
		id:           id,
		apiKeyID:     apiKeyID,
		model:        model,
		ttl:          ttl,
		createdAt:    now,
		lastActiveAt: now,
	}, nil
}

// Getters

func (s *Session) ID() vo.SessionID        { return s.id }
func (s *Session) APIKeyID() vo.APIKeyID    { return s.apiKeyID }
func (s *Session) Model() vo.ModelName      { return s.model }
func (s *Session) RequestCount() int        { return s.requestCount }
func (s *Session) CreatedAt() time.Time     { return s.createdAt }
func (s *Session) LastActiveAt() time.Time  { return s.lastActiveAt }
func (s *Session) TTL() time.Duration       { return s.ttl }

func (s *Session) AccountID() *vo.AccountID {
	return s.accountID
}

// BindAccount associates the session with an account (sticky routing).
func (s *Session) BindAccount(accountID vo.AccountID) {
	s.accountID = &accountID
}

// UnbindAccount removes the account association.
func (s *Session) UnbindAccount() {
	s.accountID = nil
}

// RecordRequest increments RequestCount and updates LastActiveAt.
func (s *Session) RecordRequest(now time.Time) {
	s.requestCount++
	s.lastActiveAt = now
}

// IsExpired returns true if LastActiveAt + TTL < now.
func (s *Session) IsExpired(now time.Time) bool {
	return s.lastActiveAt.Add(s.ttl).Before(now)
}
