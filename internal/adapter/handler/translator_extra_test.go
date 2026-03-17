package handler_test

import (
	"encoding/json"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestDomainEventToOpenAI_Error(t *testing.T) {
	errMsg := "something went wrong"
	event := vo.StreamEvent{
		Type:  vo.EventError,
		Error: &errMsg,
	}
	chunk := handler.DomainEventToOpenAI(event, "chatcmpl-err", "claude-sonnet-4-20250514")

	var parsed handler.OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q; want %q", parsed.Choices[0].FinishReason, "stop")
	}
}

func TestDomainEventToOpenAI_StopWithoutUsage(t *testing.T) {
	event := vo.StreamEvent{
		Type: vo.EventStop,
	}
	chunk := handler.DomainEventToOpenAI(event, "chatcmpl-no-usage", "claude-sonnet-4-20250514")

	var parsed handler.OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Usage != nil {
		t.Error("Usage should be nil when event has no usage")
	}
	if parsed.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q; want %q", parsed.Choices[0].FinishReason, "stop")
	}
}

func TestOpenAIToDomain_EmptyContent(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: nil},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chat.Messages[0].Content) != 0 {
		t.Errorf("expected empty content, got %v", chat.Messages[0].Content)
	}
}

func TestOpenAIToDomain_InvalidContent(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`{invalid}`)},
		},
	}

	_, err := handler.OpenAIToDomain(req)
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestOpenAIToDomain_AssistantRole(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
			{Role: "assistant", Content: json.RawMessage(`"Hello!"`)},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.Messages[1].Role != vo.RoleAssistant {
		t.Errorf("Role = %q; want %q", chat.Messages[1].Role, vo.RoleAssistant)
	}
}

func TestOpenAIToDomain_UnknownRoleDefaultsToUser(t *testing.T) {
	req := handler.OpenAIChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []handler.OpenAIMessage{
			{Role: "custom_role", Content: json.RawMessage(`"Hi"`)},
		},
	}

	chat, err := handler.OpenAIToDomain(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.Messages[0].Role != vo.RoleUser {
		t.Errorf("unknown role should default to user, got %q", chat.Messages[0].Role)
	}
}
