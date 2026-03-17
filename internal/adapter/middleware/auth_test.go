package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"golang.org/x/crypto/bcrypt"
)

// --- mock authenticator ---

type mockAuthenticator struct {
	key *entity.APIKey
	err error
}

func (m *mockAuthenticator) Execute(_ context.Context, _ string) (*entity.APIKey, error) {
	return m.key, m.err
}

// --- helpers ---

func makeTestKey(t *testing.T) *entity.APIKey {
	t.Helper()
	raw := "omni-testkey1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.MinCost)
	key, err := entity.NewAPIKey(vo.NewAPIKeyID(), "test", string(hash), raw[:12])
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	return key
}

type authErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func assertAuthError(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
	var body authErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Type != "authentication_error" {
		t.Errorf("error.type = %q; want authentication_error", body.Error.Type)
	}
	if body.Error.Code != "invalid_api_key" {
		t.Errorf("error.code = %q; want invalid_api_key", body.Error.Code)
	}
}

func TestAuth_NoAuthorizationHeader(t *testing.T) {
	auth := &mockAuthenticator{}
	mw := middleware.Auth(auth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	assertAuthError(t, rec)
}

func TestAuth_InvalidAuthorizationFormat(t *testing.T) {
	auth := &mockAuthenticator{}
	mw := middleware.Auth(auth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Basic abc123")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	assertAuthError(t, rec)
}

func TestAuth_BearerTokenEmpty(t *testing.T) {
	auth := &mockAuthenticator{}
	mw := middleware.Auth(auth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer ")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	assertAuthError(t, rec)
}

func TestAuth_UseCaseReturnsError(t *testing.T) {
	auth := &mockAuthenticator{err: errs.ErrInvalidAPIKey}
	mw := middleware.Auth(auth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer omni-invalidtoken")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	assertAuthError(t, rec)
}

func TestAuth_ValidToken(t *testing.T) {
	key := makeTestKey(t)
	auth := &mockAuthenticator{key: key}
	mw := middleware.Auth(auth)

	var calledNext bool
	var ctxKey *entity.APIKey

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer omni-validtoken123")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		ctxKey = middleware.APIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !calledNext {
		t.Fatal("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
	if ctxKey == nil {
		t.Fatal("APIKey not found in context")
	}
	if ctxKey.ID() != key.ID() {
		t.Errorf("context APIKey ID = %v; want %v", ctxKey.ID(), key.ID())
	}
}

func TestAuth_PopulatesAPIKeyIDAndName(t *testing.T) {
	key := makeTestKey(t)
	auth := &mockAuthenticator{key: key}
	mw := middleware.Auth(auth)

	var gotID, gotName string

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer omni-validtoken123")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = middleware.GetAPIKeyID(r.Context())
		gotName = middleware.GetAPIKeyName(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if gotID != key.ID().String() {
		t.Errorf("GetAPIKeyID = %q; want %q", gotID, key.ID().String())
	}
	if gotName != key.Name() {
		t.Errorf("GetAPIKeyName = %q; want %q", gotName, key.Name())
	}
}

func TestAPIKeyFromContext_NoKey(t *testing.T) {
	key := middleware.APIKeyFromContext(context.Background())
	if key != nil {
		t.Errorf("expected nil, got %v", key)
	}
}

func TestAuth_UseCaseGenericError(t *testing.T) {
	auth := &mockAuthenticator{err: errors.New("db connection lost")}
	mw := middleware.Auth(auth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer omni-sometoken12345")
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	assertAuthError(t, rec)
}
