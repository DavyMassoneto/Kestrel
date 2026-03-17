package usecase

import (
	"context"
	"errors"

	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const maxRetries = 10

// ChatSender sends synchronous requests to a provider.
type ChatSender interface {
	SendChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (*vo.ChatResponse, error)
}

// ProxyChatUseCase handles synchronous chat proxy requests with retry and fallback.
type ProxyChatUseCase struct {
	chatSender      ChatSender
	accountSelector AccountSelector
	fallbackHandler FallbackHandler
	sessionReader   SessionReader
	sessionWriter   SessionWriter
	accountWriter   AccountStatusWriter
	clock           Clock
}

// NewProxyChatUseCase creates a new ProxyChatUseCase.
func NewProxyChatUseCase(
	chatSender ChatSender,
	accountSelector AccountSelector,
	fallbackHandler FallbackHandler,
	sessionReader SessionReader,
	sessionWriter SessionWriter,
	accountWriter AccountStatusWriter,
	clock Clock,
) *ProxyChatUseCase {
	return &ProxyChatUseCase{
		chatSender:      chatSender,
		accountSelector: accountSelector,
		fallbackHandler: fallbackHandler,
		sessionReader:   sessionReader,
		sessionWriter:   sessionWriter,
		accountWriter:   accountWriter,
		clock:           clock,
	}
}

// Execute sends a chat request with retry loop, fallback, and session management.
func (uc *ProxyChatUseCase) Execute(ctx context.Context, apiKeyID vo.APIKeyID, chatReq *vo.ChatRequest) (ProxyChatResult, error) {
	session, err := uc.sessionReader.GetOrCreate(ctx, apiKeyID, chatReq.Model)
	if err != nil {
		return ProxyChatResult{}, err
	}

	var retries []RetryAttempt
	var excludeID *vo.AccountID

	for range maxRetries {
		account, err := uc.accountSelector.Execute(ctx, session.AccountID(), excludeID, uc.clock.Now())
		if err != nil {
			return ProxyChatResult{Retries: retries}, errs.ErrAllAccountsExhausted
		}

		chatResp, err := uc.chatSender.SendChat(ctx, account.Credentials(), chatReq)
		if err == nil {
			// Success path
			account.ClearError()
			account.RecordUsage(uc.clock.Now())
			uc.accountWriter.UpdateStatus(ctx, account)
			session.BindAccount(account.ID())
			session.RecordRequest(uc.clock.Now())
			uc.sessionWriter.Save(ctx, session)
			return ProxyChatResult{Response: chatResp, Retries: retries}, nil
		}

		// Error path: check if classified
		var classErr ClassifiedError
		if !errors.As(err, &classErr) {
			return ProxyChatResult{Retries: retries}, err
		}

		result, fbErr := uc.fallbackHandler.Execute(ctx, account, classErr.Classification())
		if fbErr != nil {
			return ProxyChatResult{Retries: retries}, fbErr
		}

		retries = append(retries, RetryAttempt{
			AccountID:      account.ID(),
			Classification: classErr.Classification(),
			RetryIndex:     len(retries),
		})

		if !result.ShouldFallback {
			return ProxyChatResult{Retries: retries}, err
		}

		accID := account.ID()
		excludeID = &accID
	}

	return ProxyChatResult{Retries: retries}, errs.ErrAllAccountsExhausted
}
