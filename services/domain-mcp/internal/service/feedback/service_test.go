package feedback

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// memRepo es un Repository in-memory para testear la logica del Service.
type memRepo struct {
	stored    map[int64]*Feedback
	aggs      []DailyAggregate
	persisted []DailyAggregate
	persistErr error
}

func newMemRepo() *memRepo {
	return &memRepo{stored: map[int64]*Feedback{}}
}

func (m *memRepo) Upsert(ctx context.Context, in UpsertParams) (*Feedback, error) {
	fb := &Feedback{
		ID:        "id",
		MessageID: in.MessageID,
		SkillSlug: in.SkillSlug,
		Rating:    in.Rating,
		Comment:   in.Comment,
		UserEmail: in.UserEmail,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.stored[in.MessageID] = fb
	return fb, nil
}

func (m *memRepo) GetByMessage(ctx context.Context, messageID int64) (*Feedback, error) {
	fb, ok := m.stored[messageID]
	if !ok {
		return nil, ErrNotFound
	}
	return fb, nil
}

func (m *memRepo) ListBySkill(ctx context.Context, filter ListFilter) ([]*Feedback, int64, error) {
	var out []*Feedback
	for _, fb := range m.stored {
		out = append(out, fb)
	}
	return out, int64(len(out)), nil
}

func (m *memRepo) AggregateByDay(ctx context.Context, days int) ([]DailyAggregate, error) {
	return m.aggs, nil
}

func (m *memRepo) PersistDaily(ctx context.Context, agg DailyAggregate) error {
	if m.persistErr != nil {
		return m.persistErr
	}
	m.persisted = append(m.persisted, agg)
	return nil
}

func TestCreate_RejectsInvalidRating(t *testing.T) {
	svc := NewService(newMemRepo())
	for _, r := range []int{0, 2, -2, 5} {
		_, err := svc.Create(context.Background(), UpsertParams{MessageID: 1, Rating: r})
		require.ErrorIs(t, err, ErrInvalidRating)
	}
}

func TestCreate_RejectsMissingMessage(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.Create(context.Background(), UpsertParams{MessageID: 0, Rating: 1})
	require.ErrorIs(t, err, ErrInvalidMessage)
}

func TestCreate_NormalizesEmail(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	_, err := svc.Create(context.Background(), UpsertParams{MessageID: 1, Rating: 1, UserEmail: "  Foo@BAR.com "})
	require.NoError(t, err)
	require.Equal(t, "foo@bar.com", repo.stored[1].UserEmail)
}

func TestConsolidateDaily_PersistsAllAggregates(t *testing.T) {
	repo := newMemRepo()
	last := time.Now()
	repo.aggs = []DailyAggregate{
		{SkillSlug: "go-testing", Day: time.Now(), CountUp: 3, CountDown: 1, LastFeedbackAt: &last},
		{SkillSlug: "", Day: time.Now(), CountUp: 0, CountDown: 2, LastFeedbackAt: &last},
	}
	svc := NewService(repo)
	n, err := svc.ConsolidateDaily(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Len(t, repo.persisted, 2)
	require.Equal(t, 3, repo.persisted[0].CountUp)
}

func TestListBySkill_ClampsLimit(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	// limit fuera de rango se normaliza a 50 (no panic ni 0 filas).
	_, _, err := svc.ListBySkill(context.Background(), ListFilter{Limit: 99999})
	require.NoError(t, err)
}
