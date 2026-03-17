package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// --- mock use cases ---

type mockChatSender struct {
	resp *vo.ChatResponse
	err  error
}

func (m *mockChatSender) Execute(_ context.Context, _ *vo.ChatRequest) (*vo.ChatResponse, error) {
	return m.resp, m.err
}

type mockStreamSender struct {
	events <-chan vo.StreamEvent
	err    error
}

func (m *mockStreamSender) Execute(_ context.Context, _ *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	return m.events, m.err
}

// --- tests ---

func TestChatHandler_NonStreaming_Success(t *testing.T) {
	chat := &mockChatSender{
		resp: &vo.ChatResponse{
			ID:         "msg_123",
			Content:    "Hello!",
			Model:      "claude-sonnet-4-20250514",
			Usage:      vo.Usage{InputTokens: 10, OutputTokens: 5},
			StopReason: "end_turn",
		},
	}
	h := handler.NewChatHandler(chat, nil)

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}

	var resp handler.OpenAIChatResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.ID != "chatcmpl-msg_123" {
		t.Errorf("ID = %q; want %q", resp.ID, "chatcmpl-msg_123")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices len = %d; want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Content = %q; want %q", resp.Choices[0].Message.Content, "Hello!")
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q; want %q", resp.Choices[0].FinishReason, "stop")
	}
}

func TestChatHandler_Streaming_Success(t *testing.T) {
	events := make(chan vo.StreamEvent, 3)
	events <- vo.StreamEvent{Type: vo.EventStart}
	events <- vo.StreamEvent{Type: vo.EventDelta, Content: "Hi"}
	events <- vo.StreamEvent{Type: vo.EventStop}
	close(events)

	stream := &mockStreamSender{events: events}
	h := handler.NewChatHandler(nil, stream)

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q; want %q", ct, "text/event-stream")
	}

	respBody := rec.Body.String()
	if !strings.Contains(respBody, "data: ") {
		t.Error("response should contain SSE data lines")
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Error("response should contain [DONE]")
	}
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	h := handler.NewChatHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	assertErrorResponse(t, rec, "invalid_request_error", "bad_request")
}

func TestChatHandler_MissingModel(t *testing.T) {
	h := handler.NewChatHandler(nil, nil)

	body := `{"messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	assertErrorResponse(t, rec, "invalid_request_error", "bad_request")
}

func TestChatHandler_MissingMessages(t *testing.T) {
	h := handler.NewChatHandler(nil, nil)

	body := `{"model":"claude-sonnet-4-20250514"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	assertErrorResponse(t, rec, "invalid_request_error", "bad_request")
}

func TestChatHandler_InvalidModel(t *testing.T) {
	h := handler.NewChatHandler(nil, nil)

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	assertErrorResponse(t, rec, "invalid_request_error", "bad_request")
}

func TestChatHandler_BodyTooLarge(t *testing.T) {
	h := handler.NewChatHandler(nil, nil)

	// Create a body larger than 10MB
	large := strings.Repeat("x", 11<<20)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(large))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}

	assertErrorResponse(t, rec, "invalid_request_error", "request_too_large")
}

func TestChatHandler_UseCaseError(t *testing.T) {
	chat := &mockChatSender{
		err: errors.New("provider error"),
	}
	h := handler.NewChatHandler(chat, nil)

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	assertErrorResponse(t, rec, "server_error", "internal_error")
}

func TestChatHandler_StreamUseCaseError(t *testing.T) {
	stream := &mockStreamSender{
		err: errors.New("stream error"),
	}
	h := handler.NewChatHandler(nil, stream)

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}

	assertErrorResponse(t, rec, "server_error", "internal_error")
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantType, wantCode string) {
	t.Helper()

	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Type != wantType {
		t.Errorf("error.type = %q; want %q", body.Error.Type, wantType)
	}
	if body.Error.Code != wantCode {
		t.Errorf("error.code = %q; want %q", body.Error.Code, wantCode)
	}
	if body.Error.Message == "" {
		t.Error("error.message should not be empty")
	}
}
