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
	// REQ-42.3: SessionID removido (columna session_id dropeada de captured_prompts).
	ProjectID  *uuid.UUID
	Content    string
	ClientKind string
	Model      string
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

// CompleteTurn (REQ-47): cierra el turn con el output del LLM, estima
// tokens out y completa turn_completed_at. response_chars=0 es válido
// (turn sin respuesta — útil para trackear timeouts/cancels).
func (s *Service) CompleteTurn(ctx context.Context, in CompleteTurnInput) (*Prompt, error) {
	if in.ResponseChars < 0 {
		in.ResponseChars = 0
	}
	in.Model = strings.TrimSpace(in.Model)
	return s.repo.CompleteTurn(ctx, in)
}

// SummarizeByProject agrega tokens estimados de todos los turns de un project.
func (s *Service) SummarizeByProject(ctx context.Context, orgID, projectID uuid.UUID) (*SessionUsage, error) {
	return s.repo.SummarizeByProject(ctx, orgID, projectID)
}
