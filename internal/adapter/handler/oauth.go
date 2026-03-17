package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/oauth"
)

// OAuthHandler handles OAuth authorization flow endpoints.
type OAuthHandler struct {
	oauthClient *oauth.Client
	oauthCfg    oauth.Config
	pendingAuth sync.Map // state → verifier
}

// NewOAuthHandler creates a new OAuthHandler.
func NewOAuthHandler(client *oauth.Client, cfg oauth.Config) *OAuthHandler {
	return &OAuthHandler{
		oauthClient: client,
		oauthCfg:    cfg,
	}
}

// RegisterRoutes mounts OAuth routes on the given router.
func (h *OAuthHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/oauth/authorize", h.authorize)
	r.Get("/api/oauth/callback", h.callback)
}

// --- response types ---

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// --- handlers ---

func (h *OAuthHandler) authorize(w http.ResponseWriter, r *http.Request) {
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

	h.pendingAuth.Store(state, pkce.Verifier)

	authURL := oauth.AuthorizationURL(h.oauthCfg, pkce, state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *OAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Check for provider-side errors
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
	val, ok := h.pendingAuth.LoadAndDelete(state)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid_state", "invalid or expired state")
		return
	}
	verifier := val.(string)

	// Exchange code for tokens
	tokens, err := h.oauthClient.ExchangeCode(r.Context(), h.oauthCfg, code, verifier)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_error", "token_exchange_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	})
}
