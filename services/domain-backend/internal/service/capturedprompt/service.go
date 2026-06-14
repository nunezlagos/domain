package capturedprompt

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrEmptyContent = errors.New("captured_prompt: content required")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CaptureInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	SessionID      *uuid.UUID
	ProjectID      *uuid.UUID
	Content        string
	ClientKind     string
	Model          string
}

// Capture persiste un prompt del usuario. char_count se computa server-side
// como proxy de tokens hasta tener integración con el cliente IDE real.
func (s *Service) Capture(ctx context.Context, in CaptureInput) (*Prompt, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, ErrEmptyContent
	}
	return s.repo.Insert(ctx, InsertParams{
		OrganizationID: in.OrganizationID,
		UserID:         in.UserID,
		SessionID:      in.SessionID,
		ProjectID:      in.ProjectID,
		Content:        content,
		ClientKind:     strings.TrimSpace(in.ClientKind),
		Model:          strings.TrimSpace(in.Model),
		CharCount:      utf8.RuneCountInString(content),
	})
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Prompt, int64, error) {
	return s.repo.List(ctx, orgID, filter)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Prompt, error) {
	return s.repo.Get(ctx, orgID, id)
}
