package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// --- OpenAI types ---

type OpenAIChatRequest struct {
	Model       string            `json:"model"`
	Messages    []OpenAIMessage   `json:"messages"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	Stream      *bool             `json:"stream,omitempty"`
	Tools       json.RawMessage   `json:"tools,omitempty"`
	ToolChoice  json.RawMessage   `json:"tool_choice,omitempty"`
	Thinking    json.RawMessage   `json:"thinking,omitempty"`
}

type OpenAIMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type OpenAIContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
}

type OpenAIChatResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []OpenAIChoice      `json:"choices"`
	Usage   OpenAIUsage         `json:"usage"`
}

type OpenAIChoice struct {
	Index        int            `json:"index"`
	Message      OpenAIRespMsg  `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

type OpenAIRespMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Stream types ---

type OpenAIStreamChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []OpenAIStreamChoice `json:"choices"`
	Usage   *OpenAIUsage        `json:"usage,omitempty"`
}

type OpenAIStreamChoice struct {
	Index        int             `json:"index"`
	Delta        OpenAIStreamDelta `json:"delta"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

type OpenAIStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// --- Translation functions ---

// OpenAIToDomain translates an OpenAI chat request to domain ChatRequest.
func OpenAIToDomain(req OpenAIChatRequest) (*vo.ChatRequest, error) {
	model, err := vo.ParseModelName(req.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid model: %w", err)
	}

	var systemParts []string
	var messages []vo.Message

	for _, msg := range req.Messages {
		content, err := parseContent(msg.Content)
		if err != nil {
			return nil, fmt.Errorf("invalid content in message: %w", err)
		}

		switch msg.Role {
		case "system":
			for _, block := range content {
				if block.Text != "" {
					systemParts = append(systemParts, block.Text)
				}
			}
		case "tool":
			messages = append(messages, vo.Message{
				Role: vo.RoleUser,
				Content: []vo.ContentBlock{
					{
						Type:       "tool_result",
						Text:       contentToText(content),
						ToolCallID: msg.ToolCallID,
					},
				},
			})
		default:
			role := mapRole(msg.Role)
			messages = append(messages, vo.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	maxTokens := 8192
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	return &vo.ChatRequest{
		Model:        model,
		Messages:     messages,
		MaxTokens:    maxTokens,
		Temperature:  req.Temperature,
		SystemPrompt: strings.Join(systemParts, "\n"),
	}, nil
}

// DomainToOpenAI translates a domain ChatResponse to OpenAI format.
func DomainToOpenAI(resp *vo.ChatResponse) OpenAIChatResponse {
	return OpenAIChatResponse{
		ID:      "chatcmpl-" + resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIRespMsg{
					Role:    "assistant",
					Content: resp.Content,
				},
				FinishReason: mapStopReason(resp.StopReason),
			},
		},
		Usage: OpenAIUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// DomainEventToOpenAI converts a domain StreamEvent to an OpenAI SSE chunk JSON.
func DomainEventToOpenAI(event vo.StreamEvent, id string, model string) []byte {
	chunk := OpenAIStreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
	}

	switch event.Type {
	case vo.EventStart:
		chunk.Choices = []OpenAIStreamChoice{
			{Index: 0, Delta: OpenAIStreamDelta{Role: "assistant"}},
		}
	case vo.EventDelta:
		chunk.Choices = []OpenAIStreamChoice{
			{Index: 0, Delta: OpenAIStreamDelta{Content: event.Content}},
		}
	case vo.EventStop:
		choice := OpenAIStreamChoice{
			Index:        0,
			FinishReason: "stop",
		}
		chunk.Choices = []OpenAIStreamChoice{choice}
		if event.Usage != nil {
			chunk.Usage = &OpenAIUsage{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}
	case vo.EventError:
		chunk.Choices = []OpenAIStreamChoice{
			{Index: 0, FinishReason: "stop"},
		}
	}

	data, _ := json.Marshal(chunk)
	return data
}

// --- helpers ---

func parseContent(raw json.RawMessage) ([]vo.ContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []vo.ContentBlock{{Type: "text", Text: s}}, nil
	}

	// Try array of content blocks
	var blocks []OpenAIContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, fmt.Errorf("content must be string or array: %w", err)
	}

	result := make([]vo.ContentBlock, len(blocks))
	for i, b := range blocks {
		result[i] = vo.ContentBlock{
			Type: b.Type,
			Text: b.Text,
		}
	}
	return result, nil
}

func contentToText(blocks []vo.ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func mapRole(role string) vo.Role {
	switch role {
	case "user":
		return vo.RoleUser
	case "assistant":
		return vo.RoleAssistant
	case "system":
		return vo.RoleSystem
	default:
		return vo.RoleUser
	}
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}
