package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
)

// AdminHandler handles admin CRUD endpoints for accounts and API keys.
type AdminHandler struct {
	accountUC *usecase.AdminAccountUseCase
	apiKeyUC  *usecase.AdminAPIKeyUseCase
	logReader middleware.RequestLogReader
	adminKey  string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(accountUC *usecase.AdminAccountUseCase, apiKeyUC *usecase.AdminAPIKeyUseCase, logReader middleware.RequestLogReader, adminKey string) *AdminHandler {
	return &AdminHandler{
		accountUC: accountUC,
		apiKeyUC:  apiKeyUC,
		logReader: logReader,
		adminKey:  adminKey,
	}
}

// RegisterRoutes mounts admin routes on the given router.
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Route("/admin", func(r chi.Router) {
		r.Use(h.authMiddleware)

		r.Post("/accounts", h.createAccount)
		r.Get("/accounts", h.listAccounts)
		r.Put("/accounts/{id}", h.updateAccount)
		r.Delete("/accounts/{id}", h.deleteAccount)
		r.Post("/accounts/{id}/reset", h.resetAccount)

		r.Post("/keys", h.createAPIKey)
		r.Get("/keys", h.listAPIKeys)
		r.Delete("/keys/{id}", h.revokeAPIKey)

		r.Get("/logs", h.listLogs)
	})
}

func (h *AdminHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Admin-Key")
		if key == "" || key != h.adminKey {
			writeError(w, http.StatusUnauthorized, "authentication_error", "invalid_admin_key", "invalid or missing admin key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Account endpoints ---

type createAccountRequest struct {
	Name     string `json:"name"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Priority int    `json:"priority"`
}

type accountResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	BaseURL       string  `json:"base_url"`
	Status        string  `json:"status"`
	Priority      int     `json:"priority"`
	CooldownUntil *string `json:"cooldown_until"`
	BackoffLevel  int     `json:"backoff_level"`
	LastError     *string `json:"last_error"`
}

func accountToResponse(acc *entity.Account) accountResponse {
	var cooldownUntil *string
	if t := acc.CooldownUntil(); t != nil {
		s := t.Format("2006-01-02T15:04:05Z")
		cooldownUntil = &s
	}

	return accountResponse{
		ID:            acc.ID().String(),
		Name:          acc.Name(),
		BaseURL:       acc.BaseURL(),
		Status:        string(acc.Status()),
		Priority:      acc.Priority(),
		CooldownUntil: cooldownUntil,
		BackoffLevel:  acc.BackoffLevel(),
		LastError:     acc.LastError(),
	}
}

func (h *AdminHandler) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid JSON")
		return
	}

	input := usecase.CreateAccountInput{
		Name:     req.Name,
		APIKey:   req.APIKey,
		BaseURL:  req.BaseURL,
		Priority: req.Priority,
	}

	acc, err := h.accountUC.Create(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(accountToResponse(acc))
}

func (h *AdminHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.accountUC.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	data := make([]accountResponse, len(accounts))
	for i, acc := range accounts {
		data[i] = accountToResponse(acc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

type updateAccountRequest struct {
	Name     *string `json:"name"`
	APIKey   *string `json:"api_key"`
	BaseURL  *string `json:"base_url"`
	Priority *int    `json:"priority"`
}

func (h *AdminHandler) updateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := vo.ParseAccountID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid account ID")
		return
	}

	var req updateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid JSON")
		return
	}

	input := usecase.UpdateAccountInput{
		Name:     req.Name,
		APIKey:   req.APIKey,
		BaseURL:  req.BaseURL,
		Priority: req.Priority,
	}

	acc, err := h.accountUC.Update(r.Context(), id, input)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "invalid_request_error", "not_found", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountToResponse(acc))
}

func (h *AdminHandler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := vo.ParseAccountID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid account ID")
		return
	}

	if err := h.accountUC.Delete(r.Context(), id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "invalid_request_error", "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) resetAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := vo.ParseAccountID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid account ID")
		return
	}

	acc, err := h.accountUC.Reset(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "invalid_request_error", "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountToResponse(acc))
}

// --- API Key endpoints ---

type createAPIKeyRequest struct {
	Name          string   `json:"name"`
	AllowedModels []string `json:"allowed_models"`
}

type apiKeyResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Prefix        string   `json:"prefix"`
	IsActive      bool     `json:"is_active"`
	AllowedModels []string `json:"allowed_models"`
}

type createAPIKeyResponse struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	AllowedModels []string `json:"allowed_models"`
}

func apiKeyToResponse(key *entity.APIKey) apiKeyResponse {
	models := key.AllowedModels()
	if models == nil {
		models = []string{}
	}
	return apiKeyResponse{
		ID:            key.ID().String(),
		Name:          key.Name(),
		Prefix:        key.KeyPrefix(),
		IsActive:      key.IsActive(),
		AllowedModels: models,
	}
}

func (h *AdminHandler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid JSON")
		return
	}

	input := usecase.CreateAPIKeyInput{
		Name:          req.Name,
		AllowedModels: req.AllowedModels,
	}

	key, rawKey, err := h.apiKeyUC.Create(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", err.Error())
		return
	}

	models := key.AllowedModels()
	if models == nil {
		models = []string{}
	}

	resp := createAPIKeyResponse{
		ID:            key.ID().String(),
		Key:           rawKey,
		Name:          key.Name(),
		AllowedModels: models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.apiKeyUC.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	data := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		data[i] = apiKeyToResponse(k)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func (h *AdminHandler) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := vo.ParseAPIKeyID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid API key ID")
		return
	}

	if err := h.apiKeyUC.Revoke(r.Context(), id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "invalid_request_error", "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Request Log endpoints ---

type logEntryResponse struct {
	ID           string `json:"id"`
	APIKeyName   string `json:"api_key_name"`
	AccountName  string `json:"account_name"`
	Model        string `json:"model"`
	Status       int    `json:"status"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	LatencyMs    int64  `json:"latency_ms"`
	Retries      int    `json:"retries"`
	Stream       bool   `json:"stream"`
	CreatedAt    string `json:"created_at"`
}

func (h *AdminHandler) listLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := middleware.RequestLogFilters{
		Limit:  50,
		Offset: 0,
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid limit")
			return
		}
		filters.Limit = n
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}

	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid offset")
			return
		}
		filters.Offset = n
	}

	if v := q.Get("status"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid status")
			return
		}
		filters.Status = &n
	}

	if v := q.Get("account_id"); v != "" {
		filters.AccountID = &v
	}
	if v := q.Get("api_key_id"); v != "" {
		filters.APIKeyID = &v
	}
	if v := q.Get("model"); v != "" {
		filters.Model = &v
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid from date, expected ISO 8601")
			return
		}
		filters.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid to date, expected ISO 8601")
			return
		}
		filters.To = &t
	}

	entries, total, err := h.logReader.FindAll(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
		return
	}

	data := make([]logEntryResponse, len(entries))
	for i, e := range entries {
		data[i] = logEntryResponse{
			ID:           e.RequestID,
			APIKeyName:   e.APIKeyName,
			AccountName:  e.AccountName,
			Model:        e.Model,
			Status:       e.Status,
			InputTokens:  e.InputTokens,
			OutputTokens: e.OutputTokens,
			LatencyMs:    e.LatencyMs,
			Retries:      e.Retries,
			Stream:       e.Stream,
			CreatedAt:    e.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   data,
		"total":  total,
		"limit":  filters.Limit,
		"offset": filters.Offset,
	})
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}
