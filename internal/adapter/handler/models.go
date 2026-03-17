package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type modelsResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsHandler returns an http.HandlerFunc for GET /v1/models.
func ModelsHandler() http.HandlerFunc {
	// Pre-compute the static list
	ids := vo.SupportedModelIDs()
	sort.Strings(ids)

	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	data := make([]modelEntry, len(ids))
	for i, id := range ids {
		data[i] = modelEntry{
			ID:      id,
			Object:  "model",
			Created: created,
			OwnedBy: "anthropic",
		}
	}

	resp := modelsResponse{
		Object: "list",
		Data:   data,
	}

	body, _ := json.Marshal(resp)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}
