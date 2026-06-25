package timeline

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/timeline/timelinedb"
	"nunezlagos/domain/internal/store/txctx"
)

var ErrObservationNotFound = errors.New("anchor observation not found")

type EntryKind string

const (
	KindSession     EntryKind = "session"
	KindObservation EntryKind = "observation"
	KindPrompt      EntryKind = "prompt"
)

type Entry struct {
	Kind      EntryKind  `json:"kind"`
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title,omitempty"`
	Preview   string     `json:"preview,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
}

type Snapshot struct {
	ProjectID          *uuid.UUID `json:"project_id"`
	ActiveSession      *Entry     `json:"active_session,omitempty"`
	RecentSessions     []Entry    `json:"recent_sessions"`
	RecentObservations []Entry    `json:"recent_observations"`
	RecentPrompts      []Entry    `json:"recent_prompts"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s *Service) q(ctx context.Context) *timelinedb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return timelinedb.New(tx)
	}
	return timelinedb.New(s.Pool)
}

func (s *Service) Context(ctx context.Context, orgID, userID, projectID uuid.UUID) (*Snapshot, error) {
	snap := &Snapshot{}
	if projectID != uuid.Nil {
		snap.ProjectID = &projectID
	}

	_ = userID
	snap.ActiveSession = nil
	snap.RecentSessions = nil

	obs, err := s.queryObservations(ctx, orgID, projectID, 10)
	if err != nil {
		return nil, fmt.Errorf("recent observations: %w", err)
	}
	snap.RecentObservations = obs

	prompts, err := s.queryPrompts(ctx, orgID, projectID, 5)
	if err != nil {
		return nil, fmt.Errorf("recent prompts: %w", err)
	}
	snap.RecentPrompts = prompts

	return snap, nil
}

func (s *Service) Timeline(ctx context.Context, orgID, observationID uuid.UUID, before, after int) ([]Entry, error) {
	if before < 0 {
		before = 3
	}
	if after < 0 {
		after = 3
	}
	if before > 50 {
		before = 50
	}
	if after > 50 {
		after = 50
	}

	anchor, err := s.q(ctx).GetAnchorObservation(ctx, observationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrObservationNotFound
		}
		return nil, fmt.Errorf("anchor lookup: %w", err)
	}

	priorObs, err := s.queryEntriesAround(ctx, anchor.ProjectID, anchor.CreatedAt, before, true)
	if err != nil {
		return nil, err
	}

	nextObs, err := s.queryEntriesAround(ctx, anchor.ProjectID, anchor.CreatedAt, after, false)
	if err != nil {
		return nil, err
	}

	anchorEntry := Entry{
		Kind:      KindObservation,
		ID:        observationID,
		Title:     "observation",
		Preview:   truncate(anchor.Content, 200),
		CreatedAt: anchor.CreatedAt,
		ProjectID: &anchor.ProjectID,
	}

	all := append(priorObs, anchorEntry)
	all = append(all, nextObs...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	return all, nil
}

func (s *Service) queryObservations(ctx context.Context, orgID, projectID uuid.UUID, limit int) ([]Entry, error) {
	if projectID == uuid.Nil {
		rows, err := s.q(ctx).ListObservations(ctx, int32(limit))
		if err != nil {
			return nil, err
		}
		out := make([]Entry, 0, len(rows))
		for _, r := range rows {
			out = append(out, Entry{
				Kind:      KindObservation,
				ID:        r.ID,
				Title:     r.ObservationType,
				Preview:   truncate(r.Content, 200),
				CreatedAt: r.CreatedAt,
			})
		}
		return out, nil
	}

	rows, err := s.q(ctx).ListObservationsByProject(ctx, timelinedb.ListObservationsByProjectParams{
		ProjectID: projectID,
		Lim:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		pid := projectID
		out = append(out, Entry{
			Kind:      KindObservation,
			ID:        r.ID,
			Title:     r.ObservationType,
			Preview:   truncate(r.Content, 200),
			CreatedAt: r.CreatedAt,
			ProjectID: &pid,
		})
	}
	return out, nil
}

func (s *Service) queryPrompts(ctx context.Context, orgID, projectID uuid.UUID, limit int) ([]Entry, error) {
	if projectID == uuid.Nil {
		rows, err := s.q(ctx).ListPrompts(ctx, int32(limit))
		if err != nil {
			return nil, err
		}
		out := make([]Entry, 0, len(rows))
		for _, r := range rows {
			out = append(out, Entry{
				Kind:      KindPrompt,
				ID:        r.ID,
				Title:     r.Slug,
				Preview:   truncate(r.Body, 200),
				CreatedAt: r.CreatedAt,
			})
		}
		return out, nil
	}

	rows, err := s.q(ctx).ListPromptsByProject(ctx, timelinedb.ListPromptsByProjectParams{
		ProjectID: &projectID,
		Lim:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		pid := projectID
		out = append(out, Entry{
			Kind:      KindPrompt,
			ID:        r.ID,
			Title:     r.Slug,
			Preview:   truncate(r.Body, 200),
			CreatedAt: r.CreatedAt,
			ProjectID: &pid,
		})
	}
	return out, nil
}

func (s *Service) queryEntriesAround(ctx context.Context, projectID uuid.UUID, ts time.Time, limit int, before bool) ([]Entry, error) {
	if limit == 0 {
		return nil, nil
	}

	if before {
		rows, err := s.q(ctx).ListEntriesBefore(ctx, timelinedb.ListEntriesBeforeParams{
			Lim:       int32(limit),
			ProjectID: projectID,
			Ts:        ts,
		})
		if err != nil {
			return nil, fmt.Errorf("around: %w", err)
		}
		out := make([]Entry, 0, len(rows))
		for _, r := range rows {
			pid := projectID
			out = append(out, Entry{
				Kind:      EntryKind(r.Kind),
				ID:        r.ID,
				Title:     r.ObservationType,
				Preview:   truncate(r.Content, 200),
				CreatedAt: r.CreatedAt,
				ProjectID: &pid,
			})
		}
		return out, nil
	}

	rows, err := s.q(ctx).ListEntriesAfter(ctx, timelinedb.ListEntriesAfterParams{
		Lim:       int32(limit),
		ProjectID: projectID,
		Ts:        ts,
	})
	if err != nil {
		return nil, fmt.Errorf("around: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		pid := projectID
		out = append(out, Entry{
			Kind:      EntryKind(r.Kind),
			ID:        r.ID,
			Title:     r.ObservationType,
			Preview:   truncate(r.Content, 200),
			CreatedAt: r.CreatedAt,
			ProjectID: &pid,
		})
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
