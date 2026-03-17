package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
)

func TestHealthHandler_StatusOK(t *testing.T) {
	h := handler.NewHealth(time.Now())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	h := handler.NewHealth(time.Now())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}
}

func TestHealthHandler_BodyJSON(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	h := handler.NewHealth(start)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	var body struct {
		Status        string  `json:"status"`
		Version       string  `json:"version"`
		UptimeSeconds float64 `json:"uptime_seconds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status = %q; want %q", body.Status, "ok")
	}
	if body.Version == "" {
		t.Error("version should not be empty")
	}
	if body.UptimeSeconds <= 0 {
		t.Errorf("uptime_seconds = %f; want > 0", body.UptimeSeconds)
	}
}
