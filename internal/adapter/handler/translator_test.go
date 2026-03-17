package handler_test

import (
	"encoding/json"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestOpenAIToDomain_SimpleText(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
		MaxTokens: intPtr(1024),
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.Model.Raw != "claude-sonnet-4-20250514" {
		t.Errorf("Model.Raw = %q; want %q", chat.Model.Raw, "claude-sonnet-4-20250514")
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("Messages len = %d; want 1", len(chat.Messages))
	}
	if chat.Messages[0].Role != vo.RoleUser {
		t.Errorf("Role = %q; want %q", chat.Messages[0].Role, vo.RoleUser)
	}
	if len(chat.Messages[0].Content) != 1 || chat.Messages[0].Content[0].Text != "Hello" {
		t.Errorf("Content = %v; want [{text Hello}]", chat.Messages[0].Content)
	}
	if chat.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d; want 1024", chat.MaxTokens)
	}
}

func TestOpenAIToDomain_SystemMessageExtracted(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful"`)},
			{Role: "system", Content: json.RawMessage(`"Be concise"`)},
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.SystemPrompt != "You are helpful\nBe concise" {
		t.Errorf("SystemPrompt = %q; want %q", chat.SystemPrompt, "You are helpful\nBe concise")
	}
	if len(chat.Messages) != 1 {
		t.Errorf("Messages len = %d; want 1 (system removed)", len(chat.Messages))
	}
}

func TestOpenAIToDomain_DefaultMaxTokens(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d; want 8192 (default)", chat.MaxTokens)
	}
}

func TestOpenAIToDomain_Temperature(t *testing.T) {
	temp := 0.5
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
		Temperature: &temp,
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.Temperature == nil || *chat.Temperature != 0.5 {
		t.Errorf("Temperature = %v; want 0.5", chat.Temperature)
	}
}

func TestOpenAIToDomain_ContentArray(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"Hello"},{"type":"text","text":"World"}]`)},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chat.Messages[0].Content) != 2 {
		t.Fatalf("Content blocks = %d; want 2", len(chat.Messages[0].Content))
	}
	if chat.Messages[0].Content[0].Text != "Hello" {
		t.Errorf("Content[0].Text = %q; want %q", chat.Messages[0].Content[0].Text, "Hello")
	}
	if chat.Messages[0].Content[1].Text != "World" {
		t.Errorf("Content[1].Text = %q; want %q", chat.Messages[0].Content[1].Text, "World")
	}
}

func TestOpenAIToDomain_ToolRole(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"ask"`)},
			{Role: "assistant", Content: json.RawMessage(`"response"`)},
			{Role: "tool", Content: json.RawMessage(`"result"`), ToolCallID: "call_123"},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// tool messages should be converted to user role with tool_result content
	if len(chat.Messages) != 3 {
		t.Fatalf("Messages len = %d; want 3", len(chat.Messages))
	}
	toolMsg := chat.Messages[2]
	if toolMsg.Role != vo.RoleUser {
		t.Errorf("tool message Role = %q; want %q", toolMsg.Role, vo.RoleUser)
	}
	if len(toolMsg.Content) != 1 {
		t.Fatalf("tool message Content blocks = %d; want 1", len(toolMsg.Content))
	}
	if toolMsg.Content[0].Type != "tool_result" {
		t.Errorf("Content[0].Type = %q; want %q", toolMsg.Content[0].Type, "tool_result")
	}
}

func TestOpenAIToDomain_InvalidModel(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	}

	_, err := handler.OpenAIToDomain(req)
	if err == nil {
		t.Fatal("expected error for invalid model")
	}
}

func TestDomainToOpenAI(t *testing.T) {
	resp := &vo.ChatResponse{
		ID:         "msg_123",
		Content:    "Hello!",
		Model:      "claude-sonnet-4-20250514",
		Usage:      vo.Usage{InputTokens: 10, OutputTokens: 5},
		StopReason: "end_turn",
	}

	result := handler.DomainToOpenAI(resp)

	if result.ID != "chatcmpl-msg_123" {
		t.Errorf("ID = %q; want %q", result.ID, "chatcmpl-msg_123")
	}
	if result.Object != "chat.completion" {
		t.Errorf("Object = %q; want %q", result.Object, "chat.completion")
	}
	if len(result.Choices) != 1 {
		t.Fatalf("Choices len = %d; want 1", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Content = %q; want %q", result.Choices[0].Message.Content, "Hello!")
	}
	if result.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q; want %q", result.Choices[0].FinishReason, "stop")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d; want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d; want 5", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d; want 15", result.Usage.TotalTokens)
	}
}

func TestDomainToOpenAI_StopReasonMapping(t *testing.T) {
	tests := []struct {
		claudeReason string
		openAIReason string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
		{"unknown", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.claudeReason, func(t *testing.T) {
			resp := &vo.ChatResponse{
				ID:         "msg_1",
				StopReason: tt.claudeReason,
			}
			result := handler.DomainToOpenAI(resp)
			if result.Choices[0].FinishReason != tt.openAIReason {
				t.Errorf("FinishReason = %q; want %q", result.Choices[0].FinishReason, tt.openAIReason)
			}
		})
	}
}

func TestDomainEventToOpenAI_Start(t *testing.T) {
	event := vo.StreamEvent{
		Type: vo.EventStart,
	}
	chunk := handler.DomainEventToOpenAI(event, "chatcmpl-123", "claude-sonnet-4-20250514")
	if len(chunk) == 0 {
		t.Fatal("chunk should not be empty")
	}

	var parsed handler.OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Choices[0].Delta.Role != "assistant" {
		t.Errorf("Delta.Role = %q; want %q", parsed.Choices[0].Delta.Role, "assistant")
	}
}

func TestDomainEventToOpenAI_Delta(t *testing.T) {
	event := vo.StreamEvent{
		Type:    vo.EventDelta,
		Content: "Hello",
	}
	chunk := handler.DomainEventToOpenAI(event, "chatcmpl-123", "claude-sonnet-4-20250514")

	var parsed handler.OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Choices[0].Delta.Content != "Hello" {
		t.Errorf("Delta.Content = %q; want %q", parsed.Choices[0].Delta.Content, "Hello")
	}
}

func TestDomainEventToOpenAI_Stop(t *testing.T) {
	usage := vo.Usage{InputTokens: 10, OutputTokens: 5}
	event := vo.StreamEvent{
		Type:  vo.EventStop,
		Usage: &usage,
	}
	chunk := handler.DomainEventToOpenAI(event, "chatcmpl-123", "claude-sonnet-4-20250514")

	var parsed handler.OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q; want %q", parsed.Choices[0].FinishReason, "stop")
	}
	if parsed.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if parsed.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d; want 15", parsed.Usage.TotalTokens)
	}
}

func intPtr(i int) *int { return &i }
