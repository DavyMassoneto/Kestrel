package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
)

func TestModelsHandler_Returns200(t *testing.T) {
	h := handler.ModelsHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

func TestModelsHandler_ResponseStructure(t *testing.T) {
	h := handler.ModelsHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("object = %q; want %q", resp.Object, "list")
	}

	if len(resp.Data) == 0 {
		t.Fatal("data should not be empty")
	}

	// Verify all entries have required fields
	for _, model := range resp.Data {
		if model.ID == "" {
			t.Error("model id should not be empty")
		}
		if model.Object != "model" {
			t.Errorf("model object = %q; want %q", model.Object, "model")
		}
		if model.Created == 0 {
			t.Error("model created should not be zero")
		}
		if model.OwnedBy != "anthropic" {
			t.Errorf("owned_by = %q; want %q", model.OwnedBy, "anthropic")
		}
	}
}

func TestModelsHandler_ContainsExpectedModels(t *testing.T) {
	h := handler.ModelsHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	ids := make(map[string]bool)
	for _, m := range resp.Data {
		ids[m.ID] = true
	}

	expected := []string{
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"claude-3-5-sonnet-20241022",
	}
	for _, e := range expected {
		if !ids[e] {
			t.Errorf("expected model %q not found in response", e)
		}
	}
}

func TestModelsHandler_DataIsSorted(t *testing.T) {
	h := handler.ModelsHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i].ID < resp.Data[i-1].ID {
			t.Errorf("models not sorted: %q comes after %q", resp.Data[i].ID, resp.Data[i-1].ID)
		}
	}
}
