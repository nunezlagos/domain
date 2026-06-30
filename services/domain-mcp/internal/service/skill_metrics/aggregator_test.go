package skill_metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fakeRepo simula el Repository para testear la orquestacion del Aggregator y
// del Service sin DB. El computo SQL (definicion de exito/fallo, p95, division
// por cero) se prueba en integracion contra Postgres; aqui se inyectan los
// AggregateResult ya computados para cubrir los edge cases de la spec.
type fakeRepo struct {
	activeIDs []uuid.UUID

	// aggByDay devuelve el resultado pre-computado por (skill_id|day).
	aggByDay map[string]AggregateResult
	aggErr   error

	persisted map[string]AggregateResult

	streaks       []LowRateStreak
	firePerStreak int
	fireErr       error

	rollupCalled bool
	cleanupDaily int64
	cleanupWeek  int64
}

func key(id uuid.UUID, day time.Time) string { return id.String() + "|" + day.Format("2006-01-02") }

func (f *fakeRepo) ListActiveSkillIDs(ctx context.Context) ([]uuid.UUID, error) {
	return f.activeIDs, nil
}
func (f *fakeRepo) AggregateDay(ctx context.Context, skillID uuid.UUID, day time.Time) (AggregateResult, error) {
	if f.aggErr != nil {
		return AggregateResult{}, f.aggErr
	}
	return f.aggByDay[key(skillID, day)], nil
}
func (f *fakeRepo) PersistDaily(ctx context.Context, skillID uuid.UUID, day time.Time, r AggregateResult) error {
	if f.persisted == nil {
		f.persisted = map[string]AggregateResult{}
	}
	f.persisted[key(skillID, day)] = r
	return nil
}
func (f *fakeRepo) GetDailyBySkill(ctx context.Context, skillID uuid.UUID, days int) ([]DailyMetric, error) {
	return nil, nil
}
func (f *fakeRepo) GetDailyByDay(ctx context.Context, day time.Time) ([]DailyMetric, error) {
	return nil, nil
}
func (f *fakeRepo) ListTopFailed(ctx context.Context, days, limit int) ([]TopFailed, error) {
	return nil, nil
}
func (f *fakeRepo) ListSlowest(ctx context.Context, days, limit int) ([]Slowest, error) {
	return nil, nil
}
func (f *fakeRepo) RollupWeek(ctx context.Context, weekStart time.Time) error {
	f.rollupCalled = true
	return nil
}
func (f *fakeRepo) CleanupDaily(ctx context.Context, retentionDays int) (int64, error) {
	return f.cleanupDaily, nil
}
func (f *fakeRepo) CleanupWeekly(ctx context.Context, retentionDays int) (int64, error) {
	return f.cleanupWeek, nil
}
func (f *fakeRepo) ListLowRateStreaks(ctx context.Context, threshold float64) ([]LowRateStreak, error) {
	return f.streaks, nil
}
func (f *fakeRepo) FireAlerts(ctx context.Context, streak LowRateStreak) (int, error) {
	if f.fireErr != nil {
		return 0, f.fireErr
	}
	return f.firePerStreak, nil
}

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func TestAggregate_PersistsComputedResult(t *testing.T) {
	id := uuid.New()
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	want := AggregateResult{
		InvocationsCount: 12, SuccessCount: 10, FailureCount: 2,
		SuccessRate: ptrF(83.33), AvgDurationMs: ptrI(420), P95DurationMs: ptrI(900),
	}
	repo := &fakeRepo{aggByDay: map[string]AggregateResult{key(id, day): want}}
	agg := NewAggregator(repo)

	got, err := agg.Aggregate(context.Background(), id, day)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, want, repo.persisted[key(id, day)], "debe persistir el resultado computado")
}

func TestAggregate_ZeroInvocations(t *testing.T) {
	id := uuid.New()
	day := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	// 0 invocaciones: success_rate NULL, sin avg ni p95.
	zero := AggregateResult{InvocationsCount: 0, SuccessRate: nil, AvgDurationMs: nil, P95DurationMs: nil}
	repo := &fakeRepo{aggByDay: map[string]AggregateResult{key(id, day): zero}}
	agg := NewAggregator(repo)

	got, err := agg.Aggregate(context.Background(), id, day)
	require.NoError(t, err)
	require.Equal(t, 0, got.InvocationsCount)
	require.Nil(t, got.SuccessRate, "0 invocaciones -> success_rate NULL (no 0% espurio)")
	require.Nil(t, got.P95DurationMs)
}

func TestAggregate_HundredPercentSuccess(t *testing.T) {
	id := uuid.New()
	day := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	full := AggregateResult{
		InvocationsCount: 15, SuccessCount: 15, FailureCount: 0,
		SuccessRate: ptrF(100), AvgDurationMs: ptrI(300), P95DurationMs: ptrI(500),
	}
	repo := &fakeRepo{aggByDay: map[string]AggregateResult{key(id, day): full}}
	got, err := NewAggregator(repo).Aggregate(context.Background(), id, day)
	require.NoError(t, err)
	require.NotNil(t, got.SuccessRate)
	require.Equal(t, 100.0, *got.SuccessRate)
}

func TestAggregate_ZeroPercentSuccess(t *testing.T) {
	id := uuid.New()
	day := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	none := AggregateResult{
		InvocationsCount: 11, SuccessCount: 0, FailureCount: 11,
		SuccessRate: ptrF(0), AvgDurationMs: ptrI(700), P95DurationMs: ptrI(1200),
	}
	repo := &fakeRepo{aggByDay: map[string]AggregateResult{key(id, day): none}}
	got, err := NewAggregator(repo).Aggregate(context.Background(), id, day)
	require.NoError(t, err)
	require.NotNil(t, got.SuccessRate)
	require.Equal(t, 0.0, *got.SuccessRate, "0% es un valor real, NO NULL (hubo invocaciones)")
}

func TestAggregate_BelowTenInvocationsNoP95(t *testing.T) {
	id := uuid.New()
	day := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	// <10 invocaciones: success_rate computa, pero p95 NO (data insuficiente).
	few := AggregateResult{
		InvocationsCount: 9, SuccessCount: 8, FailureCount: 1,
		SuccessRate: ptrF(88.89), AvgDurationMs: ptrI(400), P95DurationMs: nil,
	}
	repo := &fakeRepo{aggByDay: map[string]AggregateResult{key(id, day): few}}
	got, err := NewAggregator(repo).Aggregate(context.Background(), id, day)
	require.NoError(t, err)
	require.NotNil(t, got.SuccessRate, "success_rate sí se computa con <10")
	require.Nil(t, got.P95DurationMs, "<10 invocaciones -> p95 NULL")
}

func TestAggregateAll_ProcessesEverySkill(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	day := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{activeIDs: ids, aggByDay: map[string]AggregateResult{}}
	for _, id := range ids {
		repo.aggByDay[key(id, day)] = AggregateResult{InvocationsCount: 1, SuccessCount: 1, SuccessRate: ptrF(100)}
	}
	n, err := NewAggregator(repo).AggregateAll(context.Background(), day)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Len(t, repo.persisted, 3)
}

func TestAggregateAll_AbortsOnError(t *testing.T) {
	repo := &fakeRepo{activeIDs: []uuid.UUID{uuid.New()}, aggErr: errors.New("boom")}
	_, err := NewAggregator(repo).AggregateAll(context.Background(), time.Now())
	require.Error(t, err)
}

func TestDetectAndFireAlerts_NoAlertsConfigured(t *testing.T) {
	// Hay streak detectado pero NO hay usage_alerts configuradas (FireAlerts=0).
	// El hook no debe romper: devuelve detectados=1, fires=0.
	repo := &fakeRepo{
		streaks:       []LowRateStreak{{SkillID: uuid.New(), AvgSuccessRate: 40}},
		firePerStreak: 0,
	}
	detected, fired, err := NewAggregator(repo).DetectAndFireAlerts(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, detected)
	require.Equal(t, 0, fired)
}

func TestDetectAndFireAlerts_FiresPerStreak(t *testing.T) {
	repo := &fakeRepo{
		streaks: []LowRateStreak{
			{SkillID: uuid.New(), AvgSuccessRate: 40},
			{SkillID: uuid.New(), AvgSuccessRate: 55},
		},
		firePerStreak: 1,
	}
	detected, fired, err := NewAggregator(repo).DetectAndFireAlerts(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, detected)
	require.Equal(t, 2, fired)
}

func TestCleanup_ReturnsBothCounts(t *testing.T) {
	repo := &fakeRepo{cleanupDaily: 5, cleanupWeek: 2}
	d, w, err := NewAggregator(repo).Cleanup(context.Background(), 90, 365)
	require.NoError(t, err)
	require.Equal(t, int64(5), d)
	require.Equal(t, int64(2), w)
}

func TestRollupWeekly_CallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	err := NewAggregator(repo).RollupWeekly(context.Background(), time.Now())
	require.NoError(t, err)
	require.True(t, repo.rollupCalled)
}

func TestWeekStart_IsMonday(t *testing.T) {
	// 2026-06-26 es viernes; el lunes de esa semana ISO es 2026-06-22.
	got := WeekStart(time.Date(2026, 6, 26, 15, 0, 0, 0, time.UTC))
	require.Equal(t, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), got)

	// Un lunes mapea a si mismo.
	mon := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), WeekStart(mon))

	// Un domingo mapea al lunes anterior.
	sun := time.Date(2026, 6, 28, 23, 0, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), WeekStart(sun))
}

func TestService_NormalizesWindow(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	// No debe panic con valores fuera de rango; el repo recibe valores saneados.
	_, err := svc.ListTopFailed(context.Background(), -1, 0)
	require.NoError(t, err)
	_, err = svc.ListSlowest(context.Background(), 99999, 99999)
	require.NoError(t, err)
	_, err = svc.GetBySkill(context.Background(), uuid.New(), -5)
	require.NoError(t, err)
}
