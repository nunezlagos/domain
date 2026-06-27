// Package feedback — HU-52.1: user feedback loop (👍/👎) sobre cada respuesta
// del chat IA. Es el prerequisito de REQ-52.
//
// Repository + Service pattern, igual que observation/capturedprompt. El Service
// contiene la logica de negocio (validacion de rating, upsert idempotente por
// message_id, privacy del comment) y delega persistencia a Repository. La
// implementacion concreta (pg_repository.go) honra la tx-context (issue-25.14).
//
// Single-tenant (regla dura 1): NADA de organization_id. El aislamiento es por
// message_id (que ya pertenece a una conversacion de un user_email).
package feedback

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound cuando no existe feedback para el message_id pedido.
var ErrNotFound = errors.New("feedback not found")

// ErrInvalidRating cuando rating no es +1 ni -1.
var ErrInvalidRating = errors.New("feedback: rating debe ser 1 (👍) o -1 (👎)")

// ErrInvalidMessage cuando message_id <= 0.
var ErrInvalidMessage = errors.New("feedback: message_id requerido")

// Feedback es un voto sobre una respuesta del assistant.
type Feedback struct {
	ID        string    `json:"id"`
	MessageID int64     `json:"message_id"`
	SkillSlug string    `json:"skill_slug,omitempty"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment,omitempty"`
	UserEmail string    `json:"user_email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertParams agrupa los campos del upsert (1 feedback por message_id).
type UpsertParams struct {
	MessageID int64
	SkillSlug string
	Rating    int
	Comment   string
	UserEmail string
}

// ListFilter para ListBySkill paginado.
type ListFilter struct {
	SkillSlug string
	// Rating: 0 = sin filtro; 1 = solo 👍; -1 = solo 👎.
	Rating int
	Limit  int
	Offset int
}

// DailyAggregate es un agregado por skill_slug y dia (self-contained, NO
// skill_metrics; esa integracion llega en HU-52.2).
type DailyAggregate struct {
	SkillSlug      string     `json:"skill_slug"`
	Day            time.Time  `json:"day"`
	CountUp        int        `json:"count_up"`
	CountDown      int        `json:"count_down"`
	LastFeedbackAt *time.Time `json:"last_feedback_at,omitempty"`
}

// Repository contrato de persistencia.
type Repository interface {
	Upsert(ctx context.Context, in UpsertParams) (*Feedback, error)
	GetByMessage(ctx context.Context, messageID int64) (*Feedback, error)
	ListBySkill(ctx context.Context, filter ListFilter) ([]*Feedback, int64, error)
	AggregateByDay(ctx context.Context, days int) ([]DailyAggregate, error)
	// PersistDaily escribe un agregado diario en skill_feedback_daily (cron).
	PersistDaily(ctx context.Context, agg DailyAggregate) error
}
