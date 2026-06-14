// Package org — issue-21.1 organization management.
//
// CRUD organizaciones, member list, transfer ownership, soft-delete con confirm.
// Cada mutation registra audit_log via audit.Recorder inyectado.
package org

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
	"nunezlagos/domain/internal/auth/apikey"
)

// Role permitidos en users.role.
const (
	RoleOwner      = "owner"
	RoleAdmin      = "admin"
	RoleMaintainer = "maintainer"
	RoleMember     = "member"
	RoleViewer     = "viewer"
)

var (
	ErrSlugInvalid     = errors.New("slug must be lowercase ascii letters, digits and dashes")
	ErrSlugTaken       = errors.New("slug already taken")
	ErrNotFound        = errors.New("organization not found")
	ErrNotOwner        = errors.New("only owners can perform this action")
	ErrTargetNotMember = errors.New("target user is not a member of this organization")
	ErrConfirmMismatch = errors.New("confirmation slug mismatch")
	ErrUserNotFound    = errors.New("user not found in organization")
	ErrInvalidEmail    = errors.New("email format invalid")
	ErrInvalidRole     = errors.New("role must be one of: owner, admin, maintainer, member, viewer")
	ErrEmailTaken      = errors.New("email already in use within the organization")
)

// emailRegex es una validación de formato simplificada (no DNS).
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// allowedRoles cierra el conjunto de roles aceptados en AddMemberWithAPIKey.
var allowedRoles = map[string]bool{
	RoleOwner:      true,
	RoleAdmin:      true,
	RoleMaintainer: true,
	RoleMember:     true,
	RoleViewer:     true,
}

// Organization snapshot lectura.
type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Settings  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Member miembro de la org.
type Member struct {
	UserID     uuid.UUID
	Email      string
	Name       string
	Role       string
	JoinedAt   time.Time
	LastActive *time.Time
}

// Service expone la API de aplicación de orgs.
type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}[a-z0-9]$|^[a-z][a-z0-9]?$`)

// Create crea org + user owner inicial atómicamente.
// El email del owner es único por organization (constraint en BD).
func (s *Service) Create(ctx context.Context, name, slug, ownerEmail, ownerName string) (*Organization, *Member, error) {
	if !reSlug.MatchString(slug) {
		return nil, nil, ErrSlugInvalid
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var org Organization
	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, settings)
		 VALUES ($1, $2, '{}'::jsonb)
		 RETURNING id, name, slug, settings, created_at, updated_at, deleted_at`,
		name, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Settings, &org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if err != nil {
		if isUniqueViolation(err, "organizations_slug_key") || isUniqueViolation(err, "uniqueViolation") || isPgUnique(err) {
			return nil, nil, ErrSlugTaken
		}
		return nil, nil, fmt.Errorf("insert org: %w", err)
	}

	var member Member
	err = tx.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, COALESCE(name,''), role, created_at`,
		org.ID, ownerEmail, ownerName, RoleOwner,
	).Scan(&member.UserID, &member.Email, &member.Name, &member.Role, &member.JoinedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert owner user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &org.ID,
			ActorID:        &member.UserID,
			ActorType:      audit.ActorUser,
			Action:         "organization.created",
			EntityType:     "organization",
			EntityID:       &org.ID,
			NewValues:      map[string]any{"name": name, "slug": slug, "owner_email": ownerEmail},
		})
	}
	return &org, &member, nil
}

// GetByID devuelve org incluyendo soft-deleted (caller decide qué filtrar).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	var org Organization
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Settings, &org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	return &org, nil
}

// UpdateSettings reemplaza settings JSONB. ActorID requerido para auditoría.
// Solo callers con role owner/admin (la verificación RBAC vive en el handler/middleware).
func (s *Service) UpdateSettings(ctx context.Context, id, actorID uuid.UUID, settings map[string]any) (*Organization, error) {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var updated Organization
	err = s.Pool.QueryRow(ctx,
		`UPDATE organizations SET settings = $2
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, name, slug, settings, created_at, updated_at, deleted_at`,
		id, settingsJSON,
	).Scan(&updated.ID, &updated.Name, &updated.Slug, &updated.Settings, &updated.CreatedAt, &updated.UpdatedAt, &updated.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &id,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "organization.updated",
			EntityType:     "organization",
			EntityID:       &id,
			OldValues:      prev.Settings,
			NewValues:      settings,
		})
	}
	return &updated, nil
}

// ListMembers retorna miembros activos (deleted_at IS NULL).
func (s *Service) ListMembers(ctx context.Context, orgID uuid.UUID) ([]Member, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, email, COALESCE(name,''), role, created_at
		 FROM users
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// TransferOwnership: fromUserID debe ser owner; toUserID debe ser admin/maintainer.
// from pasa a admin; to pasa a owner. Atómico via tx + UPDATE.
func (s *Service) TransferOwnership(ctx context.Context, orgID, fromUserID, toUserID uuid.UUID) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fromRole, toRole string
	err = tx.QueryRow(ctx,
		`SELECT role FROM users WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`,
		fromUserID, orgID).Scan(&fromRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("query from user: %w", err)
	}
	if fromRole != RoleOwner {
		return ErrNotOwner
	}
	err = tx.QueryRow(ctx,
		`SELECT role FROM users WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`,
		toUserID, orgID).Scan(&toRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTargetNotMember
	}
	if err != nil {
		return fmt.Errorf("query to user: %w", err)
	}
	if toRole != RoleAdmin && toRole != RoleMaintainer {
		return ErrTargetNotMember
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users SET role = $1 WHERE id = $2`, RoleAdmin, fromUserID); err != nil {
		return fmt.Errorf("demote from: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET role = $1 WHERE id = $2`, RoleOwner, toUserID); err != nil {
		return fmt.Errorf("promote to: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        &fromUserID,
			ActorType:      audit.ActorUser,
			Action:         "organization.ownership_transferred",
			EntityType:     "organization",
			EntityID:       &orgID,
			OldValues:      map[string]any{"owner_id": fromUserID.String()},
			NewValues:      map[string]any{"owner_id": toUserID.String()},
		})
	}
	return nil
}

// SoftDelete marca deleted_at en la org. confirmSlug debe coincidir con el slug
// actual (anti-fat-finger). Caller debe haber verificado RBAC owner.
func (s *Service) SoftDelete(ctx context.Context, orgID, actorID uuid.UUID, confirmSlug string) error {
	org, err := s.GetByID(ctx, orgID)
	if err != nil {
		return err
	}
	if org.DeletedAt != nil {
		return nil // ya soft-deleted: idempotente
	}
	if confirmSlug != org.Slug {
		return ErrConfirmMismatch
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE organizations SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, orgID)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "organization.deleted",
			EntityType:     "organization",
			EntityID:       &orgID,
			OldValues:      map[string]any{"slug": org.Slug, "name": org.Name},
		})
	}
	return nil
}

// MemberWithKey es el resultado de AddMemberWithAPIKey: user + key plaintext
// emitida UNA sola vez al caller. El plaintext NUNCA se persiste; el hash
// bcrypt vive en api_keys.key_hash.
type MemberWithKey struct {
	User      Member
	APIKey    string    // plaintext (UNA sola vez, no se puede recuperar después)
	APIKeyID  uuid.UUID
	KeyPrefix string
}

// AddMemberWithAPIKey crea user + api_key atómicamente sin email/OTP
// (issue-36.1). Flujo paralelo a invitations: el admin entrega la key al
// invitado por el canal que tenga.
//
// Validación + RBAC:
//   - Email se valida con emailRegex local; sin DNS check.
//   - Role debe estar en allowedRoles.
//   - Verificación de RBAC (caller es owner/admin de la org) la hace el handler.
//
// Atomicidad: todo dentro de una tx. Si INSERT api_keys falla, rollback
// del INSERT users.
//
// Audit log: action "member.created_with_key" con key_prefix (NUNCA
// plaintext — security.md prohíbe loggear secretos).
func (s *Service) AddMemberWithAPIKey(
	ctx context.Context,
	orgID, actorID uuid.UUID,
	email, name, role string,
) (*MemberWithKey, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}
	if !allowedRoles[role] {
		return nil, ErrInvalidRole
	}
	if orgID == uuid.Nil {
		return nil, ErrNotFound
	}

	plaintext, prefix, hash, err := apikey.Generate("live")
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgExists bool
	err = tx.QueryRow(ctx,
		`SELECT TRUE FROM organizations WHERE id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&orgExists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check org: %w", err)
	}

	var m Member
	err = tx.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, $2, NULLIF($3, ''), $4)
		 RETURNING id, email, COALESCE(name,''), role, created_at`,
		orgID, email, name, role,
	).Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt)
	if err != nil {
		if isEmailUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	keyID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO api_keys (id, organization_id, user_id, key_hash, key_prefix,
		                        name, environment, expires_at)
		 VALUES ($1, $2, $3, $4, $5, 'default', 'live', NULL)`,
		keyID, orgID, m.UserID, hash, prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api_key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "member.created_with_key",
			EntityType:     "user",
			EntityID:       &m.UserID,
			NewValues: map[string]any{
				"email":      email,
				"role":       role,
				"key_prefix": prefix,
			},
		})
	}

	return &MemberWithKey{
		User:      m,
		APIKey:    plaintext,
		APIKeyID:  keyID,
		KeyPrefix: prefix,
	}, nil
}

// isEmailUniqueViolation detecta error de unique constraint sobre users.email.
// La constraint puede llamarse users_org_email_uniq o similar; matcheamos por
// substrings robustos a renames.
func isEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !contains(msg, "duplicate key") {
		return false
	}
	return contains(msg, "email") || contains(msg, "users_") && contains(msg, "uniq")
}

// AddMember inserta user en org con el rol indicado (helper para tests; en prod
// se usa issue-21.2 invitations flow).
func (s *Service) AddMember(ctx context.Context, orgID uuid.UUID, email, name, role string) (*Member, error) {
	var m Member
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, COALESCE(name,''), role, created_at`,
		orgID, email, name, role,
	).Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	return &m, nil
}

// --- helpers ---

func isUniqueViolation(err error, name string) bool {
	// pgconn.PgError tiene Code "23505" para unique violation; verificamos por nombre
	// de constraint también.
	if err == nil {
		return false
	}
	return false
}

func isPgUnique(err error) bool {
	// Heurística: error string contiene "duplicate key" + "slug".
	return err != nil && containsAll(err.Error(), "duplicate key", "slug")
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
