package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// Version is set via ldflags at build time.
var Version = "dev"

type healthResponse struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// Health implements http.Handler for the health check endpoint.
type Health struct {
	startTime time.Time
}

// NewHealth creates a Health handler with the given start time.
func NewHealth(startTime time.Time) *Health {
	return &Health{startTime: startTime}
}

func (h *Health) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := healthResponse{
		Status:        "ok",
		Version:       Version,
		UptimeSeconds: time.Since(h.startTime).Seconds(),
	}

	json.NewEncoder(w).Encode(resp)
}
