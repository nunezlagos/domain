package feedback

import (
	"context"
	"strings"
)

// Service contiene la logica de negocio del feedback loop.
type Service struct {
	repo Repository
}

// NewService inyecta la implementacion concreta del Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create persiste (o actualiza) el feedback de un mensaje. Idempotente por
// message_id: el segundo submit del mismo mensaje hace UPDATE (upsert).
//
// Privacy (regla del HU): el comment puede contener PII. NO se loguea en
// audit_log; el Service solo valida y persiste. Si en el futuro se audita algo,
// auditar SOLO el rating numerico, nunca el comment.
func (s *Service) Create(ctx context.Context, in UpsertParams) (*Feedback, error) {
	if in.MessageID <= 0 {
		return nil, ErrInvalidMessage
	}
	if in.Rating != 1 && in.Rating != -1 {
		return nil, ErrInvalidRating
	}
	in.SkillSlug = strings.TrimSpace(in.SkillSlug)
	in.Comment = strings.TrimSpace(in.Comment)
	in.UserEmail = strings.ToLower(strings.TrimSpace(in.UserEmail))
	return s.repo.Upsert(ctx, in)
}

// GetByMessage devuelve el feedback de un mensaje, o ErrNotFound.
func (s *Service) GetByMessage(ctx context.Context, messageID int64) (*Feedback, error) {
	if messageID <= 0 {
		return nil, ErrInvalidMessage
	}
	return s.repo.GetByMessage(ctx, messageID)
}

// ListBySkill lista feedback paginado, opcionalmente filtrado por skill_slug
// y/o rating. Devuelve (items, total, error).
func (s *Service) ListBySkill(ctx context.Context, filter ListFilter) ([]*Feedback, int64, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	// Normaliza rating a 0/1/-1; cualquier otro valor = sin filtro.
	if filter.Rating != 1 && filter.Rating != -1 {
		filter.Rating = 0
	}
	filter.SkillSlug = strings.TrimSpace(filter.SkillSlug)
	return s.repo.ListBySkill(ctx, filter)
}

// AggregateByDay computa agregados (count_up/count_down/last_feedback_at) por
// skill_slug y dia, para los ultimos `days` dias. Self-contained: NO toca
// skill_metrics (HU-52.2).
func (s *Service) AggregateByDay(ctx context.Context, days int) ([]DailyAggregate, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	return s.repo.AggregateByDay(ctx, days)
}

// ConsolidateDaily es el trabajo del aggregator cron: computa los agregados de
// los ultimos `days` dias y los persiste en skill_feedback_daily. Devuelve
// cuantas filas se consolidaron. Self-contained (NO skill_metrics).
func (s *Service) ConsolidateDaily(ctx context.Context, days int) (int, error) {
	aggs, err := s.AggregateByDay(ctx, days)
	if err != nil {
		return 0, err
	}
	persisted := 0
	for _, agg := range aggs {
		if err := s.repo.PersistDaily(ctx, agg); err != nil {
			return persisted, err
		}
		persisted++
	}
	return persisted, nil
}
