// Package requirement — issue-04.1 requirements CRUD con jerarquía padre-hijo.
//
// Un requirement (REQ) es la unidad de especificación SDD. Puede tener hijos
// (sub-requisitos) formando un árbol. Soft-delete via status = "archived".
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package requirement

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/requirement/requirementdb"
	"nunezlagos/domain/internal/store/txctx"
)

// Status y Priority valores permitidos.
const (
	StatusActive   = "active"
	StatusArchived = "archived"

	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

var (
	ErrNotFound        = errors.New("requirement not found")
	ErrSlugTaken       = errors.New("requirement slug already taken")
	ErrSlugInvalid     = errors.New("slug must match REQ-XX pattern")
	ErrParentNotFound  = errors.New("parent requirement not found")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidPriority = errors.New("invalid priority")

	ErrProjectIDRequired = errors.New("project_id required")
)

var reReqSlug = regexp.MustCompile(`^REQ-\d+(-[a-z0-9-]+)?$`)

var validStatuses = map[string]bool{StatusActive: true, StatusArchived: true}
var validPriorities = map[string]bool{PriorityLow: true, PriorityMedium: true, PriorityHigh: true, PriorityCritical: true}

// Requirement snapshot.
type Requirement struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// RequirementFilter opcional para List.
type RequirementFilter struct {
	Status   string
	Priority string
	ParentID *uuid.UUID // nil = sin filtro
	Limit    int
	Offset   int
}

// RequirementTree nodo del árbol jerárquico.
type RequirementTree struct {
	Requirement
	Children []*RequirementTree `json:"children,omitempty"`
}

// Service CRUD para requirements.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

func (s *Service) q(ctx context.Context) *requirementdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return requirementdb.New(tx)
	}
	return requirementdb.New(s.Pool)
}

// Create inserta un requirement. Si parentSlug no es vacío, busca el padre.
func (s *Service) Create(ctx context.Context, slug, title, description, status, priority string, parentSlug string, projectID *uuid.UUID) (*Requirement, error) {
	if !reReqSlug.MatchString(slug) {
		return nil, ErrSlugInvalid
	}
	if title == "" {
		return nil, errors.New("title required")
	}
	if status == "" {
		status = StatusActive
	}
	if !validStatuses[status] {
		return nil, ErrInvalidStatus
	}
	if priority == "" {
		priority = PriorityMedium
	}
	if !validPriorities[priority] {
		return nil, ErrInvalidPriority
	}

	if projectID == nil || *projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	var parentID *uuid.UUID
	if parentSlug != "" {
		p, err := s.GetBySlug(ctx, parentSlug)
		if err != nil {
			return nil, ErrParentNotFound
		}
		parentID = &p.ID
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	row, err := s.q(ctx).InsertRequirement(ctx, requirementdb.InsertRequirementParams{
		Slug:        slug,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		ParentID:    parentID,
		ProjectID:   *projectID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert requirement: %w", err)
	}
	r := toRequirement(row)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "requirement.created",
			EntityType: "requirement",
			EntityID:   &r.ID,
			NewValues:  map[string]any{"slug": slug, "title": title, "priority": priority},
		})
	}
	return &r, nil
}

// GetBySlug retorna un requirement por slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Requirement, error) {
	row, err := s.q(ctx).GetRequirementBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get requirement: %w", err)
	}
	r := toRequirement(row)
	return &r, nil
}

// GetByID retorna un requirement por ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Requirement, error) {
	row, err := s.q(ctx).GetRequirementByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get requirement: %w", err)
	}
	r := toRequirement(row)
	return &r, nil
}

// List retorna requirements según filter.
func (s *Service) List(ctx context.Context, filter RequirementFilter) ([]Requirement, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	rows, err := s.q(ctx).ListRequirements(ctx, requirementdb.ListRequirementsParams{
		Limit:    int32(filter.Limit),
		Offset:   int32(filter.Offset),
		Status:   optStr(filter.Status),
		Priority: optStr(filter.Priority),
		ParentID: filter.ParentID,
	})
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}

	var out []Requirement
	for _, row := range rows {
		out = append(out, toRequirement(row))
	}
	return out, nil
}

// Update actualiza title, description, status, priority. Campos nil no se modifican.
func (s *Service) Update(ctx context.Context, slug string, title *string, description *string, status *string, priority *string) (*Requirement, error) {
	existing, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	newTitle := existing.Title
	if title != nil {
		newTitle = *title
	}
	newDesc := existing.Description
	if description != nil {
		if *description == "" {
			newDesc = nil
		} else {
			newDesc = description
		}
	}
	newStatus := existing.Status
	if status != nil {
		if !validStatuses[*status] {
			return nil, ErrInvalidStatus
		}
		newStatus = *status
	}
	newPriority := existing.Priority
	if priority != nil {
		if !validPriorities[*priority] {
			return nil, ErrInvalidPriority
		}
		newPriority = *priority
	}

	row, err := s.q(ctx).UpdateRequirement(ctx, requirementdb.UpdateRequirementParams{
		Slug:        slug,
		Title:       newTitle,
		Description: newDesc,
		Status:      newStatus,
		Priority:    newPriority,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update requirement: %w", err)
	}
	updated := toRequirement(row)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "requirement.updated",
			EntityType: "requirement",
			EntityID:   &updated.ID,
			OldValues: map[string]any{
				"title": existing.Title, "status": existing.Status, "priority": existing.Priority,
			},
			NewValues: map[string]any{
				"title": newTitle, "status": newStatus, "priority": newPriority,
			},
		})
	}
	return &updated, nil
}

// Archive cambia status a "archived". Si recursive=true, también archiva hijos.
func (s *Service) Archive(ctx context.Context, slug string, recursive bool) error {
	r, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}

	if recursive {
		err = s.q(ctx).ArchiveRequirementRecursive(ctx, r.ID)
	} else {
		err = s.q(ctx).ArchiveRequirement(ctx, r.ID)
	}
	if err != nil {
		return fmt.Errorf("archive requirement: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "requirement.archived",
			EntityType: "requirement",
			EntityID:   &r.ID,
			OldValues:  map[string]any{"status": r.Status},
			NewValues:  map[string]any{"status": StatusArchived, "recursive": recursive},
		})
	}
	return nil
}

// GetTree retorna el árbol jerárquico desde un slug raíz. Máximo 10 niveles.
func (s *Service) GetTree(ctx context.Context, slug string) (*RequirementTree, error) {
	rows, err := s.q(ctx).GetRequirementTree(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}

	nodes := make([]Requirement, len(rows))
	for i, row := range rows {
		nodes[i] = toRequirementTreeRow(row)
	}

	root := &RequirementTree{Requirement: nodes[0]}
	if len(nodes) == 1 {
		return root, nil
	}

	nodeMap := map[uuid.UUID]*RequirementTree{}
	for _, n := range nodes {
		nodeMap[n.ID] = &RequirementTree{Requirement: n}
	}
	for _, n := range nodes {
		if n.ParentID != nil {
			if parent, ok := nodeMap[*n.ParentID]; ok {
				parent.Children = append(parent.Children, nodeMap[n.ID])
			}
		}
	}
	return root, nil
}

func toRequirement(r requirementdb.SddRequirement) Requirement {
	pid := r.ProjectID
	return Requirement{
		ID:          r.ID,
		Slug:        r.Slug,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		ParentID:    r.ParentID,
		ProjectID:   &pid,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toRequirementTreeRow(r requirementdb.GetRequirementTreeRow) Requirement {
	pid := r.ProjectID
	return Requirement{
		ID:          r.ID,
		Slug:        r.Slug,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		ParentID:    r.ParentID,
		ProjectID:   &pid,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
