package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

type mockChatSender struct {
	response *vo.ChatResponse
	err      error
}

func (m *mockChatSender) SendChat(_ context.Context, _ vo.ProviderCredentials, _ *vo.ChatRequest) (*vo.ChatResponse, error) {
	return m.response, m.err
}

func testAccount(t *testing.T) *entity.Account {
	t.Helper()
	acc, err := entity.NewAccount(
		vo.NewAccountID(),
		"test-account",
		vo.NewSensitiveString("sk-ant-test"),
		"https://api.anthropic.com",
		1,
	)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return acc
}

func testChatRequest() *vo.ChatRequest {
	return &vo.ChatRequest{
		Model: vo.ModelName{Raw: "claude-sonnet-4-20250514", Resolved: "claude-sonnet-4-20250514"},
		Messages: []vo.Message{
			{Role: vo.RoleUser, Content: []vo.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
		MaxTokens: 1024,
	}
}

func TestProxyChatUseCase_Success(t *testing.T) {
	expectedResp := &vo.ChatResponse{
		ID:         "msg_123",
		Content:    "Hi there!",
		Model:      "claude-sonnet-4-20250514",
		StopReason: "end_turn",
		Usage:      vo.Usage{InputTokens: 10, OutputTokens: 5},
	}

	sender := &mockChatSender{response: expectedResp}
	account := testAccount(t)
	uc := NewProxyChatUseCase(sender, account)

	result, err := uc.Execute(context.Background(), testChatRequest())

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.ID != "msg_123" {
		t.Errorf("ID = %q, want %q", result.ID, "msg_123")
	}
	if result.Content != "Hi there!" {
		t.Errorf("Content = %q, want %q", result.Content, "Hi there!")
	}
	if result.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.Usage.OutputTokens)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", result.StopReason, "end_turn")
	}
}

func TestProxyChatUseCase_Error(t *testing.T) {
	sender := &mockChatSender{err: errors.New("connection refused")}
	account := testAccount(t)
	uc := NewProxyChatUseCase(sender, account)

	_, err := uc.Execute(context.Background(), testChatRequest())

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("error = %q, want %q", err.Error(), "connection refused")
	}
}

func TestProxyChatUseCase_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sender := &mockChatSender{err: context.Canceled}
	account := testAccount(t)
	uc := NewProxyChatUseCase(sender, account)

	_, err := uc.Execute(ctx, testChatRequest())

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestProxyChatUseCase_UsesAccountCredentials(t *testing.T) {
	var capturedCreds vo.ProviderCredentials

	sender := &mockChatSender{
		response: &vo.ChatResponse{ID: "msg_1", Content: "ok"},
	}

	// Use a custom sender that captures credentials
	captureSender := &capturingChatSender{
		inner:    sender,
		captured: &capturedCreds,
	}

	account := testAccount(t)
	uc := NewProxyChatUseCase(captureSender, account)

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

type capturingChatSender struct {
	inner    ChatSender
	captured *vo.ProviderCredentials
}

func (c *capturingChatSender) SendChat(ctx context.Context, creds vo.ProviderCredentials, req *vo.ChatRequest) (*vo.ChatResponse, error) {
	*c.captured = creds
	return c.inner.SendChat(ctx, creds, req)
}
