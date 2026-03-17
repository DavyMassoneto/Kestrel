package claude

import (
	"encoding/json"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestDomainToClaudeRequest_SimpleText(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
		MaxTokens: 4096,
	}

	result := DomainToClaudeRequest(req)

	if result.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-sonnet-4-20250514")
	}
	if result.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want %d", result.MaxTokens, 4096)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "user")
	}
	if len(result.Messages[0].Content) != 1 {
		t.Fatalf("Messages[0].Content len = %d, want 1", len(result.Messages[0].Content))
	}
	if result.Messages[0].Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %q, want %q", result.Messages[0].Content[0].Type, "text")
	}
	if result.Messages[0].Content[0].Text != "Hello" {
		t.Errorf("Content[0].Text = %q, want %q", result.Messages[0].Content[0].Text, "Hello")
	}
	if result.System != "" {
		t.Errorf("System = %q, want empty", result.System)
	}
	if result.Stream {
		t.Error("Stream should be false")
	}
}

func TestDomainToClaudeRequest_SystemMessage(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleSystem, Content: []vo.ContentBlock{{Type: "text", Text: "You are helpful."}}},
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if result.System != "You are helpful." {
		t.Errorf("System = %q, want %q", result.System, "You are helpful.")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1 (system extracted)", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "user")
	}
}

func TestDomainToClaudeRequest_MultipleSystemMessages(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleSystem, Content: []vo.ContentBlock{{Type: "text", Text: "Rule 1."}}},
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
			{Role: vo.RoleSystem, Content: []vo.ContentBlock{{Type: "text", Text: "Rule 2."}}},
			{Role: vo.RoleAssistant, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Bye"}}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if result.System != "Rule 1.\nRule 2." {
		t.Errorf("System = %q, want %q", result.System, "Rule 1.\nRule 2.")
	}
	if len(result.Messages) != 3 {
		t.Fatalf("Messages len = %d, want 3 (system messages removed)", len(result.Messages))
	}
}

func TestDomainToClaudeRequest_Temperature(t *testing.T) {
	temp := 0.7
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens:   1024,
		Temperature: &temp,
	}

	result := DomainToClaudeRequest(req)

	if result.Temperature == nil {
		t.Fatal("Temperature should not be nil")
	}
	if *result.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want %f", *result.Temperature, 0.7)
	}
}

func TestDomainToClaudeRequest_TemperatureNil(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if result.Temperature != nil {
		t.Error("Temperature should be nil when not provided")
	}
}

func TestDomainToClaudeRequest_DefaultMaxTokens(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens: 0,
	}

	result := DomainToClaudeRequest(req)

	if result.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192 (default)", result.MaxTokens)
	}
}

func TestDomainToClaudeRequest_ToolMessage(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleTool, Content: []vo.ContentBlock{
				{Type: "tool_result", Text: "25°C sunny"},
			}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if len(result.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("Tool role should be mapped to 'user', got %q", result.Messages[0].Role)
	}
}

func TestDomainToClaudeRequest_MultipleContentBlocks(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{
				{Type: "text", Text: "Look at this:"},
				{Type: "text", Text: "More text"},
			}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if len(result.Messages[0].Content) != 2 {
		t.Fatalf("Content blocks = %d, want 2", len(result.Messages[0].Content))
	}
}

func TestClaudeResponseToDomain_SimpleText(t *testing.T) {
	resp := ClaudeResponse{
		ID:    "msg_123",
		Model: "claude-sonnet-4-20250514",
		Content: []ClaudeContentBlock{
			{Type: "text", Text: "Hello there!"},
		},
		StopReason: "end_turn",
		Usage: ClaudeUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	result := ClaudeResponseToDomain(resp)

	if result.ID != "msg_123" {
		t.Errorf("ID = %q, want %q", result.ID, "msg_123")
	}
	if result.Content != "Hello there!" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello there!")
	}
	if result.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-sonnet-4-20250514")
	}
	if result.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", result.StopReason, "end_turn")
	}
	if result.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", result.Usage.OutputTokens)
	}
}

func TestClaudeResponseToDomain_MultipleTextBlocks(t *testing.T) {
	resp := ClaudeResponse{
		ID:    "msg_456",
		Model: "claude-sonnet-4-20250514",
		Content: []ClaudeContentBlock{
			{Type: "text", Text: "First part."},
			{Type: "text", Text: " Second part."},
		},
		StopReason: "end_turn",
		Usage:      ClaudeUsage{InputTokens: 10, OutputTokens: 20},
	}

	result := ClaudeResponseToDomain(resp)

	if result.Content != "First part. Second part." {
		t.Errorf("Content = %q, want %q", result.Content, "First part. Second part.")
	}
}

func TestClaudeResponseToDomain_ThinkingBlock(t *testing.T) {
	resp := ClaudeResponse{
		ID:    "msg_789",
		Model: "claude-sonnet-4-20250514",
		Content: []ClaudeContentBlock{
			{Type: "thinking", Thinking: "Let me analyze..."},
			{Type: "text", Text: "The answer is 42."},
		},
		StopReason: "end_turn",
		Usage:      ClaudeUsage{InputTokens: 50, OutputTokens: 100},
	}

	result := ClaudeResponseToDomain(resp)

	if result.Content != "The answer is 42." {
		t.Errorf("Content = %q, want %q", result.Content, "The answer is 42.")
	}
}

func TestClaudeResponseToDomain_EmptyContent(t *testing.T) {
	resp := ClaudeResponse{
		ID:         "msg_empty",
		Model:      "claude-sonnet-4-20250514",
		Content:    []ClaudeContentBlock{},
		StopReason: "end_turn",
		Usage:      ClaudeUsage{InputTokens: 10, OutputTokens: 0},
	}

	result := ClaudeResponseToDomain(resp)

	if result.Content != "" {
		t.Errorf("Content = %q, want empty", result.Content)
	}
}

func TestClaudeRequestJSON_Serialization(t *testing.T) {
	temp := 0.5
	req := ClaudeRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ClaudeMessage{
			{
				Role: "user",
				Content: []ClaudeContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		},
		MaxTokens:   4096,
		Temperature: &temp,
		System:      "Be helpful",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v", parsed["model"])
	}
	if parsed["max_tokens"].(float64) != 4096 {
		t.Errorf("max_tokens = %v", parsed["max_tokens"])
	}
	if parsed["system"] != "Be helpful" {
		t.Errorf("system = %v", parsed["system"])
	}
}

func TestClaudeRequestJSON_OmitsEmptyFields(t *testing.T) {
	req := ClaudeRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ClaudeMessage{
			{Role: "user", Content: []ClaudeContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens: 1024,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if _, ok := parsed["system"]; ok {
		t.Error("system should be omitted when empty")
	}
	if _, ok := parsed["temperature"]; ok {
		t.Error("temperature should be omitted when nil")
	}
	if _, ok := parsed["stream"]; ok {
		t.Error("stream should be omitted when false")
	}
}

func TestClaudeResponseJSON_Deserialization(t *testing.T) {
	raw := `{
		"id": "msg_test",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-20250514",
		"content": [
			{"type": "text", "text": "Hello!"}
		],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 25,
			"output_tokens": 10
		}
	}`

	var resp ClaudeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if resp.ID != "msg_test" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q", resp.Model)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("Content len = %d", len(resp.Content))
	}
	if resp.Content[0].Text != "Hello!" {
		t.Errorf("Content[0].Text = %q", resp.Content[0].Text)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q", resp.StopReason)
	}
	if resp.Usage.InputTokens != 25 {
		t.Errorf("InputTokens = %d", resp.Usage.InputTokens)
	}
}

func TestDomainToClaudeRequest_Stream(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeStreamRequest(req)

	if !result.Stream {
		t.Error("Stream should be true for streaming request")
	}
}

func TestDomainToClaudeRequest_AssistantRole(t *testing.T) {
	req := &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hi"}}},
			{Role: vo.RoleAssistant, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Bye"}}},
		},
		MaxTokens: 1024,
	}

	result := DomainToClaudeRequest(req)

	if len(result.Messages) != 3 {
		t.Fatalf("Messages len = %d, want 3", len(result.Messages))
	}
	if result.Messages[1].Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want %q", result.Messages[1].Role, "assistant")
	}
}
