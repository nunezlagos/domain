package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/ratelimit"
	feedbacksvc "nunezlagos/domain/internal/service/feedback"
)

// fakeFeedbackRepo es un Repository in-memory para testear el handler sin DB.
type fakeFeedbackRepo struct {
	byMessage map[int64]*feedbacksvc.Feedback
	upsertErr error
}

func newFakeFeedbackRepo() *fakeFeedbackRepo {
	return &fakeFeedbackRepo{byMessage: map[int64]*feedbacksvc.Feedback{}}
}

func (f *fakeFeedbackRepo) Upsert(ctx context.Context, in feedbacksvc.UpsertParams) (*feedbacksvc.Feedback, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	fb := &feedbacksvc.Feedback{
		ID:        "00000000-0000-0000-0000-000000000001",
		MessageID: in.MessageID,
		SkillSlug: in.SkillSlug,
		Rating:    in.Rating,
		Comment:   in.Comment,
		UserEmail: in.UserEmail,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.byMessage[in.MessageID] = fb
	return fb, nil
}

func (f *fakeFeedbackRepo) GetByMessage(ctx context.Context, messageID int64) (*feedbacksvc.Feedback, error) {
	fb, ok := f.byMessage[messageID]
	if !ok {
		return nil, feedbacksvc.ErrNotFound
	}
	return fb, nil
}

func (f *fakeFeedbackRepo) ListBySkill(ctx context.Context, filter feedbacksvc.ListFilter) ([]*feedbacksvc.Feedback, int64, error) {
	var out []*feedbacksvc.Feedback
	for _, fb := range f.byMessage {
		if filter.SkillSlug != "" && fb.SkillSlug != filter.SkillSlug {
			continue
		}
		if filter.Rating != 0 && fb.Rating != filter.Rating {
			continue
		}
		out = append(out, fb)
	}
	return out, int64(len(out)), nil
}

func (f *fakeFeedbackRepo) AggregateByDay(ctx context.Context, days int) ([]feedbacksvc.DailyAggregate, error) {
	return []feedbacksvc.DailyAggregate{
		{SkillSlug: "go-testing", Day: time.Now(), CountUp: 2, CountDown: 1},
	}, nil
}

func (f *fakeFeedbackRepo) PersistDaily(ctx context.Context, agg feedbacksvc.DailyAggregate) error {
	return nil
}

func newFeedbackAPI() (*API, *fakeFeedbackRepo) {
	repo := newFakeFeedbackRepo()
	return &API{
		Feedback:        feedbacksvc.NewService(repo),
		FeedbackLimiter: ratelimit.New(30, 30.0/60.0),
	}, repo
}

func postFeedback(t *testing.T, api *API, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	api.createFeedback(rec, req)
	return rec
}

func TestCreateFeedback_ValidRatingUp(t *testing.T) {
	api, repo := newFeedbackAPI()
	rec := postFeedback(t, api, map[string]any{
		"message_id": 10, "rating": 1, "skill_slug": "go-testing", "user_email": "a@b.com",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, repo.byMessage[10].Rating)
}

func TestCreateFeedback_ValidRatingDown(t *testing.T) {
	api, repo := newFeedbackAPI()
	rec := postFeedback(t, api, map[string]any{
		"message_id": 11, "rating": -1, "comment": "no sirvio", "user_email": "a@b.com",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, -1, repo.byMessage[11].Rating)
	require.Equal(t, "no sirvio", repo.byMessage[11].Comment)
}

func TestCreateFeedback_InvalidRatingZero(t *testing.T) {
	api, _ := newFeedbackAPI()
	rec := postFeedback(t, api, map[string]any{"message_id": 12, "rating": 0})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid_rating")
}

func TestCreateFeedback_InvalidRatingTwo(t *testing.T) {
	api, _ := newFeedbackAPI()
	rec := postFeedback(t, api, map[string]any{"message_id": 13, "rating": 2})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid_rating")
}

func TestCreateFeedback_MissingMessageID(t *testing.T) {
	api, _ := newFeedbackAPI()
	rec := postFeedback(t, api, map[string]any{"rating": 1})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid_message")
}

func TestCreateFeedback_Idempotent(t *testing.T) {
	api, repo := newFeedbackAPI()
	rec1 := postFeedback(t, api, map[string]any{"message_id": 20, "rating": 1, "user_email": "x@y.com"})
	require.Equal(t, http.StatusOK, rec1.Code)
	rec2 := postFeedback(t, api, map[string]any{"message_id": 20, "rating": -1, "user_email": "x@y.com"})
	require.Equal(t, http.StatusOK, rec2.Code)
	require.Len(t, repo.byMessage, 1)
	require.Equal(t, -1, repo.byMessage[20].Rating)
}

func TestCreateFeedback_RateLimited(t *testing.T) {
	repo := newFakeFeedbackRepo()
	api := &API{
		Feedback:        feedbacksvc.NewService(repo),
		FeedbackLimiter: ratelimit.New(2, 0), // 2 burst, sin refill
	}
	for i := 0; i < 2; i++ {
		rec := postFeedback(t, api, map[string]any{"message_id": 100 + i, "rating": 1, "user_email": "spam@x.com"})
		require.Equal(t, http.StatusOK, rec.Code)
	}
	rec := postFeedback(t, api, map[string]any{"message_id": 999, "rating": 1, "user_email": "spam@x.com"})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestListFeedback_AggregateByDay(t *testing.T) {
	api, _ := newFeedbackAPI()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feedback?days=7", nil)
	rec := httptest.NewRecorder()
	api.listFeedback(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "count_up")
}

func TestListFeedback_ListPaginated(t *testing.T) {
	api, _ := newFeedbackAPI()
	postFeedback(t, api, map[string]any{"message_id": 30, "rating": 1, "skill_slug": "go-testing", "user_email": "a@b.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feedback?skill_slug=go-testing", nil)
	rec := httptest.NewRecorder()
	api.listFeedback(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "\"total\":1")
}
