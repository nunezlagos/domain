package prompt

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

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/prompt/promptdb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrSlugInvalid     = errors.New("slug must be lowercase ascii, digits, dashes (2-100 chars)")
	ErrBodyRequired    = errors.New("body required")
	ErrNotFound        = errors.New("prompt not found")
	ErrVersionExists   = errors.New("version already exists for this slug")
	ErrNoActiveVersion = errors.New("no active version found for slug")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

type Variable struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  any    `json:"default,omitempty"`
}

type Prompt struct {
	ID              uuid.UUID
	ProjectID       *uuid.UUID
	CreatedBy       *uuid.UUID
	Slug            string
	Version         int
	Body            string
	Variables       []Variable
	Description     string
	IsActive        bool
	ParentVersionID *uuid.UUID
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	CreatedBy      *uuid.UUID
	Slug           string
	Body           string
	Variables      []Variable
	Description    string
	Tags           []string

	SetActive bool
}

type SearchResult struct {
	Prompt
	Score    float64
	Headline string
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

func (s *Service) q(ctx context.Context) *promptdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return promptdb.New(tx)
	}
	return promptdb.New(s.Pool)
}

func toPrompt(id uuid.UUID, _ uuid.UUID, projectID *uuid.UUID, createdBy *uuid.UUID, slug string, version int32, body string, variables []byte, description string, isActive bool, parentVersionID *uuid.UUID, tags []string, createdAt time.Time, updatedAt time.Time) Prompt {
	var vars []Variable
	if variables != nil {
		_ = json.Unmarshal(variables, &vars)
	}
	return Prompt{
		ID:              id,
		ProjectID:       projectID,
		CreatedBy:       createdBy,
		Slug:            slug,
		Version:         int(version),
		Body:            body,
		Variables:       vars,
		Description:     description,
		IsActive:        isActive,
		ParentVersionID: parentVersionID,
		Tags:            tags,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
}

func toPromptFromGetByID(r promptdb.GetByIDRow) Prompt {
	return toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt)
}

func toPromptFromGetActive(r promptdb.GetActiveRow) Prompt {
	return toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt)
}

func toPromptFromGetByIDForUpdate(r promptdb.GetByIDForUpdateRow) Prompt {
	return toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt)
}

func toPromptFromInsert(r promptdb.InsertPromptRow) Prompt {
	return toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt)
}

func toPromptFromList(r promptdb.ListVersionsRow) Prompt {
	return toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Prompt, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if strings.TrimSpace(in.Body) == "" {
		return nil, ErrBodyRequired
	}
	if in.Variables == nil {
		in.Variables = []Variable{}
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	varsJSON, _ := json.Marshal(in.Variables)

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := promptdb.New(tx)

	nextVersion, err := q.NextVersion(ctx, promptdb.NextVersionParams{
		Slug:      in.Slug,
		ProjectID: in.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("calc version: %w", err)
	}

	if in.SetActive {
		err = q.DeactivatePriorVersions(ctx, promptdb.DeactivatePriorVersionsParams{
			Slug:      in.Slug,
			ProjectID: in.ProjectID,
		})
		if err != nil {
			return nil, fmt.Errorf("deactivate prior versions: %w", err)
		}
	}

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}

	pRow, err := q.InsertPrompt(ctx, promptdb.InsertPromptParams{
		ProjectID:   in.ProjectID,
		CreatedBy:   in.CreatedBy,
		Slug:        in.Slug,
		Version:     nextVersion,
		Body:        in.Body,
		Variables:   varsJSON,
		Description: desc,
		IsActive:    in.SetActive,
		Tags:        in.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("insert prompt: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	p := toPromptFromInsert(pRow)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "prompt.created",
			EntityType:     "prompt",
			EntityID:       &p.ID,
			NewValues:      map[string]any{"slug": p.Slug, "version": p.Version, "is_active": p.IsActive},
		})
	}
	return &p, nil
}

func (s *Service) SetActive(ctx context.Context, id, actorID uuid.UUID) (*Prompt, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := promptdb.New(tx)

	pRow, err := q.GetByIDForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	err = q.DeactivateOthers(ctx, promptdb.DeactivateOthersParams{
		Slug:      pRow.Slug,
		ProjectID: pRow.ProjectID,
		ID:        id,
	})
	if err != nil {
		return nil, fmt.Errorf("deactivate siblings: %w", err)
	}

	err = q.ActivatePrompt(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	p := toPromptFromGetByIDForUpdate(pRow)
	p.IsActive = true

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "prompt.set_active",
			EntityType: "prompt",
			EntityID:   &id,
		})
	}
	return &p, nil
}

func (s *Service) GetActive(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, slug string) (*Prompt, error) {
	pRow, err := s.q(ctx).GetActive(ctx, promptdb.GetActiveParams{
		Slug:      slug,
		ProjectID: projectID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoActiveVersion
	}
	if err != nil {
		return nil, err
	}
	p := toPromptFromGetActive(pRow)
	return &p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Prompt, error) {
	pRow, err := s.q(ctx).GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p := toPromptFromGetByID(pRow)
	return &p, nil
}

func (s *Service) ListVersions(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, slug string) ([]Prompt, error) {
	rows, err := s.q(ctx).ListVersions(ctx, promptdb.ListVersionsParams{
		Slug:      slug,
		ProjectID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	out := make([]Prompt, len(rows))
	for i, r := range rows {
		out[i] = toPromptFromList(r)
	}
	return out, nil
}

func (s *Service) Search(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.q(ctx).SearchPrompts(ctx, promptdb.SearchPromptsParams{
		Query:       query,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	out := make([]SearchResult, len(rows))
	for i, r := range rows {
		out[i] = SearchResult{
			Prompt:   toPrompt(r.ID, r.OrganizationID, r.ProjectID, r.CreatedBy, r.Slug, r.Version, r.Body, r.Variables, r.Description, r.IsActive, r.ParentVersionID, r.Tags, r.CreatedAt, r.UpdatedAt),
			Score:    r.Score,
			Headline: r.Headline,
		}
	}
	return out, nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	n, err := s.q(ctx).SoftDeletePrompt(ctx, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "prompt.deleted",
			EntityType: "prompt",
			EntityID:   &id,
		})
	}
	return nil
}
