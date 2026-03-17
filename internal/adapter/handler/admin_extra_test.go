package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdmin_CreateAccount_InvalidJSON(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	req := httptest.NewRequest(http.MethodPost, "/admin/accounts", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", "secret")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_UpdateAccount_InvalidJSON(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	req := httptest.NewRequest(http.MethodPut, "/admin/accounts/acc_000000000000000000000", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", "secret")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_UpdateAccount_InvalidID(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodPut, "/admin/accounts/bad-id", "secret", map[string]interface{}{"name": "x"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_DeleteAccount_InvalidID(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodDelete, "/admin/accounts/bad-id", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_ResetAccount_InvalidID(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodPost, "/admin/accounts/bad-id/reset", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_RevokeAPIKey_InvalidID(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	w := doRequest(r, http.MethodDelete, "/admin/keys/bad-id", "secret", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAdmin_CreateAPIKey_InvalidJSON(t *testing.T) {
	r, _, _ := setupAdminRouter("secret")

	req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", "secret")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
