package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
)

// --- mock AccountCreator ---

type mockAccountCreator struct {
	createFunc func(ctx context.Context, input AccountCreateInput) (string, error)
	calls      []AccountCreateInput
}

func (m *mockAccountCreator) Create(ctx context.Context, input AccountCreateInput) (string, error) {
	m.calls = append(m.calls, input)
	if m.createFunc != nil {
		return m.createFunc(ctx, input)
	}
	return "mock-account-id", nil
}

// --- helpers ---

func newTestOAuthCfg() oauth.Config {
	return oauth.Config{
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8080/api/oauth/callback",
		AuthURL:     "https://console.anthropic.com/oauth/authorize",
		TokenURL:    "https://console.anthropic.com/oauth/token",
		Scope:       "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers",
	}
}

func setupTestOAuthRouter(h *OAuthHandler) *chi.Mux {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// --- Tests ---

func TestAuthorize_RedirectsToProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	client := oauth.NewClient(ts.Client())
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	if !strings.Contains(location, "client_id=test-client-id") {
		t.Fatalf("Location missing client_id: %s", location)
	}
	if !strings.Contains(location, "code_challenge=") {
		t.Fatalf("Location missing code_challenge: %s", location)
	}
	if !strings.Contains(location, "state=") {
		t.Fatalf("Location missing state: %s", location)
	}
}

func TestAuthorize_IncludesScopeInURL(t *testing.T) {
	cfg := newTestOAuthCfg()
	cfg.Scope = "org:create_api_key user:profile"
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "scope=") {
		t.Fatalf("Bug #14 regression: Location missing scope parameter: %s", location)
	}
}

func TestAuthorize_StoresStateInPendingAuth(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	location := w.Header().Get("Location")
	stateIdx := strings.Index(location, "state=")
	if stateIdx == -1 {
		t.Fatal("Location header missing state parameter")
	}
	stateVal := location[stateIdx+len("state="):]
	if ampIdx := strings.Index(stateVal, "&"); ampIdx != -1 {
		stateVal = stateVal[:ampIdx]
	}

	val, ok := h.pendingAuth.Load(stateVal)
	if !ok {
		t.Fatal("expected state to be stored in pending auth")
	}
	if val.(string) == "" {
		t.Fatal("expected non-empty verifier stored with state")
	}
}

func TestCallback_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("expected grant_type=authorization_code, got %q", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "test-auth-code" {
			t.Errorf("expected code=test-auth-code, got %q", r.FormValue("code"))
		}
		if r.FormValue("code_verifier") == "" {
			t.Error("expected non-empty code_verifier")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"at-123","refresh_token":"rt-456","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	cfg.TokenURL = ts.URL
	client := oauth.NewClient(ts.Client())
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	testState := "test-state-abc"
	h.pendingAuth.Store(testState, "test-verifier-xyz")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test-auth-code&state="+testState, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", w.Code, w.Body.String())
	}

	location := w.Header().Get("Location")
	if location != "/app/accounts" {
		t.Fatalf("expected redirect to /app/accounts, got %q", location)
	}

	// Verify account was created
	if len(creator.calls) != 1 {
		t.Fatalf("expected 1 account creation call, got %d", len(creator.calls))
	}

	call := creator.calls[0]
	if call.APIKey != "at-123" {
		t.Fatalf("expected APIKey 'at-123', got %q", call.APIKey)
	}
	if call.BaseURL != "https://api.anthropic.com" {
		t.Fatalf("expected BaseURL 'https://api.anthropic.com', got %q", call.BaseURL)
	}
	if call.Priority != 10 {
		t.Fatalf("expected Priority 10, got %d", call.Priority)
	}

	// Verify state was consumed
	if _, ok := h.pendingAuth.Load(testState); ok {
		t.Fatal("expected state to be consumed from pending auth")
	}
}

func TestCallback_MissingCode(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?state=some-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallback_InvalidState(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

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
	if !strings.Contains(body.Error.Message, "invalid or expired state") {
		t.Fatalf("expected message about invalid/expired state, got %q", body.Error.Message)
	}
}

func TestCallback_ExpiredState(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=some-code&state=expired-state", nil)
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

func TestCallback_ExchangeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"code expired"}`)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	cfg.TokenURL = ts.URL
	client := oauth.NewClient(ts.Client())
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	h.pendingAuth.Store("valid-state", "some-verifier")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=bad-code&state=valid-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}

	// Verify state was consumed even on exchange error
	if _, ok := h.pendingAuth.Load("valid-state"); ok {
		t.Fatal("expected state to be consumed even on error")
	}
}

func TestCallback_AccountCreationError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"at-123","refresh_token":"rt-456","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	cfg.TokenURL = ts.URL
	client := oauth.NewClient(ts.Client())
	creator := &mockAccountCreator{
		createFunc: func(_ context.Context, _ AccountCreateInput) (string, error) {
			return "", fmt.Errorf("database is full")
		},
	}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	h.pendingAuth.Store("state-x", "verifier-x")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test-code&state=state-x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCallback_MissingState(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=some-code", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallback_ProviderError(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

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
	if body.Error.Message != "user denied" {
		t.Fatalf("expected message 'user denied', got %q", body.Error.Message)
	}
}

func TestCallback_ProviderErrorNoDescription(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	creator := &mockAccountCreator{}
	h := NewOAuthHandler(client, cfg, creator)
	r := setupTestOAuthRouter(h)

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
