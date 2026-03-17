package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
)

func TestGeneratePKCE_VerifierLength(t *testing.T) {
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if len(pkce.Verifier) < 43 || len(pkce.Verifier) > 128 {
		t.Errorf("verifier length = %d; want 43-128", len(pkce.Verifier))
	}
}

func TestGeneratePKCE_ChallengeNotEmpty(t *testing.T) {
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if pkce.Challenge == "" {
		t.Error("challenge should not be empty")
	}
	if pkce.ChallengeMethod != "S256" {
		t.Errorf("challenge method = %q; want S256", pkce.ChallengeMethod)
	}
}

func TestGeneratePKCE_Uniqueness(t *testing.T) {
	a, _ := oauth.GeneratePKCE()
	b, _ := oauth.GeneratePKCE()
	if a.Verifier == b.Verifier {
		t.Error("two PKCE verifiers should not be equal")
	}
}

func TestGeneratePKCE_ChallengeIsBase64URL(t *testing.T) {
	pkce, _ := oauth.GeneratePKCE()
	for _, c := range pkce.Challenge {
		if c == '+' || c == '/' || c == '=' {
			t.Errorf("challenge contains non-base64url char %q", string(c))
		}
	}
}

func TestAuthorizationURL(t *testing.T) {
	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost:8080/api/oauth/callback",
		AuthURL:     "https://console.anthropic.com/oauth/authorize",
	}

	pkce := oauth.PKCE{
		Challenge:       "test-challenge",
		ChallengeMethod: "S256",
	}

	url := oauth.AuthorizationURL(cfg, pkce, "random-state")

	// Verify URL contains required params
	tests := []struct {
		param string
		value string
	}{
		{"client_id", "client-123"},
		{"redirect_uri", "http://localhost:8080/api/oauth/callback"},
		{"response_type", "code"},
		{"code_challenge", "test-challenge"},
		{"code_challenge_method", "S256"},
		{"state", "random-state"},
	}

	for _, tt := range tests {
		if !containsParam(url, tt.param, tt.value) {
			t.Errorf("URL missing %s=%s\nURL: %s", tt.param, tt.value, url)
		}
	}
}

func TestAuthorizationURL_WithScope(t *testing.T) {
	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost:8080/api/oauth/callback",
		AuthURL:     "https://console.anthropic.com/oauth/authorize",
		Scope:       "org:read user:read",
	}

	pkce := oauth.PKCE{
		Challenge:       "test-challenge",
		ChallengeMethod: "S256",
	}

	u := oauth.AuthorizationURL(cfg, pkce, "random-state")

	// url.Values.Encode uses + for spaces in query strings
	if !contains(u, "scope=org%3Aread+user%3Aread") {
		t.Errorf("URL missing scope param\nURL: %s", u)
	}
}

func TestExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q; want application/x-www-form-urlencoded", ct)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q; want authorization_code", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code") != "auth-code-123" {
			t.Errorf("code = %q; want auth-code-123", r.Form.Get("code"))
		}
		if r.Form.Get("client_id") != "client-123" {
			t.Errorf("client_id = %q; want client-123", r.Form.Get("client_id"))
		}
		if r.Form.Get("redirect_uri") != "http://localhost/callback" {
			t.Errorf("redirect_uri = %q; want http://localhost/callback", r.Form.Get("redirect_uri"))
		}
		if r.Form.Get("code_verifier") != "verifier-abc" {
			t.Errorf("code_verifier = %q; want verifier-abc", r.Form.Get("code_verifier"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-token-xyz",
			"refresh_token": "refresh-token-abc",
			"token_type":    "bearer",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	tokens, err := client.ExchangeCode(context.Background(), cfg, "auth-code-123", "verifier-abc")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "access-token-xyz" {
		t.Errorf("AccessToken = %q; want access-token-xyz", tokens.AccessToken)
	}
	if tokens.RefreshToken != "refresh-token-abc" {
		t.Errorf("RefreshToken = %q; want refresh-token-abc", tokens.RefreshToken)
	}
	if tokens.TokenType != "bearer" {
		t.Errorf("TokenType = %q; want bearer", tokens.TokenType)
	}
	if tokens.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d; want 3600", tokens.ExpiresIn)
	}
	if tokens.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
}

func TestExchangeCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "code expired",
		})
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.ExchangeCode(context.Background(), cfg, "bad-code", "verifier")
	if err == nil {
		t.Fatal("expected error for bad grant")
	}

	var oauthErr *oauth.Error
	if !isOAuthError(err, &oauthErr) {
		t.Fatalf("expected *oauth.Error, got %T: %v", err, err)
	}
	if oauthErr.Code != "invalid_grant" {
		t.Errorf("error code = %q; want invalid_grant", oauthErr.Code)
	}
}

func TestExchangeCode_NetworkError(t *testing.T) {
	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    "http://localhost:1/nonexistent",
	}

	client := oauth.NewClient(http.DefaultClient)

	_, err := client.ExchangeCode(context.Background(), cfg, "code", "verifier")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q; want refresh_token", r.Form.Get("grant_type"))
		}
		if r.Form.Get("refresh_token") != "refresh-old" {
			t.Errorf("refresh_token = %q; want refresh-old", r.Form.Get("refresh_token"))
		}
		if r.Form.Get("client_id") != "client-123" {
			t.Errorf("client_id = %q; want client-123", r.Form.Get("client_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-new",
			"refresh_token": "refresh-new",
			"token_type":    "bearer",
			"expires_in":    7200,
		})
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID: "client-123",
		TokenURL: srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	tokens, err := client.RefreshToken(context.Background(), cfg, "refresh-old")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if tokens.AccessToken != "access-new" {
		t.Errorf("AccessToken = %q; want access-new", tokens.AccessToken)
	}
	if tokens.RefreshToken != "refresh-new" {
		t.Errorf("RefreshToken = %q; want refresh-new", tokens.RefreshToken)
	}
	if tokens.ExpiresIn != 7200 {
		t.Errorf("ExpiresIn = %d; want 7200", tokens.ExpiresIn)
	}
}

func TestRefreshToken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "refresh token revoked",
		})
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID: "client-123",
		TokenURL: srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.RefreshToken(context.Background(), cfg, "revoked-token")
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
	var oauthErr *oauth.Error
	if !isOAuthError(err, &oauthErr) {
		t.Fatalf("expected *oauth.Error, got %T: %v", err, err)
	}
}

func TestRefreshToken_NetworkError(t *testing.T) {
	cfg := oauth.Config{
		ClientID: "client-123",
		TokenURL: "http://localhost:1/nonexistent",
	}

	client := oauth.NewClient(http.DefaultClient)

	_, err := client.RefreshToken(context.Background(), cfg, "token")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestExchangeCode_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.ExchangeCode(context.Background(), cfg, "code", "verifier")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRefreshToken_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID: "client-123",
		TokenURL: srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.RefreshToken(context.Background(), cfg, "token")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTokenResponse_ExpiresAt(t *testing.T) {
	before := time.Now()
	tokens := oauth.TokenResponse{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "bearer",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(3600 * time.Second),
	}
	after := time.Now()

	if tokens.ExpiresAt.Before(before.Add(3599 * time.Second)) {
		t.Error("ExpiresAt is too early")
	}
	if tokens.ExpiresAt.After(after.Add(3601 * time.Second)) {
		t.Error("ExpiresAt is too late")
	}
}

func TestExchangeCode_ErrorResponseNonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.ExchangeCode(context.Background(), cfg, "code", "verifier")
	if err == nil {
		t.Fatal("expected error for server error")
	}
	var oauthErr *oauth.Error
	if !isOAuthError(err, &oauthErr) {
		t.Fatalf("expected *oauth.Error, got %T: %v", err, err)
	}
	if oauthErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d; want 500", oauthErr.StatusCode)
	}
}

func TestOAuthError_ErrorWithDescription(t *testing.T) {
	err := &oauth.Error{
		Code:        "invalid_grant",
		Description: "code expired",
		StatusCode:  400,
	}
	s := err.Error()
	if !contains(s, "invalid_grant") || !contains(s, "code expired") {
		t.Errorf("Error() = %q; want to contain code and description", s)
	}
}

func TestOAuthError_ErrorWithoutDescription(t *testing.T) {
	err := &oauth.Error{
		Code:       "server_error",
		StatusCode: 500,
	}
	s := err.Error()
	if !contains(s, "server_error") {
		t.Errorf("Error() = %q; want to contain server_error", s)
	}
	if contains(s, ": :") {
		t.Errorf("Error() = %q; should not have empty description separator", s)
	}
}

func TestExchangeCode_InvalidTokenURL(t *testing.T) {
	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    "://invalid-url",
	}

	client := oauth.NewClient(http.DefaultClient)

	_, err := client.ExchangeCode(context.Background(), cfg, "code", "verifier")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestRefreshToken_InvalidTokenURL(t *testing.T) {
	cfg := oauth.Config{
		ClientID: "client-123",
		TokenURL: "://invalid-url",
	}

	client := oauth.NewClient(http.DefaultClient)

	_, err := client.RefreshToken(context.Background(), cfg, "token")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestExchangeCode_ErrorEmptyCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// Return JSON with empty error field
		json.NewEncoder(w).Encode(map[string]string{
			"error_description": "missing code",
		})
	}))
	defer srv.Close()

	cfg := oauth.Config{
		ClientID:    "client-123",
		RedirectURI: "http://localhost/callback",
		TokenURL:    srv.URL,
	}

	client := oauth.NewClient(srv.Client())

	_, err := client.ExchangeCode(context.Background(), cfg, "", "verifier")
	if err == nil {
		t.Fatal("expected error")
	}
	var oauthErr *oauth.Error
	if !isOAuthError(err, &oauthErr) {
		t.Fatalf("expected *oauth.Error, got %T", err)
	}
	// When error field is empty, it should default to "server_error"
	if oauthErr.Code != "server_error" {
		t.Errorf("Code = %q; want server_error", oauthErr.Code)
	}
}

func TestGenerateState(t *testing.T) {
	state, err := oauth.GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	if len(state) < 16 {
		t.Errorf("state length = %d; want >= 16", len(state))
	}

	state2, _ := oauth.GenerateState()
	if state == state2 {
		t.Error("two states should not be equal")
	}
}

// helpers

func containsParam(url, key, value string) bool {
	// Simple check — URL should contain key=value (URL-encoded)
	encoded := key + "=" + encodeParam(value)
	return contains(url, encoded)
}

func encodeParam(s string) string {
	// URL encode manually for test comparison
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			result = append(result, c)
		} else {
			result = append(result, '%')
			result = append(result, "0123456789ABCDEF"[c>>4])
			result = append(result, "0123456789ABCDEF"[c&0x0f])
		}
	}
	return string(result)
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~'
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func isOAuthError(err error, target **oauth.Error) bool {
	if e, ok := err.(*oauth.Error); ok {
		*target = e
		return true
	}
	return false
}
