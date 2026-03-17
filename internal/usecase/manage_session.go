package usecase

import (
	"context"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// SessionReader abstracts session retrieval/creation.
type SessionReader interface {
	GetOrCreate(ctx context.Context, apiKeyID vo.APIKeyID, model vo.ModelName) (*entity.Session, error)
}

// SessionWriter abstracts session persistence.
type SessionWriter interface {
	Save(ctx context.Context, session *entity.Session) error
}

// ManageSessionUseCase orchestrates session lifecycle.
type ManageSessionUseCase struct {
	sessionReader SessionReader
	sessionWriter SessionWriter
}

// NewManageSessionUseCase creates a new ManageSessionUseCase.
func NewManageSessionUseCase(reader SessionReader, writer SessionWriter) *ManageSessionUseCase {
	return &ManageSessionUseCase{
		sessionReader: reader,
		sessionWriter: writer,
	}
}

// GetOrCreate retrieves an existing session or creates a new one.
func (uc *ManageSessionUseCase) GetOrCreate(ctx context.Context, apiKeyID vo.APIKeyID, model vo.ModelName) (*entity.Session, error) {
	return uc.sessionReader.GetOrCreate(ctx, apiKeyID, model)
}

// SaveSession persists the session state.
func (uc *ManageSessionUseCase) SaveSession(ctx context.Context, session *entity.Session) error {
	return uc.sessionWriter.Save(ctx, session)
}
