package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// OAuthService abstracts token exchange operations.
type OAuthService interface {
	ExchangeCode(ctx context.Context, cfg oauth.Config, code, codeVerifier string) (oauth.TokenResponse, error)
}

// AccountCreator persists new accounts.
type AccountCreator interface {
	Create(ctx context.Context, account *entity.Account) error
}

// Encryptor encrypts sensitive values before persistence.
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
}

const defaultOAuthBaseURL = "https://api.anthropic.com"

// OAuthHandler handles OAuth authorization flow endpoints.
type OAuthHandler struct {
	cfg          oauth.Config
	oauthService OAuthService
	accountStore AccountCreator
	encryptor    Encryptor
	pendingFlows sync.Map // state → verifier
}

// NewOAuthHandler creates a new OAuthHandler.
func NewOAuthHandler(cfg oauth.Config, svc OAuthService, store AccountCreator, enc Encryptor) *OAuthHandler {
	return &OAuthHandler{
		cfg:          cfg,
		oauthService: svc,
		accountStore: store,
		encryptor:    enc,
	}
}

// RegisterRoutes mounts OAuth routes on the given router.
func (h *OAuthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/oauth", func(r chi.Router) {
		r.Get("/authorize", h.handleAuthorize)
		r.Get("/callback", h.handleCallback)
	})
}

// --- request/response types ---

type authorizeResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type callbackResponse struct {
	Account callbackAccountResponse `json:"account"`
}

type callbackAccountResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// --- handlers ---

func (h *OAuthHandler) handleAuthorize(w http.ResponseWriter, _ *http.Request) {
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "pkce_generation_failed", err.Error())
		return
	}

	state, err := oauth.GenerateState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "state_generation_failed", err.Error())
		return
	}

	h.pendingFlows.Store(state, pkce.Verifier)

	authURL := oauth.AuthorizationURL(h.cfg, pkce, state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authorizeResponse{
		AuthURL: authURL,
		State:   state,
	})
}

func (h *OAuthHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Check for provider-side errors first
	if errCode := q.Get("error"); errCode != "" {
		desc := q.Get("error_description")
		if desc == "" {
			desc = "authorization denied by provider"
		}
		writeError(w, http.StatusBadRequest, "oauth_error", errCode, desc)
		return
	}

	code := q.Get("code")
	state := q.Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "missing_params", "code and state are required")
		return
	}

	// Load and consume verifier atomically
	val, ok := h.pendingFlows.LoadAndDelete(state)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid_state", "unknown or expired state")
		return
	}
	verifier := val.(string)

	// Exchange code for tokens
	tokens, err := h.oauthService.ExchangeCode(r.Context(), h.cfg, code, verifier)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_error", "token_exchange_failed", err.Error())
		return
	}

	// Encrypt access token for storage as API key
	encryptedKey, err := h.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "encryption_failed", "failed to encrypt token")
		return
	}

	// Create account with OAuth token as API key
	name := fmt.Sprintf("oauth-%s", time.Now().UTC().Format("20060102-150405"))

	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		name,
		vo.NewSensitiveString(encryptedKey),
		defaultOAuthBaseURL,
		0,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "account_creation_failed", err.Error())
		return
	}

	if err := h.accountStore.Create(r.Context(), acc); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "account_persist_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(callbackResponse{
		Account: callbackAccountResponse{
			ID:     acc.ID().String(),
			Name:   acc.Name(),
			Status: string(acc.Status()),
		},
	})
}
