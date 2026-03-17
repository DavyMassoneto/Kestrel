package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/sse"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const maxBodySize = 10 << 20 // 10MB

// ChatExecutor executes a synchronous chat request (Phase 2 interface).
type ChatExecutor interface {
	Execute(ctx context.Context, chatReq *vo.ChatRequest) (*vo.ChatResponse, error)
}

// StreamExecutor executes a streaming chat request (Phase 2 interface).
type StreamExecutor interface {
	Execute(ctx context.Context, chatReq *vo.ChatRequest) (<-chan vo.StreamEvent, error)
}

// ChatHandler handles POST /v1/chat/completions.
type ChatHandler struct {
	chatExecutor   ChatExecutor
	streamExecutor StreamExecutor
	sseWriter      sse.Writer
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(chat ChatExecutor, stream StreamExecutor) *ChatHandler {
	return &ChatHandler{
		chatExecutor:   chat,
		streamExecutor: stream,
	}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Limit body size
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "request body too large")
		return
	}

	// 2. Parse JSON
	var openaiReq OpenAIChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "invalid JSON")
		return
	}

	// 3. Validate required fields
	if openaiReq.Model == "" || len(openaiReq.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", "model and messages are required")
		return
	}

	// 4. Translate OpenAI -> domain
	chatReq, err := OpenAIToDomain(openaiReq)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad_request", err.Error())
		return
	}

	// 5. Check model access
	if apiKey := middleware.APIKeyFromContext(r.Context()); apiKey != nil {
		if !apiKey.IsModelAllowed(chatReq.Model.Raw) {
			writeError(w, http.StatusForbidden, "forbidden", "model_not_allowed", "you do not have access to this model")
			return
		}
	}

	// 6. Delegate to use case
	isStream := openaiReq.Stream != nil && *openaiReq.Stream
	if isStream {
		events, err := h.streamExecutor.Execute(r.Context(), chatReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
			return
		}
		h.sseWriter.Write(r.Context(), w, events, func(event vo.StreamEvent) []byte {
			return DomainEventToOpenAI(event, "chatcmpl-stream", chatReq.Model.Raw)
		})
	} else {
		resp, err := h.chatExecutor.Execute(r.Context(), chatReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "internal_error", err.Error())
			return
		}
		openaiResp := DomainToOpenAI(resp)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResp)
	}
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func writeError(w http.ResponseWriter, status int, errType, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorBody{
		Error: errorDetail{
			Message: message,
			Type:    errType,
			Code:    code,
		},
	})
}
