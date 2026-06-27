package skill_ab_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/skill_ab_test/skillabtestdb"
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
func (r *pgRepository) q(ctx context.Context) *skillabtestdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skillabtestdb.New(tx)
	}
	return skillabtestdb.New(r.pool)
}

func (r *pgRepository) Create(ctx context.Context, p CreateParams) (*ABTest, error) {
	var startedAt pgtype.Timestamptz
	status := StatusRunning
	if p.StartNow {
		startedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	row, err := r.q(ctx).CreateABTest(ctx, skillabtestdb.CreateABTestParams{
		SkillSlug:       p.SkillSlug,
		VersionA:        int32(p.VersionA),
		VersionB:        int32(p.VersionB),
		TrafficSplitA:   floatToNumeric(p.TrafficSplitA),
		MinInvocations:  int32(p.MinInvocations),
		AutoApplyWinner: p.AutoApplyWinner,
		StartedAt:       startedAt,
		Status:          status,
		CreatedBy:       p.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create ab test: %w", err)
	}
	return toABTest(row), nil
}

func (r *pgRepository) Get(ctx context.Context, id uuid.UUID) (*ABTest, error) {
	row, err := r.q(ctx).GetABTest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ab test: %w", err)
	}
	return toABTest(row), nil
}

func (r *pgRepository) GetRunningBySlug(ctx context.Context, slug string) (*ABTest, error) {
	row, err := r.q(ctx).GetRunningBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get running by slug: %w", err)
	}
	return toABTest(row), nil
}

func (r *pgRepository) ListRunning(ctx context.Context) ([]*ABTest, error) {
	rows, err := r.q(ctx).ListRunningABTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("list running: %w", err)
	}
	out := make([]*ABTest, 0, len(rows))
	for _, row := range rows {
		out = append(out, toABTest(row))
	}
	return out, nil
}

func (r *pgRepository) Start(ctx context.Context, id uuid.UUID) error {
	if err := r.q(ctx).StartABTest(ctx, id); err != nil {
		return fmt.Errorf("start ab test: %w", err)
	}
	return nil
}

func (r *pgRepository) DeclareWinner(ctx context.Context, id uuid.UUID, winner string, confidence *float64) error {
	w := winner
	err := r.q(ctx).DeclareWinner(ctx, skillabtestdb.DeclareWinnerParams{
		Winner:     &w,
		Confidence: floatPtrToNumeric(confidence),
		ID:         id,
	})
	if err != nil {
		return fmt.Errorf("declare winner: %w", err)
	}
	return nil
}

func (r *pgRepository) Cancel(ctx context.Context, id uuid.UUID) error {
	if err := r.q(ctx).CancelABTest(ctx, id); err != nil {
		return fmt.Errorf("cancel ab test: %w", err)
	}
	return nil
}

func (r *pgRepository) GetResults(ctx context.Context, testID uuid.UUID) ([]VariantResult, error) {
	rows, err := r.q(ctx).GetResults(ctx, testID)
	if err != nil {
		return nil, fmt.Errorf("get results: %w", err)
	}
	out := make([]VariantResult, 0, len(rows))
	for _, row := range rows {
		sr := row.SuccessRate
		af := row.AvgFeedback
		out = append(out, VariantResult{
			Version:          row.Version,
			InvocationsCount: int(row.InvocationsCount),
			SuccessCount:     int(row.SuccessCount),
			SuccessRate:      &sr,
			AvgFeedback:      &af,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return out, nil
}

func (r *pgRepository) IncrementResult(ctx context.Context, testID uuid.UUID, v Variant, success bool) error {
	delta := int32(0)
	if success {
		delta = 1
	}
	err := r.q(ctx).IncrementResult(ctx, skillabtestdb.IncrementResultParams{
		AbTestID:     testID,
		Version:      string(v),
		SuccessDelta: delta,
	})
	if err != nil {
		return fmt.Errorf("increment result: %w", err)
	}
	return nil
}

func (r *pgRepository) UpsertResult(ctx context.Context, testID uuid.UUID, v VariantResult) error {
	err := r.q(ctx).UpsertResult(ctx, skillabtestdb.UpsertResultParams{
		AbTestID:         testID,
		Version:          v.Version,
		InvocationsCount: int32(v.InvocationsCount),
		SuccessCount:     int32(v.SuccessCount),
		SuccessRate:      floatPtrToNumeric(v.SuccessRate),
		AvgFeedback:      floatPtrToNumeric(v.AvgFeedback),
	})
	if err != nil {
		return fmt.Errorf("upsert result: %w", err)
	}
	return nil
}

func (r *pgRepository) SkillIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	id, err := r.q(ctx).SkillIDBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("skill id by slug: %w", err)
	}
	return id, nil
}

func (r *pgRepository) PinSkillVersion(ctx context.Context, skillID uuid.UUID, version int) error {
	v := int32(version)
	err := r.q(ctx).PinSkillVersion(ctx, skillabtestdb.PinSkillVersionParams{
		Version: &v,
		ID:      skillID,
	})
	if err != nil {
		return fmt.Errorf("pin skill version: %w", err)
	}
	return nil
}

// ---- mapeo db -> dominio ----

func toABTest(row skillabtestdb.SkillAbTest) *ABTest {
	t := &ABTest{
		ID:              row.ID,
		SkillSlug:       row.SkillSlug,
		VersionA:        int(row.VersionA),
		VersionB:        int(row.VersionB),
		TrafficSplitA:   numericToFloat(row.TrafficSplitA, DefaultTrafficSplitA),
		MinInvocations:  int(row.MinInvocations),
		AutoApplyWinner: row.AutoApplyWinner,
		Winner:          row.Winner,
		Confidence:      numericToFloatPtr(row.Confidence),
		Status:          row.Status,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
	}
	if row.StartedAt.Valid {
		s := row.StartedAt.Time
		t.StartedAt = &s
	}
	if row.EndedAt.Valid {
		e := row.EndedAt.Time
		t.EndedAt = &e
	}
	return t
}

// ---- conversiones pgtype <-> Go ----

func numericToFloat(n pgtype.Numeric, fallback float64) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return fallback
	}
	return f.Float64
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func floatPtrToNumeric(p *float64) pgtype.Numeric {
	if p == nil {
		return pgtype.Numeric{Valid: false}
	}
	return floatToNumeric(*p)
}

func floatToNumeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.6f", v))
	return n
}
