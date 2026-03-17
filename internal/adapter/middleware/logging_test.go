package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type spyRequestLogger struct {
	entries []middleware.RequestLogEntry
}

func (s *spyRequestLogger) LogRequest(_ context.Context, entry middleware.RequestLogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func reqWithAPIKey(req *http.Request) *http.Request {
	ctx := middleware.WithAPIKeyID(req.Context(), vo.NewAPIKeyID())
	ctx = middleware.WithAPIKeyName(ctx, "test-key")
	return req.WithContext(ctx)
}

func TestLogging_Status200(t *testing.T) {
	mw := middleware.NewLogging(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestLogging_StatusError(t *testing.T) {
	mw := middleware.NewLogging(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogging_DefaultStatus200(t *testing.T) {
	mw := middleware.NewLogging(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WriteHeader not called explicitly — default 200
		w.Write([]byte("implicit ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogging_StatusCodeCaptured(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/missing", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", spy.entries[0].Status, http.StatusNotFound)
	}
}

func TestLogging_LatencyPositive(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].LatencyMs <= 0 {
		t.Errorf("LatencyMs = %d, want > 0", spy.entries[0].LatencyMs)
	}
}

func TestLogging_RequestIDFromContext(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	// Chain: RequestID → Logging → handler (inject APIKeyID so LogRequest fires)
	chain := middleware.RequestID(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req_test-request-id-12345")
	// Inject APIKeyID so LogRequest is called
	ctx := middleware.WithAPIKeyID(req.Context(), vo.NewAPIKeyID())
	ctx = middleware.WithAPIKeyName(ctx, "test-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].RequestID != "req_test-request-id-12345" {
		t.Errorf("RequestID = %q, want %q", spy.entries[0].RequestID, "req_test-request-id-12345")
	}
}

func TestLogging_RequestLoggerCalled(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	entry := spy.entries[0]
	if entry.Status != http.StatusCreated {
		t.Errorf("Status = %d, want %d", entry.Status, http.StatusCreated)
	}
	if entry.LatencyMs < 0 {
		t.Errorf("LatencyMs = %d, should be >= 0", entry.LatencyMs)
	}
}

func TestLogging_NilRequestLogger(t *testing.T) {
	mw := middleware.NewLogging(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogging_APIKeyIDFromContext(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	keyID := vo.NewAPIKeyID()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := middleware.WithAPIKeyID(req.Context(), keyID)
	ctx = middleware.WithAPIKeyName(ctx, "test-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].APIKeyID != keyID.String() {
		t.Errorf("APIKeyID = %q, want %q", spy.entries[0].APIKeyID, keyID.String())
	}
	if spy.entries[0].APIKeyName != "test-key" {
		t.Errorf("APIKeyName = %q, want %q", spy.entries[0].APIKeyName, "test-key")
	}
}

func TestLogging_NoAPIKeyInContext_SkipsLogRequest(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if len(spy.entries) != 0 {
		t.Fatalf("expected 0 log entries (unauthenticated), got %d", len(spy.entries))
	}
}

func TestLogging_WriteHeaderCalledOnce(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError) // second call should be ignored
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	// First WriteHeader should win
	if spy.entries[0].Status != http.StatusAccepted {
		t.Errorf("Status = %d, want %d (first WriteHeader should win)", spy.entries[0].Status, http.StatusAccepted)
	}
}

func TestLogging_WriteImplicitHeader(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write without WriteHeader — implicit 200
		w.Write([]byte("data"))
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].Status != http.StatusOK {
		t.Errorf("Status = %d, want %d", spy.entries[0].Status, http.StatusOK)
	}
}

func TestLogging_RequestLoggerError(t *testing.T) {
	// LogRequest returning error should not break the response
	errLogger := &errorRequestLogger{}
	mw := middleware.NewLogging(errLogger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogging_NoWriteAtAll(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler does nothing — no Write, no WriteHeader
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/empty", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].Status != http.StatusOK {
		t.Errorf("Status = %d, want %d (default when nothing written)", spy.entries[0].Status, http.StatusOK)
	}
}

func TestLogging_LogDataEnrichedByHandler(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ld := middleware.LogDataFromContext(r.Context())
		if ld == nil {
			t.Fatal("LogData not found in context")
		}
		ld.Model = "claude-sonnet-4-20250514"
		ld.AccountID = "acc-123"
		ld.AccountName = "main-account"
		ld.InputTokens = 100
		ld.OutputTokens = 50
		ld.Retries = 2
		ld.Stream = false
		ld.Error = ""
		w.WriteHeader(http.StatusOK)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	e := spy.entries[0]
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", e.Model, "claude-sonnet-4-20250514")
	}
	if e.AccountID != "acc-123" {
		t.Errorf("AccountID = %q, want %q", e.AccountID, "acc-123")
	}
	if e.AccountName != "main-account" {
		t.Errorf("AccountName = %q, want %q", e.AccountName, "main-account")
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", e.OutputTokens)
	}
	if e.Retries != 2 {
		t.Errorf("Retries = %d, want 2", e.Retries)
	}
	if e.Stream {
		t.Error("Stream = true, want false")
	}
}

func TestLogging_CreatedAtPopulated(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	before := time.Now().UTC()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].CreatedAt == "" {
		t.Fatal("CreatedAt should not be empty")
	}
	parsed, err := time.Parse(time.RFC3339, spy.entries[0].CreatedAt)
	if err != nil {
		t.Fatalf("CreatedAt parse error: %v", err)
	}
	if parsed.Before(before.Add(-1 * time.Second)) {
		t.Errorf("CreatedAt = %v, expected around %v", parsed, before)
	}
}

func TestLogging_LogDataFromContext_NoLogData(t *testing.T) {
	ld := middleware.LogDataFromContext(context.Background())
	if ld != nil {
		t.Errorf("expected nil, got %v", ld)
	}
}

func TestLogging_LogDataErrorField(t *testing.T) {
	spy := &spyRequestLogger{}
	mw := middleware.NewLogging(spy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ld := middleware.LogDataFromContext(r.Context())
		if ld != nil {
			ld.Error = "provider timeout"
			ld.Model = "claude-sonnet-4-20250514"
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := reqWithAPIKey(httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(spy.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(spy.entries))
	}
	if spy.entries[0].Error != "provider timeout" {
		t.Errorf("Error = %q, want %q", spy.entries[0].Error, "provider timeout")
	}
}

type errorRequestLogger struct{}

func (e *errorRequestLogger) LogRequest(_ context.Context, _ middleware.RequestLogEntry) error {
	return context.DeadlineExceeded
}
