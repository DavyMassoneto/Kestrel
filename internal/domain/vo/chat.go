package vo

// ChatRequest is the domain representation — neither OpenAI nor Claude.
// The handler translates OpenAI -> ChatRequest. The Claude adapter translates ChatRequest -> Claude.
type ChatRequest struct {
	Model        ModelName
	Messages     []Message
	MaxTokens    int
	Temperature  *float64
	SystemPrompt string
}

// ChatResponse is the domain representation of a chat completion response.
type ChatResponse struct {
	ID         string
	Content    string
	Model      string
	Usage      Usage
	StopReason string
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	Type    StreamEventType
	Content string
	Usage   *Usage
	Error   *string
}

// StreamEventType enumerates domain-level stream event types.
type StreamEventType string

const (
	EventStart StreamEventType = "start"
	EventDelta StreamEventType = "delta"
	EventStop  StreamEventType = "stop"
	EventError StreamEventType = "error"
)

// Role represents the role of a message participant.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    Role
	Content []ContentBlock
}

// ContentBlock represents a block of content within a message.
type ContentBlock struct {
	Type       string // "text", "image", "tool_use", "tool_result"
	Text       string // for type "text"
	ToolCallID string // for type "tool_result"
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}
