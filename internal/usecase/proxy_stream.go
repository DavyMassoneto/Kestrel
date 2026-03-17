package usecase

import (
	"context"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// ChatStreamer sends streaming requests to a provider.
type ChatStreamer interface {
	StreamChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (<-chan vo.StreamEvent, error)
}

// ProxyStreamUseCase handles streaming chat proxy requests.
// Phase 2: simplified single-account version without fallback or session.
type ProxyStreamUseCase struct {
	chatStreamer ChatStreamer
	account     *entity.Account
}

// NewProxyStreamUseCase creates a new ProxyStreamUseCase.
func NewProxyStreamUseCase(chatStreamer ChatStreamer, account *entity.Account) *ProxyStreamUseCase {
	return &ProxyStreamUseCase{
		chatStreamer: chatStreamer,
		account:     account,
	}
}

// Execute sends a streaming chat request through the configured account.
func (uc *ProxyStreamUseCase) Execute(ctx context.Context, chatReq *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	creds := uc.account.Credentials()
	return uc.chatStreamer.StreamChat(ctx, creds, chatReq)
}
