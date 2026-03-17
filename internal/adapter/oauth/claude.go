package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds OAuth configuration for the Anthropic provider.
type Config struct {
	ClientID    string
	RedirectURI string
	AuthURL     string
	TokenURL    string
	Scope       string
}

// PKCE holds the PKCE code verifier and challenge pair.
type PKCE struct {
	Verifier        string
	Challenge       string
	ChallengeMethod string
}

// TokenResponse holds the tokens returned from the OAuth token endpoint.
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"-"`
}

// Error represents an OAuth error response.
type Error struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
	StatusCode  int    `json:"-"`
}

func (e *Error) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth: %s: %s (HTTP %d)", e.Code, e.Description, e.StatusCode)
	}
	return fmt.Sprintf("oauth: %s (HTTP %d)", e.Code, e.StatusCode)
}

// Client executes OAuth token operations via HTTP.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new OAuth client.
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// GeneratePKCE generates a cryptographically random PKCE code_verifier
// and its S256 code_challenge.
func GeneratePKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, fmt.Errorf("oauth: generate verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return PKCE{
		Verifier:        verifier,
		Challenge:       challenge,
		ChallengeMethod: "S256",
	}, nil
}

// GenerateState generates a cryptographically random state parameter.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizationURL builds the authorization URL for the OAuth flow.
func AuthorizationURL(cfg Config, pkce PKCE, state string) string {
	params := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.RedirectURI},
		"response_type":         {"code"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {pkce.ChallengeMethod},
		"state":                 {state},
	}
	if cfg.Scope != "" {
		params.Set("scope", cfg.Scope)
	}
	return cfg.AuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *Client) ExchangeCode(ctx context.Context, cfg Config, code, codeVerifier string) (TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {cfg.ClientID},
		"redirect_uri":  {cfg.RedirectURI},
		"code_verifier": {codeVerifier},
	}
	return c.doTokenRequest(ctx, cfg.TokenURL, data)
}

// RefreshToken exchanges a refresh token for new tokens.
func (c *Client) RefreshToken(ctx context.Context, cfg Config, refreshToken string) (TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClientID},
	}
	return c.doTokenRequest(ctx, cfg.TokenURL, data)
}

func (c *Client) doTokenRequest(ctx context.Context, tokenURL string, data url.Values) (TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		oauthErr := &Error{StatusCode: resp.StatusCode}
		if json.Unmarshal(body, oauthErr) != nil {
			oauthErr.Code = "server_error"
			oauthErr.Description = string(body)
		}
		if oauthErr.Code == "" {
			oauthErr.Code = "server_error"
		}
		return TokenResponse{}, oauthErr
	}

	var tokens TokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: decode response: %w", err)
	}

	tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	return tokens, nil
}
