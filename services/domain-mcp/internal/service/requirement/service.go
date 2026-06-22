// Package requirement — issue-04.1 requirements CRUD con jerarquía padre-hijo.
//
// Un requirement (REQ) es la unidad de especificación SDD. Puede tener hijos
// (sub-requisitos) formando un árbol. Soft-delete via status = "archived".
package requirement

import (
	"context"
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

	"nunezlagos/domain/internal/audit"
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

	var r Requirement
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO sdd_requirements (slug, title, description, status, priority, parent_id, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, slug, title, description, status, priority, parent_id, project_id, created_at, updated_at`,
		slug, title, desc, status, priority, parentID, projectID,
	).Scan(&r.ID, &r.Slug, &r.Title, &r.Description, &r.Status, &r.Priority, &r.ParentID, &r.ProjectID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert requirement: %w", err)
	}

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
	var r Requirement
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at
		 FROM sdd_requirements WHERE slug = $1`, slug,
	).Scan(&r.ID, &r.Slug, &r.Title, &r.Description, &r.Status, &r.Priority, &r.ParentID, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get requirement: %w", err)
	}
	return &r, nil
}

// GetByID retorna un requirement por ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Requirement, error) {
	var r Requirement
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at
		 FROM sdd_requirements WHERE id = $1`, id,
	).Scan(&r.ID, &r.Slug, &r.Title, &r.Description, &r.Status, &r.Priority, &r.ParentID, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get requirement: %w", err)
	}
	return &r, nil
}

// List retorna requirements según filter.
func (s *Service) List(ctx context.Context, filter RequirementFilter) ([]Requirement, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if filter.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, filter.Status)
		idx++
	}
	if filter.Priority != "" {
		where = append(where, fmt.Sprintf("priority = $%d", idx))
		args = append(args, filter.Priority)
		idx++
	}
	if filter.ParentID != nil {
		where = append(where, fmt.Sprintf("parent_id = $%d", idx))
		args = append(args, *filter.ParentID)
		idx++
	}

	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	q := fmt.Sprintf(`SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at
		 FROM sdd_requirements WHERE %s ORDER BY slug LIMIT $%d OFFSET $%d`,
		strings.Join(where, " AND "), idx, idx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}
	defer rows.Close()

	var out []Requirement
	for rows.Next() {
		var r Requirement
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.Description, &r.Status, &r.Priority, &r.ParentID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
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

	var updated Requirement
	err = s.Pool.QueryRow(ctx,
		`UPDATE sdd_requirements SET title = $2, description = $3, status = $4, priority = $5, updated_at = NOW()
		 WHERE slug = $1
		 RETURNING id, slug, title, description, status, priority, parent_id, created_at, updated_at`,
		slug, newTitle, newDesc, newStatus, newPriority,
	).Scan(&updated.ID, &updated.Slug, &updated.Title, &updated.Description, &updated.Status, &updated.Priority, &updated.ParentID, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update requirement: %w", err)
	}

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
		_, err = s.Pool.Exec(ctx,
			`UPDATE sdd_requirements SET status = 'archived', updated_at = NOW()
			 WHERE id = $1 OR parent_id = $1`, r.ID)
	} else {
		_, err = s.Pool.Exec(ctx,
			`UPDATE sdd_requirements SET status = 'archived', updated_at = NOW()
			 WHERE id = $1`, r.ID)
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
	q := `
WITH RECURSIVE req_tree AS (
    SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at, 0 AS depth
    FROM sdd_requirements WHERE slug = $1
    UNION ALL
    SELECT r.id, r.slug, r.title, r.description, r.status, r.priority, r.parent_id, r.created_at, r.updated_at, rt.depth + 1
    FROM sdd_requirements r
    INNER JOIN req_tree rt ON r.parent_id = rt.id
    WHERE rt.depth < 10
)
SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at, depth
FROM req_tree ORDER BY depth, slug
`
	rows, err := s.Pool.Query(ctx, q, slug)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	defer rows.Close()

	var nodes []struct {
		Requirement
		depth int
	}
	for rows.Next() {
		var n struct {
			Requirement
			depth int
		}
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Status, &n.Priority, &n.ParentID, &n.CreatedAt, &n.UpdatedAt, &n.depth); err != nil {
			return nil, fmt.Errorf("scan tree node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrNotFound
	}

	root := &RequirementTree{Requirement: nodes[0].Requirement}
	if len(nodes) == 1 {
		return root, nil
	}

	// Build tree: map id -> node
	nodeMap := map[uuid.UUID]*RequirementTree{}
	for _, n := range nodes {
		nodeMap[n.ID] = &RequirementTree{Requirement: n.Requirement}
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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
