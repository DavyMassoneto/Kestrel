package session

import (
	"context"
	"sync"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// MemorySessionStore implements SessionReader and SessionWriter in-memory.
type MemorySessionStore struct {
	mu         sync.RWMutex
	sessions   map[string]*entity.Session
	sessionTTL time.Duration
	stopOnce   sync.Once
	done       chan struct{}
}

// NewMemorySessionStore creates an in-memory session store with periodic cleanup.
func NewMemorySessionStore(cleanupInterval, sessionTTL time.Duration) *MemorySessionStore {
	s := &MemorySessionStore{
		sessions:   make(map[string]*entity.Session),
		sessionTTL: sessionTTL,
		done:       make(chan struct{}),
	}
	go s.cleanupLoop(cleanupInterval)
	return s
}

// GetOrCreate returns an existing valid session or creates a new one.
func (s *MemorySessionStore) GetOrCreate(_ context.Context, apiKeyID vo.APIKeyID, model vo.ModelName) (*entity.Session, error) {
	key := sessionKey(apiKeyID, model)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[key]; ok && !sess.IsExpired(now) {
		return sess, nil
	}

	sess, err := entity.NewSession(vo.NewSessionID(), apiKeyID, model, s.sessionTTL, now)
	if err != nil {
		return nil, err
	}
	s.sessions[key] = sess
	return sess, nil
}

// Save persists a session update.
func (s *MemorySessionStore) Save(_ context.Context, sess *entity.Session) error {
	key := sessionKey(sess.APIKeyID(), sess.Model())
	s.mu.Lock()
	s.sessions[key] = sess
	s.mu.Unlock()
	return nil
}

// Stop halts the cleanup goroutine.
func (s *MemorySessionStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.done)
	})
}

func (s *MemorySessionStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *MemorySessionStore) cleanup() {
	now := time.Now()
	s.mu.Lock()
	for key, sess := range s.sessions {
		if sess.IsExpired(now) {
			delete(s.sessions, key)
		}
	}
	s.mu.Unlock()
}

func sessionKey(apiKeyID vo.APIKeyID, model vo.ModelName) string {
	return apiKeyID.String() + ":" + model.Raw
}
