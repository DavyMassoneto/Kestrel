package vo_test

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestChatRequest_Fields(t *testing.T) {
	temp := 0.7
	model, _ := vo.ParseModelName("claude-sonnet-4-20250514")
	req := vo.ChatRequest{
		Model: model,
		Messages: []vo.Message{
			{
				Role: vo.RoleUser,
				Content: []vo.ContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		},
		MaxTokens:   1024,
		Temperature: &temp,
	}

	if req.Model.Raw != "claude-sonnet-4-20250514" {
		t.Errorf("Model.Raw = %q; want %q", req.Model.Raw, "claude-sonnet-4-20250514")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("Messages len = %d; want 1", len(req.Messages))
	}
	if req.Messages[0].Role != vo.RoleUser {
		t.Errorf("Role = %q; want %q", req.Messages[0].Role, vo.RoleUser)
	}
	if req.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d; want 1024", req.MaxTokens)
	}
	if *req.Temperature != 0.7 {
		t.Errorf("Temperature = %f; want 0.7", *req.Temperature)
	}
}

func TestChatResponse_Fields(t *testing.T) {
	resp := vo.ChatResponse{
		ID:         "msg_123",
		Content:    "Hello, world!",
		Model:      "claude-sonnet-4-20250514",
		Usage:      vo.Usage{InputTokens: 10, OutputTokens: 20},
		StopReason: "end_turn",
	}

	if resp.ID != "msg_123" {
		t.Errorf("ID = %q; want %q", resp.ID, "msg_123")
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("Content = %q; want %q", resp.Content, "Hello, world!")
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d; want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d; want 20", resp.Usage.OutputTokens)
	}
}

func TestStreamEvent_Fields(t *testing.T) {
	errMsg := "something went wrong"
	usage := vo.Usage{InputTokens: 5, OutputTokens: 15}
	event := vo.StreamEvent{
		Type:    vo.EventDelta,
		Content: "partial content",
		Usage:   &usage,
		Error:   &errMsg,
	}

	if event.Type != vo.EventDelta {
		t.Errorf("Type = %q; want %q", event.Type, vo.EventDelta)
	}
	if event.Content != "partial content" {
		t.Errorf("Content = %q; want %q", event.Content, "partial content")
	}
	if event.Usage.InputTokens != 5 {
		t.Errorf("Usage.InputTokens = %d; want 5", event.Usage.InputTokens)
	}
	if *event.Error != errMsg {
		t.Errorf("Error = %q; want %q", *event.Error, errMsg)
	}
}

func TestStreamEventTypes(t *testing.T) {
	if vo.EventStart != "start" {
		t.Errorf("EventStart = %q; want %q", vo.EventStart, "start")
	}
	if vo.EventDelta != "delta" {
		t.Errorf("EventDelta = %q; want %q", vo.EventDelta, "delta")
	}
	if vo.EventStop != "stop" {
		t.Errorf("EventStop = %q; want %q", vo.EventStop, "stop")
	}
	if vo.EventError != "error" {
		t.Errorf("EventError = %q; want %q", vo.EventError, "error")
	}
}

func TestRoles(t *testing.T) {
	if vo.RoleUser != "user" {
		t.Errorf("RoleUser = %q; want %q", vo.RoleUser, "user")
	}
	if vo.RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q; want %q", vo.RoleAssistant, "assistant")
	}
	if vo.RoleSystem != "system" {
		t.Errorf("RoleSystem = %q; want %q", vo.RoleSystem, "system")
	}
	if vo.RoleTool != "tool" {
		t.Errorf("RoleTool = %q; want %q", vo.RoleTool, "tool")
	}
}
