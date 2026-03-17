package usecase

import (
	"context"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// ChatSender sends synchronous requests to a provider.
type ChatSender interface {
	SendChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (*vo.ChatResponse, error)
}

// ProxyChatUseCase handles synchronous chat proxy requests.
// Phase 2: simplified single-account version without fallback or session.
type ProxyChatUseCase struct {
	chatSender ChatSender
	account    *entity.Account
}

// NewProxyChatUseCase creates a new ProxyChatUseCase.
func NewProxyChatUseCase(chatSender ChatSender, account *entity.Account) *ProxyChatUseCase {
	return &ProxyChatUseCase{
		chatSender: chatSender,
		account:    account,
	}
}

// Execute sends a chat request through the configured account.
func (uc *ProxyChatUseCase) Execute(ctx context.Context, chatReq *vo.ChatRequest) (*vo.ChatResponse, error) {
	creds := uc.account.Credentials()
	return uc.chatSender.SendChat(ctx, creds, chatReq)
}
