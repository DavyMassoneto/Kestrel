package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/sse"
	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const maxBodySize = 10 << 20 // 10MB

// ChatResult encapsulates a chat response for the handler.
type ChatResult struct {
	Response    *vo.ChatResponse
	AccountID   string
	AccountName string
	Retries     int
}

// ChatExecutor executes a synchronous chat request.
type ChatExecutor interface {
	Execute(ctx context.Context, apiKeyID vo.APIKeyID, chatReq *vo.ChatRequest) (ChatResult, error)
}

// StreamResult encapsulates a streaming response for the handler.
type StreamResult struct {
	Events      <-chan vo.StreamEvent
	AccountID   string
	AccountName string
	Retries     int
}

// StreamExecutor executes a streaming chat request.
type StreamExecutor interface {
	Execute(ctx context.Context, apiKeyID vo.APIKeyID, chatReq *vo.ChatRequest) (StreamResult, error)
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

	// 5. Extract API key from context
	apiKey := middleware.APIKeyFromContext(r.Context())
	if apiKey == nil {
		writeError(w, http.StatusUnauthorized, "authentication_error", "invalid_api_key", "API key required")
		return
	}

	// 6. Check model access
	if !apiKey.IsModelAllowed(chatReq.Model.Raw) {
		writeError(w, http.StatusForbidden, "forbidden", "model_not_allowed", "you do not have access to this model")
		return
	}

	apiKeyID := apiKey.ID()
	ld := middleware.LogDataFromContext(r.Context())

	// 7. Delegate to use case
	isStream := openaiReq.Stream != nil && *openaiReq.Stream

	// Populate model in LogData before calling use case
	if ld != nil {
		ld.Model = chatReq.Model.Raw
		ld.Stream = isStream
	}

	if isStream {
		result, err := h.streamExecutor.Execute(r.Context(), apiKeyID, chatReq)
		if err != nil {
			if ld != nil {
				ld.Error = err.Error()
			}
			status, code := mapUseCaseError(err)
			writeError(w, status, "server_error", code, err.Error())
			return
		}
		if ld != nil {
			ld.AccountID = result.AccountID
			ld.AccountName = result.AccountName
			ld.Retries = result.Retries
		}
		h.sseWriter.Write(r.Context(), w, result.Events, func(event vo.StreamEvent) []byte {
			if ld != nil && event.Usage != nil {
				ld.InputTokens = event.Usage.InputTokens
				ld.OutputTokens = event.Usage.OutputTokens
			}
			return DomainEventToOpenAI(event, "chatcmpl-stream", chatReq.Model.Raw)
		})
	} else {
		result, err := h.chatExecutor.Execute(r.Context(), apiKeyID, chatReq)
		if err != nil {
			if ld != nil {
				ld.Error = err.Error()
			}
			status, code := mapUseCaseError(err)
			writeError(w, status, "server_error", code, err.Error())
			return
		}
		if ld != nil {
			ld.AccountID = result.AccountID
			ld.AccountName = result.AccountName
			ld.Retries = result.Retries
			if result.Response != nil {
				ld.InputTokens = result.Response.Usage.InputTokens
				ld.OutputTokens = result.Response.Usage.OutputTokens
			}
		}
		openaiResp := DomainToOpenAI(result.Response)
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

func mapUseCaseError(err error) (int, string) {
	if errors.Is(err, errs.ErrAllAccountsExhausted) {
		return http.StatusServiceUnavailable, "all_accounts_exhausted"
	}
	return http.StatusInternalServerError, "internal_error"
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
