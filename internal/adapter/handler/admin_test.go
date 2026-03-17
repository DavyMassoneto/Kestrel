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

// --- mock request log reader ---

type mockRequestLogReader struct {
	entries []middleware.RequestLogEntry
	total   int
	err     error
	filters middleware.RequestLogFilters
}

func (m *mockRequestLogReader) FindAll(_ context.Context, filters middleware.RequestLogFilters) ([]middleware.RequestLogEntry, int, error) {
	m.filters = filters
	return m.entries, m.total, m.err
}

func setupAdminRouterWithLogs(adminKey string, logReader middleware.RequestLogReader) *chi.Mux {
	accStore := newMockAccountStore()
	keyStore := newMockAPIKeyStore()

	accUC := usecase.NewAdminAccountUseCase(accStore)
	keyUC := usecase.NewAdminAPIKeyUseCase(keyStore)

	adminHandler := NewAdminHandler(accUC, keyUC, logReader, adminKey)

	r := chi.NewRouter()
	adminHandler.RegisterRoutes(r)
	return r
}

// --- request log tests ---

func TestAdmin_ListLogs_Defaults(t *testing.T) {
	logReader := &mockRequestLogReader{
		entries: []middleware.RequestLogEntry{
			{RequestID: "req_1", Status: 200, Model: "claude-sonnet-4-20250514", LatencyMs: 500, CreatedAt: "2026-03-17T12:00:00Z"},
			{RequestID: "req_2", Status: 500, Model: "claude-sonnet-4-20250514", LatencyMs: 1200, CreatedAt: "2026-03-17T12:01:00Z"},
		},
		total: 2,
	}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("response should have 'data' array")
	}
	if len(data) != 2 {
		t.Errorf("data len = %d, want 2", len(data))
	}
	if resp["total"] != float64(2) {
		t.Errorf("total = %v, want 2", resp["total"])
	}
	if resp["limit"] != float64(50) {
		t.Errorf("limit = %v, want 50", resp["limit"])
	}
	if resp["offset"] != float64(0) {
		t.Errorf("offset = %v, want 0", resp["offset"])
	}

	// Verify default filters passed to reader
	if logReader.filters.Limit != 50 {
		t.Errorf("filters.Limit = %d, want 50", logReader.filters.Limit)
	}
	if logReader.filters.Offset != 0 {
		t.Errorf("filters.Offset = %d, want 0", logReader.filters.Offset)
	}
}

func TestAdmin_ListLogs_Pagination(t *testing.T) {
	logReader := &mockRequestLogReader{
		entries: []middleware.RequestLogEntry{
			{RequestID: "req_3", Status: 200},
		},
		total: 100,
	}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?limit=10&offset=5", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["limit"] != float64(10) {
		t.Errorf("limit = %v, want 10", resp["limit"])
	}
	if resp["offset"] != float64(5) {
		t.Errorf("offset = %v, want 5", resp["offset"])
	}
	if logReader.filters.Limit != 10 {
		t.Errorf("filters.Limit = %d, want 10", logReader.filters.Limit)
	}
	if logReader.filters.Offset != 5 {
		t.Errorf("filters.Offset = %d, want 5", logReader.filters.Offset)
	}
}

func TestAdmin_ListLogs_LimitCappedAt500(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?limit=1000", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["limit"] != float64(500) {
		t.Errorf("limit = %v, want 500", resp["limit"])
	}
}

func TestAdmin_ListLogs_NegativeLimitDefaultsTo50(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?limit=-1", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["limit"] != float64(50) {
		t.Errorf("limit = %v, want 50", resp["limit"])
	}
}

func TestAdmin_ListLogs_FilterByStatus(t *testing.T) {
	logReader := &mockRequestLogReader{
		entries: []middleware.RequestLogEntry{
			{RequestID: "req_ok", Status: 200},
		},
		total: 1,
	}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?status=200", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if logReader.filters.Status == nil || *logReader.filters.Status != 200 {
		t.Errorf("filters.Status = %v, want 200", logReader.filters.Status)
	}
}

func TestAdmin_ListLogs_FilterByAccountID(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?account_id=acc_123", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if logReader.filters.AccountID == nil || *logReader.filters.AccountID != "acc_123" {
		t.Errorf("filters.AccountID = %v, want acc_123", logReader.filters.AccountID)
	}
}

func TestAdmin_ListLogs_FilterByAPIKeyID(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?api_key_id=key_456", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if logReader.filters.APIKeyID == nil || *logReader.filters.APIKeyID != "key_456" {
		t.Errorf("filters.APIKeyID = %v, want key_456", logReader.filters.APIKeyID)
	}
}

func TestAdmin_ListLogs_FilterByModel(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?model=claude-sonnet-4-20250514", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if logReader.filters.Model == nil || *logReader.filters.Model != "claude-sonnet-4-20250514" {
		t.Errorf("filters.Model = %v, want claude-sonnet-4-20250514", logReader.filters.Model)
	}
}

func TestAdmin_ListLogs_FilterByDateRange(t *testing.T) {
	logReader := &mockRequestLogReader{total: 0}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?from=2026-03-16T00:00:00Z&to=2026-03-17T23:59:59Z", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	expectedFrom := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	expectedTo := time.Date(2026, 3, 17, 23, 59, 59, 0, time.UTC)

	if logReader.filters.From == nil || !logReader.filters.From.Equal(expectedFrom) {
		t.Errorf("filters.From = %v, want %v", logReader.filters.From, expectedFrom)
	}
	if logReader.filters.To == nil || !logReader.filters.To.Equal(expectedTo) {
		t.Errorf("filters.To = %v, want %v", logReader.filters.To, expectedTo)
	}
}

func TestAdmin_ListLogs_InvalidLimit(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?limit=abc", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListLogs_InvalidOffset(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?offset=abc", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListLogs_InvalidStatus(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?status=abc", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListLogs_InvalidFromDate(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?from=not-a-date", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListLogs_InvalidToDate(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs?to=not-a-date", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ListLogs_ReaderError(t *testing.T) {
	logReader := &mockRequestLogReader{err: errors.New("db error")}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs", "secret", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestAdmin_ListLogs_EmptyResult(t *testing.T) {
	logReader := &mockRequestLogReader{
		entries: []middleware.RequestLogEntry{},
		total:   0,
	}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("response should have 'data' array")
	}
	if len(data) != 0 {
		t.Errorf("data len = %d, want 0", len(data))
	}
	if resp["total"] != float64(0) {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestAdmin_ListLogs_ResponseFields(t *testing.T) {
	logReader := &mockRequestLogReader{
		entries: []middleware.RequestLogEntry{
			{
				RequestID:    "req_abc",
				APIKeyName:   "my-key",
				AccountName:  "acc-1",
				Model:        "claude-sonnet-4-20250514",
				Status:       200,
				InputTokens:  100,
				OutputTokens: 50,
				LatencyMs:    350,
				Retries:      1,
				Stream:       true,
				CreatedAt:    "2026-03-17T12:00:00Z",
			},
		},
		total: 1,
	}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs", "secret", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].([]interface{})
	entry := data[0].(map[string]interface{})

	if entry["id"] != "req_abc" {
		t.Errorf("id = %v", entry["id"])
	}
	if entry["api_key_name"] != "my-key" {
		t.Errorf("api_key_name = %v", entry["api_key_name"])
	}
	if entry["account_name"] != "acc-1" {
		t.Errorf("account_name = %v", entry["account_name"])
	}
	if entry["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v", entry["model"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("status = %v", entry["status"])
	}
	if entry["input_tokens"] != float64(100) {
		t.Errorf("input_tokens = %v", entry["input_tokens"])
	}
	if entry["output_tokens"] != float64(50) {
		t.Errorf("output_tokens = %v", entry["output_tokens"])
	}
	if entry["latency_ms"] != float64(350) {
		t.Errorf("latency_ms = %v", entry["latency_ms"])
	}
	if entry["retries"] != float64(1) {
		t.Errorf("retries = %v", entry["retries"])
	}
	if entry["stream"] != true {
		t.Errorf("stream = %v", entry["stream"])
	}
	if entry["created_at"] != "2026-03-17T12:00:00Z" {
		t.Errorf("created_at = %v", entry["created_at"])
	}
}

func TestAdmin_ListLogs_RequiresAuth(t *testing.T) {
	logReader := &mockRequestLogReader{}
	r := setupAdminRouterWithLogs("secret", logReader)

	w := doRequest(r, http.MethodGet, "/admin/logs", "", nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
