package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/cron/cronsdb"
)

var (
	ErrSlugInvalid       = errors.New("slug must be lowercase ascii, digits, dashes")
	ErrSlugTaken         = errors.New("slug already taken in this organization")
	ErrInvalidCronExpr   = errors.New("invalid cron expression")
	ErrInvalidTargetType = errors.New("target_type must be flow|agent|skill")
	ErrInvalidTimezone   = errors.New("invalid timezone")
	ErrNotFound          = errors.New("cron not found")
)

var (
	reSlug      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)
	cronParser  = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	validTypes  = map[string]bool{"flow": true, "agent": true, "skill": true}
)

type Cron struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CreatedBy      *uuid.UUID
	Slug           string
	Name           string
	Description    string
	CronExpression string
	Timezone       string
	TargetType     string
	TargetID       uuid.UUID
	Inputs         map[string]any
	Enabled        bool
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	CreatedBy      *uuid.UUID
	Slug           string
	Name           string
	Description    string
	CronExpression string
	Timezone       string
	TargetType     string
	TargetID       uuid.UUID
	Inputs         map[string]any
	Enabled        bool
	ActorID        uuid.UUID
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

func NextRun(expression, timezone string, from time.Time) (time.Time, error) {
	sched, err := cronParser.Parse(expression)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidCronExpr, err)
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidTimezone, err)
	}
	return sched.Next(from.In(loc)), nil
}

func toCron(id uuid.UUID, organizationID uuid.UUID, createdBy *uuid.UUID, slug, name, description string, cronExpression, timezone, targetType string, targetID uuid.UUID, inputs []byte, enabled bool, lastRunAt, nextRunAt *time.Time, createdAt, updatedAt time.Time) Cron {
	var in map[string]any
	if inputs != nil {
		_ = json.Unmarshal(inputs, &in)
	}
	return Cron{
		ID:             id,
		OrganizationID: organizationID,
		CreatedBy:      createdBy,
		Slug:           slug,
		Name:           name,
		Description:    description,
		CronExpression: cronExpression,
		Timezone:       timezone,
		TargetType:     targetType,
		TargetID:       targetID,
		Inputs:         in,
		Enabled:        enabled,
		LastRunAt:      lastRunAt,
		NextRunAt:      nextRunAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

func toCronFromInsert(r cronsdb.InsertCronRow) Cron {
	return toCron(r.ID, r.OrganizationID, r.CreatedBy, r.Slug, r.Name, r.Description, r.CronExpression, r.Timezone, r.TargetType, r.TargetID, r.Inputs, r.Enabled, r.LastRunAt, r.NextRunAt, r.CreatedAt, r.UpdatedAt)
}

func toCronFromGet(r cronsdb.GetCronByIDRow) Cron {
	return toCron(r.ID, r.OrganizationID, r.CreatedBy, r.Slug, r.Name, r.Description, r.CronExpression, r.Timezone, r.TargetType, r.TargetID, r.Inputs, r.Enabled, r.LastRunAt, r.NextRunAt, r.CreatedAt, r.UpdatedAt)
}

func toCronFromList(r cronsdb.ListCronsRow) Cron {
	return toCron(r.ID, r.OrganizationID, r.CreatedBy, r.Slug, r.Name, r.Description, r.CronExpression, r.Timezone, r.TargetType, r.TargetID, r.Inputs, r.Enabled, r.LastRunAt, r.NextRunAt, r.CreatedAt, r.UpdatedAt)
}

func toCronFromPick(r cronsdb.PickDueCronsRow) Cron {
	return toCron(r.ID, r.OrganizationID, r.CreatedBy, r.Slug, r.Name, r.Description, r.CronExpression, r.Timezone, r.TargetType, r.TargetID, r.Inputs, r.Enabled, r.LastRunAt, r.NextRunAt, r.CreatedAt, r.UpdatedAt)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Cron, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("name required")
	}
	if !validTypes[in.TargetType] {
		return nil, ErrInvalidTargetType
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}

	next, err := NextRun(in.CronExpression, in.Timezone, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if in.Inputs == nil {
		in.Inputs = map[string]any{}
	}
	inputsJSON, _ := json.Marshal(in.Inputs)

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}

	q := cronsdb.New(s.Pool)
	cRow, err := q.InsertCron(ctx, cronsdb.InsertCronParams{
		CreatedBy:      in.CreatedBy,
		Slug:           in.Slug,
		Name:           in.Name,
		Description:    desc,
		CronExpression: in.CronExpression,
		Timezone:       in.Timezone,
		TargetType:     in.TargetType,
		TargetID:       in.TargetID,
		Inputs:         inputsJSON,
		Enabled:        in.Enabled,
		NextRunAt:      &next,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert cron: %w", err)
	}

	c := toCronFromInsert(cRow)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "cron.created",
			EntityType:     "cron",
			EntityID:       &c.ID,
			NewValues: map[string]any{
				"slug": c.Slug, "expression": c.CronExpression,
				"target_type": c.TargetType,
			},
		})
	}
	return &c, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Cron, error) {
	rows, err := cronsdb.New(s.Pool).ListCrons(ctx, 200)
	if err != nil {
		return nil, fmt.Errorf("list crons: %w", err)
	}
	out := make([]Cron, len(rows))
	for i, r := range rows {
		out[i] = toCronFromList(r)
	}
	return out, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Cron, error) {
	cRow, err := cronsdb.New(s.Pool).GetCronByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cron: %w", err)
	}
	c := toCronFromGet(cRow)
	return &c, nil
}

func (s *Service) PickDue(ctx context.Context, limit int) ([]Cron, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := cronsdb.New(tx)

	rows, err := q.PickDueCrons(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("pick due: %w", err)
	}

	out := make([]Cron, len(rows))
	for i, r := range rows {
		out[i] = toCronFromPick(r)
	}

	now := time.Now().UTC()
	for i := range out {
		next, _ := NextRun(out[i].CronExpression, out[i].Timezone, now)
		err := q.UpdateCronRun(ctx, cronsdb.UpdateCronRunParams{
			LastRunAt: &now,
			NextRunAt: &next,
			ID:        out[i].ID,
		})
		if err != nil {
			return nil, fmt.Errorf("update cron run %s: %w", out[i].ID, err)
		}
		nowCopy, nextCopy := now, next
		out[i].LastRunAt = &nowCopy
		out[i].NextRunAt = &nextCopy
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit pick: %w", err)
	}
	return out, nil
}

func (s *Service) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	n, err := cronsdb.New(s.Pool).SetCronEnabled(ctx, cronsdb.SetCronEnabledParams{
		Enabled: enabled,
		ID:      id,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	n, err := cronsdb.New(s.Pool).SoftDeleteCron(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID: &actorID, ActorType: audit.ActorUser,
			Action: "cron.deleted", EntityType: "cron", EntityID: &id,
		})
	}
	return nil
}
