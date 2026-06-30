package feedback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/feedback/feedbackdb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository crea el Repository concreto sobre pgx.
func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// q resuelve el Queries contra la tx-context (RLS/tx) si existe, o el pool.
func (r *pgRepository) q(ctx context.Context) *feedbackdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return feedbackdb.New(tx)
	}
	return feedbackdb.New(r.pool)
}

func (r *pgRepository) Upsert(ctx context.Context, in UpsertParams) (*Feedback, error) {
	row, err := r.q(ctx).UpsertFeedback(ctx, feedbackdb.UpsertFeedbackParams{
		MessageID: in.MessageID,
		SkillSlug: in.SkillSlug,
		Rating:    int16(in.Rating),
		Comment:   in.Comment,
		UserEmail: in.UserEmail,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert feedback: %w", err)
	}
	f := Feedback{
		ID:        row.ID.String(),
		MessageID: row.MessageID,
		SkillSlug: row.SkillSlug,
		Rating:    int(row.Rating),
		Comment:   row.Comment,
		UserEmail: row.UserEmail,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	return &f, nil
}

func (r *pgRepository) GetByMessage(ctx context.Context, messageID int64) (*Feedback, error) {
	row, err := r.q(ctx).GetFeedbackByMessage(ctx, messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get feedback by message: %w", err)
	}
	f := Feedback{
		ID:        row.ID.String(),
		MessageID: row.MessageID,
		SkillSlug: row.SkillSlug,
		Rating:    int(row.Rating),
		Comment:   row.Comment,
		UserEmail: row.UserEmail,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	return &f, nil
}

func (r *pgRepository) ListBySkill(ctx context.Context, filter ListFilter) ([]*Feedback, int64, error) {
	total, err := r.q(ctx).CountFeedbackBySkill(ctx, feedbackdb.CountFeedbackBySkillParams{
		SkillSlug:    filter.SkillSlug,
		RatingFilter: int16(filter.Rating),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count feedback: %w", err)
	}

	rows, err := r.q(ctx).ListFeedbackBySkill(ctx, feedbackdb.ListFeedbackBySkillParams{
		SkillSlug:    filter.SkillSlug,
		RatingFilter: int16(filter.Rating),
		ResultLimit:  int32(filter.Limit),
		ResultOffset: int32(filter.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list feedback: %w", err)
	}

	out := make([]*Feedback, 0, len(rows))
	for _, row := range rows {
		out = append(out, &Feedback{
			ID:        row.ID.String(),
			MessageID: row.MessageID,
			SkillSlug: row.SkillSlug,
			Rating:    int(row.Rating),
			Comment:   row.Comment,
			UserEmail: row.UserEmail,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, total, nil
}

func (r *pgRepository) AggregateByDay(ctx context.Context, days int) ([]DailyAggregate, error) {
	rows, err := r.q(ctx).AggregateByDay(ctx, int32(days))
	if err != nil {
		return nil, fmt.Errorf("aggregate by day: %w", err)
	}
	out := make([]DailyAggregate, 0, len(rows))
	for _, row := range rows {
		agg := DailyAggregate{
			SkillSlug: row.SkillSlug,
			CountUp:   int(row.CountUp),
			CountDown: int(row.CountDown),
		}
		if row.Day.Valid {
			agg.Day = row.Day.Time
		}
		// last_feedback_at lo genera sqlc como interface{} (MAX sin override).
		// pgx lo entrega como time.Time cuando hay filas.
		if t, ok := row.LastFeedbackAt.(time.Time); ok {
			tt := t
			agg.LastFeedbackAt = &tt
		}
		out = append(out, agg)
	}
	return out, nil
}

func (r *pgRepository) PersistDaily(ctx context.Context, agg DailyAggregate) error {
	var last pgtype.Timestamptz
	if agg.LastFeedbackAt != nil {
		last = pgtype.Timestamptz{Time: *agg.LastFeedbackAt, Valid: true}
	}
	err := r.q(ctx).UpsertFeedbackDaily(ctx, feedbackdb.UpsertFeedbackDailyParams{
		SkillSlug:      agg.SkillSlug,
		Day:            pgtype.Date{Time: agg.Day, Valid: true},
		CountUp:        int32(agg.CountUp),
		CountDown:      int32(agg.CountDown),
		LastFeedbackAt: last,
	})
	if err != nil {
		return fmt.Errorf("persist daily aggregate: %w", err)
	}
	return nil
}
