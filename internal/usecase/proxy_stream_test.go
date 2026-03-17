package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type mockChatStreamer struct {
	events <-chan vo.StreamEvent
	err    error
}

func (m *mockChatStreamer) StreamChat(_ context.Context, _ vo.ProviderCredentials, _ *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	return m.events, m.err
}

func makeEventChannel(events ...vo.StreamEvent) <-chan vo.StreamEvent {
	ch := make(chan vo.StreamEvent, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

func collectStreamEvents(ch <-chan vo.StreamEvent) []vo.StreamEvent {
	var events []vo.StreamEvent
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func TestProxyStreamUseCase_Success(t *testing.T) {
	expectedEvents := []vo.StreamEvent{
		{Type: vo.EventStart},
		{Type: vo.EventDelta, Content: "Hello"},
		{Type: vo.EventDelta, Content: " world"},
		{Type: vo.EventStop, Usage: &vo.Usage{InputTokens: 10, OutputTokens: 5}},
	}

	streamer := &mockChatStreamer{events: makeEventChannel(expectedEvents...)}
	account := testAccount(t)
	uc := NewProxyStreamUseCase(streamer, account)

	ch, err := uc.Execute(context.Background(), testChatRequest())

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	events := collectStreamEvents(ch)

	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4", len(events))
	}
	if events[0].Type != vo.EventStart {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, vo.EventStart)
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
	if events[3].Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", events[3].Usage.OutputTokens)
	}
}

func TestProxyStreamUseCase_Error(t *testing.T) {
	streamer := &mockChatStreamer{err: errors.New("connection refused")}
	account := testAccount(t)
	uc := NewProxyStreamUseCase(streamer, account)

	_, err := uc.Execute(context.Background(), testChatRequest())

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("error = %q, want %q", err.Error(), "connection refused")
	}
}

func TestProxyStreamUseCase_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	streamer := &mockChatStreamer{err: context.Canceled}
	account := testAccount(t)
	uc := NewProxyStreamUseCase(streamer, account)

	_, err := uc.Execute(ctx, testChatRequest())

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestProxyStreamUseCase_UsesAccountCredentials(t *testing.T) {
	var capturedCreds vo.ProviderCredentials

	innerStreamer := &mockChatStreamer{
		events: makeEventChannel(vo.StreamEvent{Type: vo.EventStop}),
	}
	captureStreamer := &capturingChatStreamer{
		inner:    innerStreamer,
		captured: &capturedCreds,
	}

	account := testAccount(t)
	uc := NewProxyStreamUseCase(captureStreamer, account)

	_, err := uc.Execute(context.Background(), testChatRequest())
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	expectedCreds := account.Credentials()
	if capturedCreds.APIKey.Value() != expectedCreds.APIKey.Value() {
		t.Errorf("APIKey = %q, want %q", capturedCreds.APIKey.Value(), expectedCreds.APIKey.Value())
	}
	if capturedCreds.BaseURL != expectedCreds.BaseURL {
		t.Errorf("BaseURL = %q, want %q", capturedCreds.BaseURL, expectedCreds.BaseURL)
	}
}

type capturingChatStreamer struct {
	inner    ChatStreamer
	captured *vo.ProviderCredentials
}

func (c *capturingChatStreamer) StreamChat(ctx context.Context, creds vo.ProviderCredentials, req *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	*c.captured = creds
	return c.inner.StreamChat(ctx, creds, req)
}

func TestProxyStreamUseCase_EmptyChannel(t *testing.T) {
	streamer := &mockChatStreamer{events: makeEventChannel()}
	account := testAccount(t)
	uc := NewProxyStreamUseCase(streamer, account)

	ch, err := uc.Execute(context.Background(), testChatRequest())

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	events := collectStreamEvents(ch)
	if len(events) != 0 {
		t.Errorf("events len = %d, want 0", len(events))
	}
}
