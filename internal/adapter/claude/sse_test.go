package claude

import (
	"context"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func collectEvents(ch <-chan vo.StreamEvent) []vo.StreamEvent {
	var events []vo.StreamEvent
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func TestReadSSE_MessageStart(t *testing.T) {
	data := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"stop_reason":null,"usage":{"input_tokens":25,"output_tokens":0}}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != vo.EventStart {
		t.Errorf("Type = %q, want %q", events[0].Type, vo.EventStart)
	}
}

func TestReadSSE_TextDelta(t *testing.T) {
	data := `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != vo.EventDelta {
		t.Errorf("Type = %q, want %q", events[0].Type, vo.EventDelta)
	}
	if events[0].Content != "Hello" {
		t.Errorf("Content = %q, want %q", events[0].Content, "Hello")
	}
}

func TestReadSSE_ThinkingDelta(t *testing.T) {
	data := `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think..."}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != vo.EventDelta {
		t.Errorf("Type = %q, want %q", events[0].Type, vo.EventDelta)
	}
	if events[0].Content != "Let me think..." {
		t.Errorf("Content = %q, want %q", events[0].Content, "Let me think...")
	}
}

func TestReadSSE_MessageDelta_StopReason(t *testing.T) {
	data := `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":50}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != vo.EventStop {
		t.Errorf("Type = %q, want %q", events[0].Type, vo.EventStop)
	}
	if events[0].Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if events[0].Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", events[0].Usage.OutputTokens)
	}
}

func TestReadSSE_ContentBlockStop_Ignored(t *testing.T) {
	data := `event: content_block_stop
data: {"type":"content_block_stop","index":0}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 0 {
		t.Errorf("events len = %d, want 0 (content_block_stop ignored)", len(events))
	}
}

func TestReadSSE_MessageStop_Ignored(t *testing.T) {
	data := `event: message_stop
data: {"type":"message_stop"}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 0 {
		t.Errorf("events len = %d, want 0 (message_stop ignored)", len(events))
	}
}

func TestReadSSE_ContentBlockStart_Ignored(t *testing.T) {
	data := `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 0 {
		t.Errorf("events len = %d, want 0 (text content_block_start ignored)", len(events))
	}
}

func TestReadSSE_FullConversation(t *testing.T) {
	data := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"stop_reason":null,"usage":{"input_tokens":25,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}

`
	r := strings.NewReader(data)
	ch := ReadSSE(context.Background(), r)
	events := collectEvents(ch)

	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4 (start, 2 deltas, stop)", len(events))
	}
	if events[0].Type != vo.EventStart {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, vo.EventStart)
	}
	if events[1].Type != vo.EventDelta {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, vo.EventDelta)
	}
	if events[1].Content != "Hello" {
		t.Errorf("events[1].Content = %q, want %q", events[1].Content, "Hello")
	}
	if events[2].Content != " world" {
		t.Errorf("events[2].Content = %q, want %q", events[2].Content, " world")
	}
	if events[3].Type != vo.EventStop {
		t.Errorf("events[3].Type = %q, want %q", events[3].Type, vo.EventStop)
	}
}

func TestReadSSE_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data := `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`
	r := strings.NewReader(data)
	ch := ReadSSE(ctx, r)
	events := collectEvents(ch)

	// With cancelled context, we should get 0 or fewer events
	if len(events) > 1 {
		t.Errorf("events len = %d, expected at most 1 with cancelled context", len(events))
	}
}

func TestReadSSE_InvalidJSON(t *testing.T) {
	data := `event: content_block_delta
data: {invalid json}

`
	r := strings.NewReader(data)
	ch := ReadSSE(ctx(), r)
	events := collectEvents(ch)

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1 (error event)", len(events))
	}
	if events[0].Type != vo.EventError {
		t.Errorf("Type = %q, want %q", events[0].Type, vo.EventError)
	}
	if events[0].Error == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestReadSSE_EmptyData(t *testing.T) {
	data := `event: ping
data: {}

`
	r := strings.NewReader(data)
	ch := ReadSSE(ctx(), r)
	events := collectEvents(ch)

	// ping events with empty/unknown type should be ignored
	if len(events) != 0 {
		t.Errorf("events len = %d, want 0 (ping ignored)", len(events))
	}
}

func ctx() context.Context {
	return context.Background()
}
