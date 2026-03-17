package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const maxStreamRetries = 10

// ChatStreamer sends streaming requests to a provider.
type ChatStreamer interface {
	StreamChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (<-chan vo.StreamEvent, error)
}

// ProxyStreamUseCase handles streaming chat proxy requests with retry/fallback.
type ProxyStreamUseCase struct {
	chatStreamer     ChatStreamer
	accountSelector AccountSelector
	fallbackHandler FallbackHandler
	accountWriter   AccountStatusWriter
	sessionReader   SessionReader
	sessionWriter   SessionWriter
	clock           Clock
}

// NewProxyStreamUseCase creates a new ProxyStreamUseCase.
func NewProxyStreamUseCase(
	chatStreamer ChatStreamer,
	accountSelector AccountSelector,
	fallbackHandler FallbackHandler,
	accountWriter AccountStatusWriter,
	sessionReader SessionReader,
	sessionWriter SessionWriter,
	clock Clock,
) *ProxyStreamUseCase {
	return &ProxyStreamUseCase{
		chatStreamer:     chatStreamer,
		accountSelector: accountSelector,
		fallbackHandler: fallbackHandler,
		accountWriter:   accountWriter,
		sessionReader:   sessionReader,
		sessionWriter:   sessionWriter,
		clock:           clock,
	}
}

// Execute sends a streaming chat request with retry loop and fallback.
func (uc *ProxyStreamUseCase) Execute(ctx context.Context, apiKeyID vo.APIKeyID, chatReq *vo.ChatRequest) (ProxyStreamResult, error) {
	session, err := uc.sessionReader.GetOrCreate(ctx, apiKeyID, chatReq.Model)
	if err != nil {
		return ProxyStreamResult{}, err
	}

	var retries []RetryAttempt
	var excludeID *vo.AccountID

	for range maxStreamRetries {
		account, err := uc.accountSelector.Execute(ctx, session.AccountID(), excludeID, uc.clock.Now())
		if err != nil {
			return ProxyStreamResult{Retries: retries}, errs.ErrAllAccountsExhausted
		}

		creds := account.Credentials()
		events, err := uc.chatStreamer.StreamChat(ctx, creds, chatReq)

		if err != nil {
			// Pre-stream error — retry is possible
			var classErr ClassifiedError
			if !errors.As(err, &classErr) {
				return ProxyStreamResult{Retries: retries}, err
			}

			fbResult, fbErr := uc.fallbackHandler.Execute(ctx, account, classErr.Classification())
			if fbErr != nil {
				return ProxyStreamResult{Retries: retries}, fbErr
			}

			retries = append(retries, RetryAttempt{
				AccountID:      account.ID(),
				Classification: classErr.Classification(),
				RetryIndex:     len(retries),
			})

			if fbResult.ShouldFallback {
				id := account.ID()
				excludeID = &id
				continue
			}
			return ProxyStreamResult{Retries: retries}, err
		}

		// Stream opened — wrap in goroutine for best-effort cleanup
		output := make(chan vo.StreamEvent)
		go func() {
			defer close(output)
			for evt := range events {
				output <- evt
			}
			// Best-effort: record success and save session
			if err := uc.accountWriter.RecordSuccess(context.Background(), account.ID(), uc.clock.Now()); err != nil {
				slog.Error("proxy_stream: RecordSuccess failed", "error", err, "account_id", account.ID())
			}
			session.BindAccount(account.ID())
			session.RecordRequest(uc.clock.Now())
			if err := uc.sessionWriter.Save(context.Background(), session); err != nil {
				slog.Error("proxy_stream: session save failed", "error", err)
			}
		}()

		return ProxyStreamResult{
			Events:      output,
			Retries:     retries,
			AccountID:   account.ID().String(),
			AccountName: account.Name(),
		}, nil
	}

	return ProxyStreamResult{Retries: retries}, errs.ErrAllAccountsExhausted
}
