package usecase

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

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
