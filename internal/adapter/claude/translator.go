package claude

import (
	"strings"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const defaultMaxTokens = 8192

// ClaudeRequest is the internal representation of a Claude API request.
type ClaudeRequest struct {
	Model       string              `json:"model"`
	Messages    []ClaudeMessage     `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	System      string              `json:"system,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

// ClaudeMessage represents a message in the Claude API format.
type ClaudeMessage struct {
	Role    string              `json:"role"`
	Content []ClaudeContentBlock `json:"content"`
}

// ClaudeContentBlock represents a content block in the Claude API format.
type ClaudeContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// ClaudeResponse is the internal representation of a Claude API response.
type ClaudeResponse struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Role       string              `json:"role"`
	Model      string              `json:"model"`
	Content    []ClaudeContentBlock `json:"content"`
	StopReason string              `json:"stop_reason"`
	Usage      ClaudeUsage         `json:"usage"`
}

// ClaudeUsage represents token usage in a Claude API response.
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// DomainToClaudeRequest converts a domain ChatRequest to a Claude API request.
func DomainToClaudeRequest(req *vo.ChatRequest) ClaudeRequest {
	return domainToClaudeRequest(req, false)
}

// DomainToClaudeStreamRequest converts a domain ChatRequest to a streaming Claude API request.
func DomainToClaudeStreamRequest(req *vo.ChatRequest) ClaudeRequest {
	return domainToClaudeRequest(req, true)
}

func domainToClaudeRequest(req *vo.ChatRequest, stream bool) ClaudeRequest {
	var systemParts []string
	var messages []ClaudeMessage

	for _, msg := range req.Messages {
		if msg.Role == vo.RoleSystem {
			for _, block := range msg.Content {
				if block.Type == "text" {
					systemParts = append(systemParts, block.Text)
				}
			}
			continue
		}

		role := string(msg.Role)
		if msg.Role == vo.RoleTool {
			role = "user"
		}

		var content []ClaudeContentBlock
		for _, block := range msg.Content {
			content = append(content, ClaudeContentBlock{
				Type: block.Type,
				Text: block.Text,
			})
		}

		messages = append(messages, ClaudeMessage{
			Role:    role,
			Content: content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	cr := ClaudeRequest{
		Model:       req.Model.Resolved,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      stream,
	}

	if len(systemParts) > 0 {
		cr.System = strings.Join(systemParts, "\n")
	}

	return cr
}

// ClaudeResponseToDomain converts a Claude API response to a domain ChatResponse.
func ClaudeResponseToDomain(resp ClaudeResponse) *vo.ChatResponse {
	var textParts []string

	for _, block := range resp.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}

	return &vo.ChatResponse{
		ID:         resp.ID,
		Content:    strings.Join(textParts, ""),
		Model:      resp.Model,
		StopReason: resp.StopReason,
		Usage: vo.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}
}
