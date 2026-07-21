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
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`

	ProjectID          *uuid.UUID `json:"project_id,omitempty"`
	Content            string     `json:"content"`
	ClientKind         string     `json:"client_kind,omitempty"`
	Model              string     `json:"model,omitempty"`
	CharCount          int        `json:"char_count"`
	ResponseChars      int        `json:"response_chars"`
	EstimatedTokensIn  int        `json:"estimated_tokens_in"`
	EstimatedTokensOut int        `json:"estimated_tokens_out"`
	CapturedAt         time.Time  `json:"captured_at"`
	TurnCompletedAt    *time.Time `json:"turn_completed_at,omitempty"`
}

// CompleteTurnInput cierra el turn con el output del LLM (REQ-47).
type CompleteTurnInput struct {
	OrganizationID uuid.UUID
	PromptID       uuid.UUID
	ResponseChars  int
	Model          string // opcional, overrides el del Capture
}

// SessionUsage agrega tokens estimados (por project; REQ-42.3: ya no por session).
type SessionUsage struct {
	ProjectID          *uuid.UUID `json:"project_id,omitempty"`
	Turns              int        `json:"turns"`
	EstimatedTokensIn  int64      `json:"estimated_tokens_in"`
	EstimatedTokensOut int64      `json:"estimated_tokens_out"`
	TotalChars         int64      `json:"total_chars"`
}

// PromptCluster es un grupo de prompts con firma normalizada común (patrón
// repetido) con su frecuencia y tokens. La agregación corre en Postgres.
type PromptCluster struct {
	Key    string `json:"cluster_key"`
	Turns  int    `json:"turns"`
	Tokens int64  `json:"tokens"`
	Sample string `json:"sample"`
}

type InsertParams struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	ProjectID      *uuid.UUID
	Content        string
	ClientKind     string
	Model          string
	CharCount      int
}

type ListFilter struct {
	ProjectID *uuid.UUID
	UserID    *uuid.UUID
	Limit     int
	Offset    int
}

type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Prompt, error)
	List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Prompt, int64, error)
	Get(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*Prompt, error)
	CompleteTurn(ctx context.Context, in CompleteTurnInput) (*Prompt, error)
	SummarizeByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) (*SessionUsage, error)
	HeatmapByProject(ctx context.Context, orgID, projectID uuid.UUID, minTurns, maxClusters int) ([]PromptCluster, error)
}
