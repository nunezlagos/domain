// Package role — issue-02.8 custom roles CRUD.
//
// Cada organización puede definir roles custom con matriz de permisos JSONB
// validada contra AllowedResources. Roles built-in son inmutables.
package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/rbac"
)

var (
	ErrNotFound      = errors.New("role not found")
	ErrSlugTaken     = errors.New("role slug already taken in this organization")
	ErrBuiltinRole   = errors.New("built-in roles are immutable")
	ErrHasMembers    = errors.New("cannot delete role: members assigned; reassign first")
	ErrOrgRequired   = errors.New("organization_id required")
	ErrSlugRequired  = errors.New("slug required")
	ErrNameRequired  = errors.New("name required")
	ErrPermsRequired = errors.New("permissions required")
)

// CustomRole snapshot de un rol custom.
type CustomRole struct {
	ID            uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID             `json:"organization_id"`
	Slug          string                 `json:"slug"`
	Name          string                 `json:"name"`
	Permissions   map[string]interface{} `json:"permissions"`
	Description   *string                `json:"description,omitempty"`
	CreatedBy     *uuid.UUID             `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Service CRUD para custom_roles.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

// CreateRole inserta un custom role validado. ActorID opcional para audit.
func (s *Service) CreateRole(ctx context.Context, orgID, actorID uuid.UUID, slug, name, description string, rawPerms map[string]interface{}) (*CustomRole, error) {
	if slug == "" {
		return nil, ErrSlugRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(rawPerms) == 0 {
		return nil, ErrPermsRequired
	}

	perms, err := toResourceActionMap(rawPerms)
	if err != nil {
		return nil, fmt.Errorf("parse permissions: %w", err)
	}
	if err := rbac.ValidatePermissions(perms); err != nil {
		return nil, err
	}

	if rbac.IsBuiltin(rbac.Role(slug)) {
		return nil, ErrBuiltinRole
	}

	permsJSON, _ := json.Marshal(rawPerms)
	var desc *string
	if description != "" {
		desc = &description
	}
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}

	var role CustomRole
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO custom_roles (organization_id, slug, name, permissions, description, created_by)
		 VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		 RETURNING id, organization_id, slug, name, permissions, description, created_by, created_at, updated_at`,
		orgID, slug, name, string(permsJSON), desc, actor,
	).Scan(&role.ID, &role.OrganizationID, &role.Slug, &role.Name, &role.Permissions,
		&role.Description, &role.CreatedBy, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert custom_role: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &orgID,
			ActorID:        actor,
			ActorType:      audit.ActorUser,
			Action:         "role.created",
			EntityType:     "custom_role",
			EntityID:       &role.ID,
			NewValues:      map[string]any{"slug": slug, "name": name, "permissions": rawPerms},
		})
	}
	return &role, nil
}

// ListRoles retorna todos los roles (custom + built-in) de una org.
// built-in no viven en DB, se agregan acá para completitud.
func (s *Service) ListRoles(ctx context.Context, orgID uuid.UUID) ([]CustomRole, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, slug, name, permissions, description, created_by, created_at, updated_at
		 FROM custom_roles WHERE organization_id = $1
		 ORDER BY slug`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query custom_roles: %w", err)
	}
	defer rows.Close()

	var out []CustomRole
	for rows.Next() {
		var r CustomRole
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.Slug, &r.Name, &r.Permissions,
			&r.Description, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan custom_role: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRoleBySlug retorna un custom role por slug.
func (s *Service) GetRoleBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*CustomRole, error) {
	var r CustomRole
	err := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, slug, name, permissions, description, created_by, created_at, updated_at
		 FROM custom_roles WHERE organization_id = $1 AND slug = $2`,
		orgID, slug,
	).Scan(&r.ID, &r.OrganizationID, &r.Slug, &r.Name, &r.Permissions,
		&r.Description, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get custom_role: %w", err)
	}
	return &r, nil
}

// UpdateRole actualiza name/permissions/description de un custom role.
// ActorID opcional. Re-valida permissions contra whitelist.
func (s *Service) UpdateRole(ctx context.Context, orgID, actorID uuid.UUID, slug string, name *string, permissions map[string]interface{}, description *string) (*CustomRole, error) {
	if rbac.IsBuiltin(rbac.Role(slug)) {
		return nil, ErrBuiltinRole
	}

	existing, err := s.GetRoleBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}

	if permissions != nil {
		perms, err := toResourceActionMap(permissions)
		if err != nil {
			return nil, fmt.Errorf("parse permissions: %w", err)
		}
		if err := rbac.ValidatePermissions(perms); err != nil {
			return nil, err
		}
	}

	newName := existing.Name
	if name != nil {
		newName = *name
	}
	newDesc := existing.Description
	if description != nil {
		if *description == "" {
			newDesc = nil
		} else {
			newDesc = description
		}
	}
	newPerms := existing.Permissions
	if permissions != nil {
		newPerms = permissions
	}

	permsJSON, _ := json.Marshal(newPerms)
	var updated CustomRole
	err = s.Pool.QueryRow(ctx,
		`UPDATE custom_roles SET name = $3, permissions = $4::jsonb, description = $5, updated_at = NOW()
		 WHERE organization_id = $1 AND slug = $2
		 RETURNING id, organization_id, slug, name, permissions, description, created_by, created_at, updated_at`,
		orgID, slug, newName, string(permsJSON), newDesc,
	).Scan(&updated.ID, &updated.OrganizationID, &updated.Slug, &updated.Name, &updated.Permissions,
		&updated.Description, &updated.CreatedBy, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update custom_role: %w", err)
	}

	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &orgID,
			ActorID:        actor,
			ActorType:      audit.ActorUser,
			Action:         "role.updated",
			EntityType:     "custom_role",
			EntityID:       &updated.ID,
			OldValues:      map[string]any{"name": existing.Name, "slug": existing.Slug, "permissions": existing.Permissions},
			NewValues:      map[string]any{"name": newName, "slug": slug, "permissions": newPerms},
		})
	}
	return &updated, nil
}

// DeleteRole elimina un custom role. Rechaza si hay users asignados (409).
func (s *Service) DeleteRole(ctx context.Context, orgID, actorID uuid.UUID, slug string) error {
	if rbac.IsBuiltin(rbac.Role(slug)) {
		return ErrBuiltinRole
	}

	var count int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE organization_id = $1 AND role = $2 AND deleted_at IS NULL`,
		orgID, slug,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("count assigned members: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %d members assigned", ErrHasMembers, count)
	}

	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM custom_roles WHERE organization_id = $1 AND slug = $2`,
		orgID, slug)
	if err != nil {
		return fmt.Errorf("delete custom_role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &orgID,
			ActorID:        actor,
			ActorType:      audit.ActorUser,
			Action:         "role.deleted",
			EntityType:     "custom_role",
			EntityID:       nil,
			OldValues:      map[string]any{"slug": slug},
		})
	}
	return nil
}

// AssignRole cambia el role de un user en la org.
func (s *Service) AssignRole(ctx context.Context, orgID, actorID, targetUserID uuid.UUID, roleSlug string) error {
	if roleSlug == "" {
		return ErrSlugRequired
	}

	if !rbac.IsBuiltin(rbac.Role(roleSlug)) {
		_, err := s.GetRoleBySlug(ctx, orgID, roleSlug)
		if err != nil {
			return fmt.Errorf("custom role not found: %w", err)
		}
	}

	tag, err := s.Pool.Exec(ctx,
		`UPDATE users SET role = $3 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, targetUserID, roleSlug)
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &orgID,
			ActorID:        actor,
			ActorType:      audit.ActorUser,
			Action:         "role.assigned",
			EntityType:     "user",
			EntityID:       &targetUserID,
			NewValues:      map[string]any{"role": roleSlug},
		})
	}
	return nil
}

// toResourceActionMap convierte map[string]interface{} a map[Resource][]Action para validator.
func toResourceActionMap(raw map[string]interface{}) (map[rbac.Resource][]rbac.Action, error) {
	out := make(map[rbac.Resource][]rbac.Action, len(raw))
	for resStr, v := range raw {
		res := rbac.Resource(resStr)
		items, ok := v.([]interface{})
		if !ok {
			return nil, fmt.Errorf("permissions.%s: expected array of strings", resStr)
		}
		actions := make([]rbac.Action, 0, len(items))
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("permissions.%s: non-string action", resStr)
			}
			actions = append(actions, rbac.Action(s))
		}
		out[res] = actions
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
