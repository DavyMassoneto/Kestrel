package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
)

func TestRequestID_GeneratesNew(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetRequestID(r.Context())
		if id == "" {
			t.Error("request ID should not be empty in context")
		}
		if !strings.HasPrefix(id, "req_") {
			t.Errorf("request ID = %q; want prefix %q", id, "req_")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("X-Request-ID header should be set in response")
	}
	if !strings.HasPrefix(respID, "req_") {
		t.Errorf("X-Request-ID = %q; want prefix %q", respID, "req_")
	}
}

func TestRequestID_ReusesExisting(t *testing.T) {
	existing := "external-trace-id-123"

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetRequestID(r.Context())
		if id != existing {
			t.Errorf("request ID = %q; want %q", id, existing)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existing)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get("X-Request-ID")
	if respID != existing {
		t.Errorf("X-Request-ID = %q; want %q", respID, existing)
	}
}

func TestRequestID_InResponse(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header must be present in response")
	}
}

func TestRequestID_AccessibleViaContext(t *testing.T) {
	var ctxID string

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = middleware.GetRequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ctxID == "" {
		t.Error("request ID should be accessible via context")
	}

	respID := rec.Header().Get("X-Request-ID")
	if ctxID != respID {
		t.Errorf("context ID = %q; response header ID = %q; should match", ctxID, respID)
	}
}
