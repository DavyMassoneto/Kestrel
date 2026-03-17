package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
)

func newTestOAuthCfg() oauth.Config {
	return oauth.Config{
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8080/api/oauth/callback",
		AuthURL:     "https://console.anthropic.com/oauth/authorize",
		TokenURL:    "https://console.anthropic.com/oauth/token",
		Scope:       "openid",
	}
}

func setupTestOAuthRouter(h *OAuthHandler) *chi.Mux {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// --- Tests ---

func TestAuthorize_RedirectsToProvider(t *testing.T) {
	// Use a mock token server (won't be called in authorize)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	client := oauth.NewClient(ts.Client())
	h := NewOAuthHandler(client, cfg)
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

func TestCallback_Success(t *testing.T) {
	// Mock token endpoint
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
	h := NewOAuthHandler(client, cfg)
	r := setupTestOAuthRouter(h)

	// Pre-store a pending flow
	testState := "test-state-abc"
	h.pendingAuth.Store(testState, "test-verifier-xyz")

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test-auth-code&state="+testState, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp tokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.AccessToken != "at-123" {
		t.Fatalf("expected access_token 'at-123', got %q", resp.AccessToken)
	}
	if resp.RefreshToken != "rt-456" {
		t.Fatalf("expected refresh_token 'rt-456', got %q", resp.RefreshToken)
	}
	if resp.TokenType != "bearer" {
		t.Fatalf("expected token_type 'bearer', got %q", resp.TokenType)
	}
	if resp.ExpiresIn != 3600 {
		t.Fatalf("expected expires_in 3600, got %d", resp.ExpiresIn)
	}

	// Verify state was consumed
	if _, ok := h.pendingAuth.Load(testState); ok {
		t.Fatal("expected state to be consumed from pending auth")
	}
}

func TestCallback_MissingCode(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	h := NewOAuthHandler(client, cfg)
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
	h := NewOAuthHandler(client, cfg)
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
	h := NewOAuthHandler(client, cfg)
	r := setupTestOAuthRouter(h)

	// Never stored this state — simulates expiry
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
	// Mock token endpoint that returns an OAuth error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"code expired"}`)
	}))
	defer ts.Close()

	cfg := newTestOAuthCfg()
	cfg.TokenURL = ts.URL
	client := oauth.NewClient(ts.Client())
	h := NewOAuthHandler(client, cfg)
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

func TestCallback_MissingState(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	h := NewOAuthHandler(client, cfg)
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
	h := NewOAuthHandler(client, cfg)
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
	h := NewOAuthHandler(client, cfg)
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

func TestAuthorize_StoresStateInPendingAuth(t *testing.T) {
	cfg := newTestOAuthCfg()
	client := oauth.NewClient(http.DefaultClient)
	h := NewOAuthHandler(client, cfg)
	r := setupTestOAuthRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Extract state from Location header
	location := w.Header().Get("Location")
	// Find state= in the URL
	stateIdx := strings.Index(location, "state=")
	if stateIdx == -1 {
		t.Fatal("Location header missing state parameter")
	}
	stateVal := location[stateIdx+len("state="):]
	if ampIdx := strings.Index(stateVal, "&"); ampIdx != -1 {
		stateVal = stateVal[:ampIdx]
	}

	// Verify state is stored in pending auth
	val, ok := h.pendingAuth.Load(stateVal)
	if !ok {
		t.Fatal("expected state to be stored in pending auth")
	}
	if val.(string) == "" {
		t.Fatal("expected non-empty verifier stored with state")
	}
}
