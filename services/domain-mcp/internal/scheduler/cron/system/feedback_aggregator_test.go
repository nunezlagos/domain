package systemcron

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	feedbacksvc "nunezlagos/domain/internal/service/feedback"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// aggRepo es un Repository fake para alimentar el aggregator con datos de prueba.
type aggRepo struct {
	aggs      []feedbacksvc.DailyAggregate
	persisted []feedbacksvc.DailyAggregate
}

func (r *aggRepo) Upsert(ctx context.Context, in feedbacksvc.UpsertParams) (*feedbacksvc.Feedback, error) {
	return &feedbacksvc.Feedback{MessageID: in.MessageID, Rating: in.Rating}, nil
}
func (r *aggRepo) GetByMessage(ctx context.Context, messageID int64) (*feedbacksvc.Feedback, error) {
	return nil, feedbacksvc.ErrNotFound
}
func (r *aggRepo) ListBySkill(ctx context.Context, f feedbacksvc.ListFilter) ([]*feedbacksvc.Feedback, int64, error) {
	return nil, 0, nil
}
func (r *aggRepo) AggregateByDay(ctx context.Context, days int) ([]feedbacksvc.DailyAggregate, error) {
	return r.aggs, nil
}
func (r *aggRepo) PersistDaily(ctx context.Context, agg feedbacksvc.DailyAggregate) error {
	r.persisted = append(r.persisted, agg)
	return nil
}

func TestFeedbackAggregator_NilServiceExitsClean(t *testing.T) {
	agg := &FeedbackAggregator{Feedback: nil}
	// No debe panic ni bloquear: detecta dep faltante y sale.
	agg.Start(context.Background())
}

func TestFeedbackAggregator_RunTickConsolidates(t *testing.T) {
	now := time.Now()
	repo := &aggRepo{aggs: []feedbacksvc.DailyAggregate{
		{SkillSlug: "go-testing", Day: now, CountUp: 5, CountDown: 2, LastFeedbackAt: &now},
		{SkillSlug: "deep-research", Day: now, CountUp: 1, CountDown: 0, LastFeedbackAt: &now},
	}}
	agg := &FeedbackAggregator{
		Feedback: feedbacksvc.NewService(repo),
		Days:     7,
	}
	agg.runTick(context.Background(), discardLogger())
	require.Len(t, repo.persisted, 2)
	require.Equal(t, 5, repo.persisted[0].CountUp)
}
