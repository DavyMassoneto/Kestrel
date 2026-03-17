package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
)

var testNow = time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)

// --- mock account store ---

type mockAccountStore struct {
	accounts map[string]*entity.Account
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{accounts: make(map[string]*entity.Account)}
}

func (m *mockAccountStore) FindByID(_ context.Context, id vo.AccountID) (*entity.Account, error) {
	acc, ok := m.accounts[id.String()]
	if !ok {
		return nil, errors.New("account not found")
	}
	return acc, nil
}

func (m *mockAccountStore) FindAll(_ context.Context) ([]*entity.Account, error) {
	var result []*entity.Account
	for _, acc := range m.accounts {
		result = append(result, acc)
	}
	return result, nil
}

func (m *mockAccountStore) Create(_ context.Context, account *entity.Account) error {
	m.accounts[account.ID().String()] = account
	return nil
}

func (m *mockAccountStore) Save(_ context.Context, account *entity.Account) error {
	m.accounts[account.ID().String()] = account
	return nil
}

func (m *mockAccountStore) Delete(_ context.Context, id vo.AccountID) error {
	if _, ok := m.accounts[id.String()]; !ok {
		return errors.New("account not found")
	}
	delete(m.accounts, id.String())
	return nil
}

// --- mock apikey store ---

type mockAPIKeyStore struct {
	keys map[string]*entity.APIKey
}

func newMockAPIKeyStore() *mockAPIKeyStore {
	return &mockAPIKeyStore{keys: make(map[string]*entity.APIKey)}
}

func (m *mockAPIKeyStore) FindByID(_ context.Context, id vo.APIKeyID) (*entity.APIKey, error) {
	k, ok := m.keys[id.String()]
	if !ok {
		return nil, errors.New("API key not found")
	}
	return k, nil
}

func (m *mockAPIKeyStore) FindAll(_ context.Context) ([]*entity.APIKey, error) {
	var result []*entity.APIKey
	for _, k := range m.keys {
		result = append(result, k)
	}
	return result, nil
}

func (m *mockAPIKeyStore) Create(_ context.Context, key *entity.APIKey) error {
	m.keys[key.ID().String()] = key
	return nil
}

func (m *mockAPIKeyStore) Delete(_ context.Context, id vo.APIKeyID) error {
	if _, ok := m.keys[id.String()]; !ok {
		return errors.New("API key not found")
	}
	delete(m.keys, id.String())
	return nil
}

// --- helpers ---

func setupAdminRouter(adminKey string) (*chi.Mux, *mockAccountStore, *mockAPIKeyStore) {
	accStore := newMockAccountStore()
	keyStore := newMockAPIKeyStore()

	accUC := usecase.NewAdminAccountUseCase(accStore)
	keyUC := usecase.NewAdminAPIKeyUseCase(keyStore)

	adminHandler := NewAdminHandler(accUC, keyUC, nil, adminKey)

	r := chi.NewRouter()
	adminHandler.RegisterRoutes(r)
	return r, accStore, keyStore
}

func doRequest(r *chi.Mux, method, path, adminKey string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if adminKey != "" {
		req.Header.Set("X-Admin-Key", adminKey)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- auth tests ---

func TestAdmin_NoAdminKey_Returns401(t *testing.T) {
	r, _, _ := setupAdminRouter("secret-key")

	w := doRequest(r, http.MethodGet, "/admin/accounts", "", nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAdmin_WrongAdminKey_Returns401(t *testing.T) {
	r, _, _ := setupAdminRouter("secret-key")

	w := doRequest(r, http.MethodGet, "/admin/accounts", "wrong-key", nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// --- account tests ---

func TestAdmin_CreateAccount_Success(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	body := map[string]interface{}{
		"name":     "test-acc",
		"api_key":  "sk-ant-api03-test",
		"base_url": "https://api.anthropic.com",
		"priority": 1,
	}

	w := doRequest(r, http.MethodPost, "/admin/accounts", "secret", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["name"] != "test-acc" {
		t.Errorf("name = %v", resp["name"])
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("id should be present")
	}
}

func TestAdmin_CreateAccount_InvalidInput(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	body := map[string]interface{}{
		"name": "",
	}

	w := doRequest(r, http.MethodPost, "/admin/accounts", "secret", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListAccounts_Success(t *testing.T) {
	r, store, _ := setupAdminRouter("secret")

	acc, _ := entity.NewAccount(vo.NewAccountID(), "acc1", vo.NewSensitiveString("sk-key"), "https://api.anthropic.com", 0)
	store.accounts[acc.ID().String()] = acc

	w := doRequest(r, http.MethodGet, "/admin/accounts", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("response should have 'data' array")
	}
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}

	// api_key should NOT be exposed
	accData := data[0].(map[string]interface{})
	if _, hasKey := accData["api_key"]; hasKey {
		t.Error("api_key should NOT be in the response")
	}
}

func TestAdmin_UpdateAccount_Success(t *testing.T) {
	r, store, _ := setupAdminRouter("secret")

	acc, _ := entity.NewAccount(vo.NewAccountID(), "original", vo.NewSensitiveString("sk-key"), "https://api.anthropic.com", 0)
	store.accounts[acc.ID().String()] = acc

	body := map[string]interface{}{
		"name": "updated",
	}

	w := doRequest(r, http.MethodPut, "/admin/accounts/"+acc.ID().String(), "secret", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["name"] != "updated" {
		t.Errorf("name = %v, want 'updated'", resp["name"])
	}
}

func TestAdmin_UpdateAccount_NotFound(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	body := map[string]interface{}{
		"name": "updated",
	}

	w := doRequest(r, http.MethodPut, "/admin/accounts/acc_000000000000000000000", "secret", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestAdmin_DeleteAccount_Success(t *testing.T) {
	r, store, _ := setupAdminRouter("secret")

	acc, _ := entity.NewAccount(vo.NewAccountID(), "to-delete", vo.NewSensitiveString("sk-key"), "https://api.anthropic.com", 0)
	store.accounts[acc.ID().String()] = acc

	w := doRequest(r, http.MethodDelete, "/admin/accounts/"+acc.ID().String(), "secret", nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if len(store.accounts) != 0 {
		t.Errorf("store still has %d accounts", len(store.accounts))
	}
}

func TestAdmin_DeleteAccount_NotFound(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodDelete, "/admin/accounts/acc_000000000000000000000", "secret", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestAdmin_ResetAccount_Success(t *testing.T) {
	r, store, _ := setupAdminRouter("secret")

	acc, _ := entity.NewAccount(vo.NewAccountID(), "cooldown-acc", vo.NewSensitiveString("sk-key"), "https://api.anthropic.com", 0)
	acc.ApplyCooldown(vo.ErrRateLimit, testNow)
	store.accounts[acc.ID().String()] = acc

	w := doRequest(r, http.MethodPost, "/admin/accounts/"+acc.ID().String()+"/reset", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "active" {
		t.Errorf("status = %v, want 'active'", resp["status"])
	}
}

func TestAdmin_ResetAccount_NotFound(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodPost, "/admin/accounts/acc_000000000000000000000/reset", "secret", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- apikey tests ---

func TestAdmin_CreateAPIKey_Success(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	body := map[string]interface{}{
		"name":           "test-key",
		"allowed_models": []string{"claude-sonnet-4-20250514"},
	}

	w := doRequest(r, http.MethodPost, "/admin/keys", "secret", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["name"] != "test-key" {
		t.Errorf("name = %v", resp["name"])
	}
	if resp["key"] == nil || resp["key"] == "" {
		t.Error("key (raw) should be present in creation response")
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("id should be present")
	}
}

func TestAdmin_CreateAPIKey_InvalidInput(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	body := map[string]interface{}{
		"name": "",
	}

	w := doRequest(r, http.MethodPost, "/admin/keys", "secret", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListAPIKeys_Success(t *testing.T) {
	r, _, keyStore := setupAdminRouter("secret")

	key, _ := entity.NewAPIKey(vo.NewAPIKeyID(), "key1", "hash", "prefix12")
	keyStore.keys[key.ID().String()] = key

	w := doRequest(r, http.MethodGet, "/admin/keys", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("response should have 'data' array")
	}
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
}

func TestAdmin_RevokeAPIKey_Success(t *testing.T) {
	r, _, keyStore := setupAdminRouter("secret")

	key, _ := entity.NewAPIKey(vo.NewAPIKeyID(), "to-revoke", "hash", "prefix12")
	keyStore.keys[key.ID().String()] = key

	w := doRequest(r, http.MethodDelete, "/admin/keys/"+key.ID().String(), "secret", nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if len(keyStore.keys) != 0 {
		t.Errorf("store still has %d keys", len(keyStore.keys))
	}
}

func TestAdmin_RevokeAPIKey_NotFound(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodDelete, "/admin/keys/key_000000000000000000000", "secret", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
