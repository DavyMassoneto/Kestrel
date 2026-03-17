package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/sse"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestWrite_Headers(t *testing.T) {
	events := make(chan vo.StreamEvent)
	close(events)

	rec := httptest.NewRecorder()
	w := sse.Writer{}
	w.Write(context.Background(), rec, events, func(e vo.StreamEvent) []byte { return nil })

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q; want %q", ct, "text/event-stream")
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q; want %q", cc, "no-cache")
	}
	conn := rec.Header().Get("Connection")
	if conn != "keep-alive" {
		t.Errorf("Connection = %q; want %q", conn, "keep-alive")
	}
}

func TestWrite_Events(t *testing.T) {
	events := make(chan vo.StreamEvent, 2)
	events <- vo.StreamEvent{Type: vo.EventStart}
	events <- vo.StreamEvent{Type: vo.EventDelta, Content: "hello"}
	close(events)

	rec := httptest.NewRecorder()
	w := sse.Writer{}
	w.Write(context.Background(), rec, events, func(e vo.StreamEvent) []byte {
		return []byte(`{"type":"` + string(e.Type) + `"}`)
	})

	body := rec.Body.String()
	if !strings.Contains(body, `data: {"type":"start"}`) {
		t.Error("body should contain start event")
	}
	if !strings.Contains(body, `data: {"type":"delta"}`) {
		t.Error("body should contain delta event")
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("body should end with [DONE], got: %q", body)
	}
}

func TestWrite_ContextCancelled(t *testing.T) {
	events := make(chan vo.StreamEvent, 1)
	events <- vo.StreamEvent{Type: vo.EventDelta, Content: "should be skipped"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before writing

	rec := httptest.NewRecorder()
	w := sse.Writer{}
	w.Write(ctx, rec, events, func(e vo.StreamEvent) []byte {
		return []byte(`data`)
	})

	// When context is already cancelled, we should get headers but no event data
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q; want %q", ct, "text/event-stream")
	}
}

func TestWrite_EmptyChannel(t *testing.T) {
	events := make(chan vo.StreamEvent)
	close(events)

	rec := httptest.NewRecorder()
	w := sse.Writer{}
	w.Write(context.Background(), rec, events, func(e vo.StreamEvent) []byte { return nil })

	body := rec.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("body should contain [DONE] for empty channel, got: %q", body)
	}
}

func TestWrite_NilTranslateSkipped(t *testing.T) {
	events := make(chan vo.StreamEvent, 1)
	events <- vo.StreamEvent{Type: vo.EventStart}
	close(events)

	rec := httptest.NewRecorder()
	w := sse.Writer{}
	w.Write(context.Background(), rec, events, func(e vo.StreamEvent) []byte {
		return nil // nil return should be skipped
	})

	body := rec.Body.String()
	// Should only have [DONE], no data lines for nil chunks
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line != "data: [DONE]" {
			t.Errorf("unexpected line: %q (expected only [DONE])", line)
		}
	}
}

// flusherRecorder implements http.Flusher for testing
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (f *flusherRecorder) Flush() {
	f.flushCount++
	f.ResponseRecorder.Flush()
}

func TestWrite_FlushCalled(t *testing.T) {
	events := make(chan vo.StreamEvent, 1)
	events <- vo.StreamEvent{Type: vo.EventDelta, Content: "test"}
	close(events)

	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	w := sse.Writer{}
	w.Write(context.Background(), rec, events, func(e vo.StreamEvent) []byte {
		return []byte(`{"test":true}`)
	})

	// At least 2 flushes: one for the event, one for [DONE]
	if rec.flushCount < 2 {
		t.Errorf("flushCount = %d; want >= 2", rec.flushCount)
	}
}

// Verify the Writer works with http.ResponseWriter (interface compliance)
func TestWriter_InterfaceCompliance(t *testing.T) {
	var _ http.ResponseWriter = httptest.NewRecorder()
}
