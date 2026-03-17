package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// --- mock OAuthService ---

type mockOAuthService struct {
	exchangeFunc func(ctx context.Context, cfg oauth.Config, code, verifier string) (oauth.TokenResponse, error)
}

func (m *mockOAuthService) ExchangeCode(ctx context.Context, cfg oauth.Config, code, verifier string) (oauth.TokenResponse, error) {
	return m.exchangeFunc(ctx, cfg, code, verifier)
}

// --- mock AccountCreator ---

type mockAccountCreator struct {
	created *entity.Account
	err     error
}

func (m *mockAccountCreator) Create(_ context.Context, account *entity.Account) error {
	if m.err != nil {
		return m.err
	}
	m.created = account
	return nil
}

// --- mock Encryptor ---

type mockEncryptor struct {
	encryptFunc func(plaintext string) (string, error)
}

func (m *mockEncryptor) Encrypt(plaintext string) (string, error) {
	if m.encryptFunc != nil {
		return m.encryptFunc(plaintext)
	}
	return "encrypted:" + plaintext, nil
}

// --- helpers ---

func newTestOAuthHandler(svc OAuthService, creator AccountCreator, enc Encryptor) *OAuthHandler {
	cfg := oauth.Config{
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8080/api/oauth/callback",
		AuthURL:     "https://auth.example.com/authorize",
		TokenURL:    "https://auth.example.com/token",
		Scope:       "openid",
	}
	return NewOAuthHandler(cfg, svc, creator, enc)
}

func setupOAuthRouter(h *OAuthHandler) *chi.Mux {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// --- Tests ---

func TestHandleAuthorize_Success(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp authorizeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.AuthURL == "" {
		t.Fatal("expected non-empty auth_url")
	}
	if resp.State == "" {
		t.Fatal("expected non-empty state")
	}

	// Verify state is stored in pending flows
	if _, ok := h.pendingFlows.Load(resp.State); !ok {
		t.Fatal("expected state to be stored in pending flows")
	}
}

func TestHandleCallback_Success(t *testing.T) {
	creator := &mockAccountCreator{}
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, code, verifier string) (oauth.TokenResponse, error) {
			if code != "test-code" {
				t.Fatalf("expected code 'test-code', got %q", code)
			}
			if verifier == "" {
				t.Fatal("expected non-empty verifier")
			}
			return oauth.TokenResponse{
				AccessToken:  "acc-token-123",
				RefreshToken: "ref-token-456",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	enc := &mockEncryptor{}

	h := newTestOAuthHandler(svc, creator, enc)
	r := setupOAuthRouter(h)

	// Pre-store a state with verifier
	testState := "test-state-abc"
	h.pendingFlows.Store(testState, "test-verifier-xyz")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test-code&state="+testState, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp callbackResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Account.ID == "" {
		t.Fatal("expected non-empty account ID")
	}
	if resp.Account.Name == "" {
		t.Fatal("expected non-empty account name")
	}
	if resp.Account.Status != "active" {
		t.Fatalf("expected status 'active', got %q", resp.Account.Status)
	}

	// Verify state was consumed
	if _, ok := h.pendingFlows.Load(testState); ok {
		t.Fatal("expected state to be consumed from pending flows")
	}

	// Verify account was persisted
	if creator.created == nil {
		t.Fatal("expected account to be created")
	}
}

func TestHandleCallback_MissingCode(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?state=some-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCallback_MissingState(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=some-code", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCallback_InvalidState(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=some-code&state=unknown-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var body errorBody
	json.NewDecoder(w.Body).Decode(&body)
	if body.Error.Code != "invalid_state" {
		t.Fatalf("expected code 'invalid_state', got %q", body.Error.Code)
	}
}

func TestHandleCallback_ExchangeError(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{}, &oauth.Error{
				Code:        "invalid_grant",
				Description: "code expired",
				StatusCode:  400,
			}
		},
	}

	h := newTestOAuthHandler(svc, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("valid-state", "some-verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=bad-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}

	// Verify state was consumed even on exchange error
	if _, ok := h.pendingFlows.Load("valid-state"); ok {
		t.Fatal("expected state to be consumed even on error")
	}
}

func TestHandleCallback_EncryptError(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{
				AccessToken:  "token",
				RefreshToken: "refresh",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	enc := &mockEncryptor{
		encryptFunc: func(_ string) (string, error) {
			return "", context.DeadlineExceeded
		},
	}

	h := newTestOAuthHandler(svc, &mockAccountCreator{}, enc)
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("enc-state", "verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=code&state=enc-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleCallback_StoreError(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{
				AccessToken:  "token",
				RefreshToken: "refresh",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	creator := &mockAccountCreator{err: context.DeadlineExceeded}

	h := newTestOAuthHandler(svc, creator, &mockEncryptor{})
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("store-state", "verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=code&state=store-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleAuthorize_ReturnsValidURL(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp authorizeResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// URL should contain the configured auth URL base
	if len(resp.AuthURL) < len("https://auth.example.com/authorize") {
		t.Fatalf("auth_url too short: %s", resp.AuthURL)
	}
}

func TestHandleCallback_ProviderError(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	// Simulate provider returning an error in callback
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?error=access_denied&error_description=user+denied", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var body errorBody
	json.NewDecoder(w.Body).Decode(&body)
	if body.Error.Code != "access_denied" {
		t.Fatalf("expected code 'access_denied', got %q", body.Error.Code)
	}
}

// Verify the account name uses a human-readable format
func TestHandleCallback_AccountName(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{
				AccessToken:  "sk-ant-oauth-test-token",
				RefreshToken: "refresh",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	creator := &mockAccountCreator{}

	h := newTestOAuthHandler(svc, creator, &mockEncryptor{})
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("name-state", "verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=code&state=name-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if creator.created == nil {
		t.Fatal("expected account to be created")
	}

	name := creator.created.Name()
	if name == "" {
		t.Fatal("expected non-empty account name")
	}

	// Name should start with "oauth-"
	if len(name) < 7 || name[:6] != "oauth-" {
		t.Fatalf("expected name to start with 'oauth-', got %q", name)
	}
}

func TestHandleCallback_NewAccountValidationError(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{
				AccessToken:  "token",
				RefreshToken: "refresh",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	// Encryptor returns empty string → NewAccount fails because api_key is required
	enc := &mockEncryptor{
		encryptFunc: func(_ string) (string, error) {
			return "", nil
		},
	}

	h := newTestOAuthHandler(svc, &mockAccountCreator{}, enc)
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("val-state", "verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=code&state=val-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	var body errorBody
	json.NewDecoder(w.Body).Decode(&body)
	if body.Error.Code != "account_creation_failed" {
		t.Fatalf("expected code 'account_creation_failed', got %q", body.Error.Code)
	}
}

func TestHandleCallback_ProviderErrorNoDescription(t *testing.T) {
	h := newTestOAuthHandler(&mockOAuthService{}, &mockAccountCreator{}, &mockEncryptor{})
	r := setupOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?error=server_error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var body errorBody
	json.NewDecoder(w.Body).Decode(&body)
	if body.Error.Message != "authorization denied by provider" {
		t.Fatalf("expected default description, got %q", body.Error.Message)
	}
}

// Verify unused account ID type
func TestHandleCallback_AccountHasValidID(t *testing.T) {
	svc := &mockOAuthService{
		exchangeFunc: func(_ context.Context, _ oauth.Config, _, _ string) (oauth.TokenResponse, error) {
			return oauth.TokenResponse{
				AccessToken:  "token",
				RefreshToken: "refresh",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			}, nil
		},
	}
	creator := &mockAccountCreator{}

	h := newTestOAuthHandler(svc, creator, &mockEncryptor{})
	r := setupOAuthRouter(h)

	h.pendingFlows.Store("id-state", "verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=code&state=id-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if creator.created == nil {
		t.Fatal("expected account to be created")
	}

	id := creator.created.ID()
	if id.String() == "" {
		t.Fatal("expected non-empty account ID")
	}

	// Verify the ID can be parsed back
	if _, err := vo.ParseAccountID(id.String()); err != nil {
		t.Fatalf("expected valid account ID, got parse error: %v", err)
	}
}
