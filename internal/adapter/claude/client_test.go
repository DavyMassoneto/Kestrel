package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func testCreds(baseURL string) vo.ProviderCredentials {
	return vo.ProviderCredentials{
		APIKey:  vo.NewSensitiveString("sk-ant-test-key"),
		BaseURL: baseURL,
	}
}

func testRequest() *vo.ChatRequest {
	return &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
		MaxTokens: 1024,
	}
}

func TestSendChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Path = %q, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant-test-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		var req ClaudeRequest
		json.Unmarshal(body, &req)
		if req.Stream {
			t.Error("Stream should be false for SendChat")
		}

		resp := ClaudeResponse{
			ID:    "msg_test",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []ClaudeContentBlock{
				{Type: "text", Text: "Hi there!"},
			},
			StopReason: "end_turn",
			Usage:      ClaudeUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	result, err := client.SendChat(context.Background(), testCreds(srv.URL), testRequest())

	if err != nil {
		t.Fatalf("SendChat error: %v", err)
	}
	if result.ID != "msg_test" {
		t.Errorf("ID = %q, want %q", result.ID, "msg_test")
	}
	if result.Content != "Hi there!" {
		t.Errorf("Content = %q, want %q", result.Content, "Hi there!")
	}
	if result.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.Usage.OutputTokens)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", result.StopReason, "end_turn")
	}
}

func TestSendChat_HTTPError429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	_, err := client.SendChat(context.Background(), testCreds(srv.URL), testRequest())

	if err == nil {
		t.Fatal("expected error")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected ProviderError")
	}
	if provErr.Classification() != vo.ErrRateLimit {
		t.Errorf("Classification = %q, want %q", provErr.Classification(), vo.ErrRateLimit)
	}
	if provErr.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", provErr.StatusCode)
	}
}

func TestSendChat_HTTPError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	_, err := client.SendChat(context.Background(), testCreds(srv.URL), testRequest())

	if err == nil {
		t.Fatal("expected error")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected ProviderError")
	}
	if provErr.Classification() != vo.ErrAuth {
		t.Errorf("Classification = %q, want %q", provErr.Classification(), vo.ErrAuth)
	}
}

func TestSendChat_InvalidResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid json}`)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	_, err := client.SendChat(context.Background(), testCreds(srv.URL), testRequest())

	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestStreamChat_Success(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"id":"msg_stream","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ClaudeRequest
		json.Unmarshal(body, &req)
		if !req.Stream {
			t.Error("Stream should be true for StreamChat")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	ch, err := client.StreamChat(context.Background(), testCreds(srv.URL), testRequest())

	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	events := collectEvents(ch)

	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4 (start, 2 deltas, stop)", len(events))
	}
	if events[0].Type != vo.EventStart {
		t.Errorf("events[0].Type = %q", events[0].Type)
	}
	if events[1].Content != "Hello" {
		t.Errorf("events[1].Content = %q", events[1].Content)
	}
	if events[2].Content != " world" {
		t.Errorf("events[2].Content = %q", events[2].Content)
	}
	if events[3].Type != vo.EventStop {
		t.Errorf("events[3].Type = %q", events[3].Type)
	}
}

func TestStreamChat_HTTPErrorBeforeStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	_, err := client.StreamChat(context.Background(), testCreds(srv.URL), testRequest())

	if err == nil {
		t.Fatal("expected error")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected ProviderError")
	}
	if provErr.Classification() != vo.ErrRateLimit {
		t.Errorf("Classification = %q, want %q", provErr.Classification(), vo.ErrRateLimit)
	}
}

func TestStreamChat_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)

		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-20250514\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n")
		flusher.Flush()

		// Cancel after first event
		cancel()

		// Server keeps writing but client should stop reading
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	ch, err := client.StreamChat(ctx, testCreds(srv.URL), testRequest())

	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	events := collectEvents(ch)
	// Should get at most the start event before cancellation takes effect
	if len(events) > 2 {
		t.Errorf("events len = %d, expected at most 2 with cancelled context", len(events))
	}
}

func TestSendChat_Headers(t *testing.T) {
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		resp := ClaudeResponse{
			ID: "msg_h", Model: "claude-sonnet-4-20250514",
			Content: []ClaudeContentBlock{{Type: "text", Text: "ok"}},
			StopReason: "end_turn",
			Usage:      ClaudeUsage{InputTokens: 1, OutputTokens: 1},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	client.SendChat(context.Background(), testCreds(srv.URL), testRequest())

	if capturedHeaders.Get("x-api-key") != "sk-ant-test-key" {
		t.Errorf("x-api-key = %q", capturedHeaders.Get("x-api-key"))
	}
	if capturedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version = %q", capturedHeaders.Get("anthropic-version"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", capturedHeaders.Get("Content-Type"))
	}
}
