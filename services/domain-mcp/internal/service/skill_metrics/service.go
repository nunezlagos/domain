package skill_metrics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service expone la lectura de metricas de skills. La logica de agregacion vive
// en Aggregator (aggregator.go); aqui solo lectura/reporting.
type Service struct {
	repo Repository
}

// NewService inyecta la implementacion concreta del Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetBySkill devuelve la serie diaria de un skill (ultimos `days` dias).
func (s *Service) GetBySkill(ctx context.Context, skillID uuid.UUID, days int) ([]DailyMetric, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	return s.repo.GetDailyBySkill(ctx, skillID, days)
}

// GetByDay devuelve todas las metricas de un dia (todos los skills).
func (s *Service) GetByDay(ctx context.Context, day time.Time) ([]DailyMetric, error) {
	return s.repo.GetDailyByDay(ctx, day)
}

// ListTopFailed devuelve los `limit` skills con peor success_rate en `days`.
func (s *Service) ListTopFailed(ctx context.Context, days, limit int) ([]TopFailed, error) {
	days, limit = normalizeWindow(days, limit)
	return s.repo.ListTopFailed(ctx, days, limit)
}

// ListSlowest devuelve los `limit` skills con peor p95 en `days`.
func (s *Service) ListSlowest(ctx context.Context, days, limit int) ([]Slowest, error) {
	days, limit = normalizeWindow(days, limit)
	return s.repo.ListSlowest(ctx, days, limit)
}

func normalizeWindow(days, limit int) (int, int) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}
	return days, limit
}
