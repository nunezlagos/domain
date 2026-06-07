// Package cron — HU-10.1 cron schedules service (CRUD).
//
// Tabla crons: cron_expression interpretado con robfig/cron v3 (standard
// 5-field syntax + extensión @every / @daily). target_type → flow/agent/skill.
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"

	"nunezlagos/domain/internal/audit"
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

// NextRun calcula el próximo trigger time según expression + tz.
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

	// Validar expression + calc próximo
	next, err := NextRun(in.CronExpression, in.Timezone, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if in.Inputs == nil {
		in.Inputs = map[string]any{}
	}
	inputsJSON, _ := json.Marshal(in.Inputs)

	var c Cron
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO crons
		   (organization_id, created_by, slug, name, description, cron_expression,
		    timezone, target_type, target_id, inputs, enabled, next_run_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, organization_id, created_by, slug, name, COALESCE(description,''),
		           cron_expression, timezone, target_type, target_id, inputs, enabled,
		           last_run_at, next_run_at, created_at, updated_at`,
		in.OrganizationID, in.CreatedBy, in.Slug, in.Name, nullStr(in.Description),
		in.CronExpression, in.Timezone, in.TargetType, in.TargetID,
		inputsJSON, in.Enabled, next,
	).Scan(&c.ID, &c.OrganizationID, &c.CreatedBy, &c.Slug, &c.Name, &c.Description,
		&c.CronExpression, &c.Timezone, &c.TargetType, &c.TargetID, &c.Inputs, &c.Enabled,
		&c.LastRunAt, &c.NextRunAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "crons_organization_id_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert cron: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
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
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, created_by, slug, name, COALESCE(description,''),
		        cron_expression, timezone, target_type, target_id, inputs, enabled,
		        last_run_at, next_run_at, created_at, updated_at
		 FROM crons WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 200`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list crons: %w", err)
	}
	defer rows.Close()
	var out []Cron
	for rows.Next() {
		var c Cron
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.CreatedBy, &c.Slug, &c.Name, &c.Description,
			&c.CronExpression, &c.Timezone, &c.TargetType, &c.TargetID, &c.Inputs, &c.Enabled,
			&c.LastRunAt, &c.NextRunAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Cron, error) {
	var c Cron
	err := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, created_by, slug, name, COALESCE(description,''),
		        cron_expression, timezone, target_type, target_id, inputs, enabled,
		        last_run_at, next_run_at, created_at, updated_at
		 FROM crons WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&c.ID, &c.OrganizationID, &c.CreatedBy, &c.Slug, &c.Name, &c.Description,
		&c.CronExpression, &c.Timezone, &c.TargetType, &c.TargetID, &c.Inputs, &c.Enabled,
		&c.LastRunAt, &c.NextRunAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cron: %w", err)
	}
	return &c, nil
}

// PickDue marca y devuelve los crons due (next_run_at <= NOW) usando
// SELECT ... FOR UPDATE SKIP LOCKED para safety multi-worker. Devuelve los
// IDs claimed por este worker; caller debe ejecutar el target y llamar
// MarkRan() al terminar.
func (s *Service) PickDue(ctx context.Context, limit int) ([]Cron, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`SELECT id, organization_id, created_by, slug, name, COALESCE(description,''),
		        cron_expression, timezone, target_type, target_id, inputs, enabled,
		        last_run_at, next_run_at, created_at, updated_at
		 FROM crons
		 WHERE enabled = true AND deleted_at IS NULL
		   AND next_run_at IS NOT NULL AND next_run_at <= NOW()
		 ORDER BY next_run_at ASC LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("pick due: %w", err)
	}
	var out []Cron
	for rows.Next() {
		var c Cron
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.CreatedBy, &c.Slug, &c.Name, &c.Description,
			&c.CronExpression, &c.Timezone, &c.TargetType, &c.TargetID, &c.Inputs, &c.Enabled,
			&c.LastRunAt, &c.NextRunAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Avanzar next_run_at para todos los claimed para que otro worker no los
	// reescoja inmediatamente. La ejecución real ocurre fuera de esta tx;
	// si falla el caller llama MarkRan(success=false) y schedule sigue normal.
	now := time.Now().UTC()
	for _, c := range out {
		next, _ := NextRun(c.CronExpression, c.Timezone, now)
		_, _ = tx.Exec(ctx,
			`UPDATE crons SET last_run_at = $1, next_run_at = $2 WHERE id = $3`,
			now, next, c.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit pick: %w", err)
	}
	return out, nil
}

// SetEnabled toggle.
func (s *Service) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE crons SET enabled = $1 WHERE id = $2 AND deleted_at IS NULL`,
		enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE crons SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			ActorID: &actorID, ActorType: audit.ActorUser,
			Action: "cron.deleted", EntityType: "cron", EntityID: &id,
		})
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
