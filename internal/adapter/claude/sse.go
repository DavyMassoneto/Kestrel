package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// sseEvent represents a raw SSE event parsed from the stream.
type sseEvent struct {
	Event string
	Data  string
}

// claudeSSEData is the common structure for Claude SSE event data.
type claudeSSEData struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"`
	Delta   json.RawMessage `json:"delta,omitempty"`
	Usage   *ClaudeUsage    `json:"usage,omitempty"`
}

// claudeDelta represents a delta object within a Claude SSE event.
type claudeDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	Thinking   string `json:"thinking,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// ReadSSE reads SSE events from a reader and emits domain StreamEvents on a channel.
// The channel is closed when the reader is exhausted or the context is cancelled.
func ReadSSE(ctx context.Context, r io.Reader) <-chan vo.StreamEvent {
	ch := make(chan vo.StreamEvent)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		var currentEvent sseEvent

		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()

			if line == "" {
				// Empty line = end of event
				if currentEvent.Data != "" {
					event, ok := translateSSEEvent(currentEvent)
					if ok {
						select {
						case <-ctx.Done():
							return
						case ch <- event:
						}
					}
				}
				currentEvent = sseEvent{}
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				currentEvent.Event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				currentEvent.Data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()

	return ch
}

// translateSSEEvent converts a raw SSE event into a domain StreamEvent.
// Returns false if the event should be ignored.
func translateSSEEvent(raw sseEvent) (vo.StreamEvent, bool) {
	var data claudeSSEData
	if err := json.Unmarshal([]byte(raw.Data), &data); err != nil {
		errMsg := fmt.Sprintf("failed to parse SSE data: %v", err)
		return vo.StreamEvent{
			Type:  vo.EventError,
			Error: &errMsg,
		}, true
	}

	switch data.Type {
	case "message_start":
		return vo.StreamEvent{Type: vo.EventStart}, true

	case "content_block_delta":
		var delta claudeDelta
		if err := json.Unmarshal(data.Delta, &delta); err != nil {
			errMsg := fmt.Sprintf("failed to parse delta: %v", err)
			return vo.StreamEvent{Type: vo.EventError, Error: &errMsg}, true
		}

		content := delta.Text
		if delta.Type == "thinking_delta" {
			content = delta.Thinking
		}

		return vo.StreamEvent{
			Type:    vo.EventDelta,
			Content: content,
		}, true

	case "message_delta":
		var usage *vo.Usage
		if data.Usage != nil {
			usage = &vo.Usage{
				InputTokens:  data.Usage.InputTokens,
				OutputTokens: data.Usage.OutputTokens,
			}
		}
		return vo.StreamEvent{
			Type:  vo.EventStop,
			Usage: usage,
		}, true

	case "content_block_start", "content_block_stop", "message_stop":
		return vo.StreamEvent{}, false

	default:
		return vo.StreamEvent{}, false
	}
}
