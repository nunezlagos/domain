// Package capturedprompt — REQ-41: persistir prompts del usuario para
// análisis posterior. Repository + Service pattern (Strangler Fig).
//
// Diferente a `prompt` (templates reutilizables) y a `intake` (intakes
// clasificados): acá guardamos raw_text del user sin filtrar, con
// metadata mínima (session, project, char_count proxy de tokens).
package capturedprompt

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("captured prompt not found")

type Prompt struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UserID         uuid.UUID  `json:"user_id"`
	SessionID      *uuid.UUID `json:"session_id,omitempty"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	Content        string     `json:"content"`
	ClientKind     string     `json:"client_kind,omitempty"`
	Model          string     `json:"model,omitempty"`
	CharCount      int        `json:"char_count"`
	CapturedAt     time.Time  `json:"captured_at"`
}

type InsertParams struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	SessionID      *uuid.UUID
	ProjectID      *uuid.UUID
	Content        string
	ClientKind     string
	Model          string
	CharCount      int
}

type ListFilter struct {
	SessionID *uuid.UUID
	ProjectID *uuid.UUID
	UserID    *uuid.UUID
	Limit     int
	Offset    int
}

type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Prompt, error)
	List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Prompt, int64, error)
	Get(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*Prompt, error)
}
