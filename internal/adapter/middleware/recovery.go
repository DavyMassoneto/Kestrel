package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Recovery is a middleware that recovers from panics, logs the error with
// stack trace and request ID (if available), and returns a 500 JSON response.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()

				attrs := []any{
					slog.String("stack", string(stack)),
					slog.String("panic", fmt.Sprintf("%v", rec)),
				}
				if reqID := GetRequestID(r.Context()); reqID != "" {
					attrs = append(attrs, slog.String("request_id", reqID))
				}

				slog.Error("panic recovered", attrs...)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				json.NewEncoder(w).Encode(errorResponse{
					Error: errorDetail{
						Message: "Internal server error",
						Type:    "server_error",
						Code:    "internal_error",
					},
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
