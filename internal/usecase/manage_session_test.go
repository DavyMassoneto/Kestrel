package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type mockSessionReader struct {
	session *entity.Session
	err     error
	calls   int
}

func (m *mockSessionReader) GetOrCreate(ctx context.Context, apiKeyID vo.APIKeyID, model vo.ModelName) (*entity.Session, error) {
	m.calls++
	return m.session, m.err
}

type mockSessionWriter struct {
	saved *entity.Session
	err   error
	calls int
}

func (m *mockSessionWriter) Save(ctx context.Context, session *entity.Session) error {
	m.calls++
	m.saved = session
	return m.err
}

var sessionNow = time.Date(2026, 3, 17, 14, 0, 0, 0, time.UTC)

func newTestSession(t *testing.T) *entity.Session {
	t.Helper()
	model, err := vo.ParseModelName("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("ParseModelName: %v", err)
	}
	sess, err := entity.NewSession(
		vo.NewSessionID(),
		vo.NewAPIKeyID(),
		model,
		30*time.Minute,
		sessionNow,
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return sess
}

func TestManageSession_GetOrCreate_ReturnsSession(t *testing.T) {
	sess := newTestSession(t)
	reader := &mockSessionReader{session: sess}
	writer := &mockSessionWriter{}
	uc := NewManageSessionUseCase(reader, writer)

	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	got, err := uc.GetOrCreate(context.Background(), sess.APIKeyID(), model)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if got.ID() != sess.ID() {
		t.Errorf("ID = %v, want %v", got.ID(), sess.ID())
	}
	if reader.calls != 1 {
		t.Errorf("reader.GetOrCreate called %d times, want 1", reader.calls)
	}
}

func TestManageSession_GetOrCreate_ReaderError(t *testing.T) {
	reader := &mockSessionReader{err: errors.New("db error")}
	writer := &mockSessionWriter{}
	uc := NewManageSessionUseCase(reader, writer)

	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	_, err := uc.GetOrCreate(context.Background(), vo.NewAPIKeyID(), model)
	if err == nil {
		t.Fatal("expected error when reader fails")
	}
}

func TestManageSession_SaveSession_Success(t *testing.T) {
	sess := newTestSession(t)
	reader := &mockSessionReader{}
	writer := &mockSessionWriter{}
	uc := NewManageSessionUseCase(reader, writer)

	err := uc.SaveSession(context.Background(), sess)
	if err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if writer.calls != 1 {
		t.Errorf("writer.Save called %d times, want 1", writer.calls)
	}
	if writer.saved.ID() != sess.ID() {
		t.Errorf("saved session ID = %v, want %v", writer.saved.ID(), sess.ID())
	}
}

func TestManageSession_SaveSession_WriterError(t *testing.T) {
	sess := newTestSession(t)
	reader := &mockSessionReader{}
	writer := &mockSessionWriter{err: errors.New("write error")}
	uc := NewManageSessionUseCase(reader, writer)

	err := uc.SaveSession(context.Background(), sess)
	if err == nil {
		t.Fatal("expected error when writer fails")
	}
}
