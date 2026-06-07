// Package invite — HU-21.2 invitaciones a organization.
//
// Token-based, email-bound, expira en 7 días. Estados: pending → accepted/
// declined/expired/revoked. Email mismatch al aceptar rechaza. UNIQUE constraint
// (org, email) WHERE status='pending' impide dobles invitaciones simultáneas.
package invite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saargo/domain/internal/audit"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusDeclined Status = "declined"
	StatusExpired  Status = "expired"
	StatusRevoked  Status = "revoked"
)

var (
	ErrNotFound          = errors.New("invitation not found")
	ErrAlreadyPending    = errors.New("invitation already pending for this email")
	ErrInvalidRole       = errors.New("invalid role for invitation")
	ErrExpired           = errors.New("invitation expired")
	ErrEmailMismatch     = errors.New("email mismatch")
	ErrNotPending        = errors.New("invitation no longer pending")
	ErrInvalidIdentifier = errors.New("invalid identifier")
)

var allowedRoles = map[string]bool{
	"admin": true, "maintainer": true, "member": true, "viewer": true,
}

// Invitation snapshot.
type Invitation struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	InvitedByUserID  uuid.UUID
	Email            string
	Role             string
	Token            uuid.UUID
	Status           Status
	ExpiresAt        time.Time
	AcceptedUserID   *uuid.UUID
	CreatedAt        time.Time
}

// Mailer envía emails (HU-20.2 implementa SMTP). Aquí es una abstracción.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type NopMailer struct{}

func (NopMailer) Send(ctx context.Context, to, subject, body string) error { return nil }

// Service expone API de invitaciones.
type Service struct {
	Pool       *pgxpool.Pool
	Audit      audit.Recorder
	Mailer     Mailer
	AcceptURL  string // base URL del frontend de aceptación
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// Create crea invite + envía email. role debe ser válido.
func (s *Service) Create(ctx context.Context, orgID, invitedByUserID uuid.UUID, email, role string) (*Invitation, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidIdentifier
	}
	if !allowedRoles[role] {
		return nil, ErrInvalidRole
	}

	var inv Invitation
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO invitations (organization_id, invited_by_user_id, email, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, organization_id, invited_by_user_id, email, role, token,
		           status, expires_at, accepted_user_id, created_at`,
		orgID, invitedByUserID, email, role,
	).Scan(&inv.ID, &inv.OrganizationID, &inv.InvitedByUserID, &inv.Email, &inv.Role,
		&inv.Token, &inv.Status, &inv.ExpiresAt, &inv.AcceptedUserID, &inv.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "invitations_org_email_pending_uniq") {
			return nil, ErrAlreadyPending
		}
		return nil, fmt.Errorf("insert invitation: %w", err)
	}

	if s.Mailer != nil {
		link := s.AcceptURL + "?token=" + inv.Token.String()
		body := fmt.Sprintf("Te han invitado a unirte. Aceptá: %s\nVence: %s",
			link, inv.ExpiresAt.Format(time.RFC3339))
		_ = s.Mailer.Send(ctx, email, "Invitación a Domain", body)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        &invitedByUserID,
			ActorType:      audit.ActorUser,
			Action:         "invitation.sent",
			EntityType:     "invitation",
			EntityID:       &inv.ID,
			NewValues:      map[string]any{"email": email, "role": role},
		})
	}
	return &inv, nil
}

// GetByToken devuelve la invitación si existe (cualquier status). Útil para
// frontend que muestra "esta invitación venció" antes del intento.
func (s *Service) GetByToken(ctx context.Context, token uuid.UUID) (*Invitation, error) {
	return s.queryOne(ctx, `WHERE token = $1`, token)
}

// Accept verifica token + status pending + no expirada + email match, marca
// accepted, crea el user en la org con el role del invite, retorna user.
func (s *Service) Accept(ctx context.Context, token uuid.UUID, authedEmail, userName string) (newUserID uuid.UUID, orgID uuid.UUID, role string, err error) {
	authedEmail = strings.TrimSpace(strings.ToLower(authedEmail))

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inv Invitation
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, invited_by_user_id, email, role, token,
		        status, expires_at, accepted_user_id, created_at
		 FROM invitations WHERE token = $1 FOR UPDATE`, token,
	).Scan(&inv.ID, &inv.OrganizationID, &inv.InvitedByUserID, &inv.Email, &inv.Role,
		&inv.Token, &inv.Status, &inv.ExpiresAt, &inv.AcceptedUserID, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, "", ErrNotFound
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("query invitation: %w", err)
	}
	if inv.Status != StatusPending {
		return uuid.Nil, uuid.Nil, "", ErrNotPending
	}
	if !s.now().Before(inv.ExpiresAt) {
		// expirada: marcar como tal
		_, _ = tx.Exec(ctx, `UPDATE invitations SET status = 'expired' WHERE id = $1`, inv.ID)
		_ = tx.Commit(ctx)
		return uuid.Nil, uuid.Nil, "", ErrExpired
	}
	if inv.Email != authedEmail {
		return uuid.Nil, uuid.Nil, "", ErrEmailMismatch
	}

	// Crear user en la org. Si ya existe (por algún flow paralelo), update role.
	var userID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (organization_id, email) DO UPDATE SET role = EXCLUDED.role
		 RETURNING id`,
		inv.OrganizationID, inv.Email, userName, inv.Role,
	).Scan(&userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("upsert user: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE invitations SET status = 'accepted', accepted_user_id = $1 WHERE id = $2`,
		userID, inv.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("mark accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &inv.OrganizationID,
			ActorID:        &userID,
			ActorType:      audit.ActorUser,
			Action:         "invitation.accepted",
			EntityType:     "invitation",
			EntityID:       &inv.ID,
			NewValues:      map[string]any{"user_id": userID.String()},
		})
	}
	return userID, inv.OrganizationID, inv.Role, nil
}

// Decline marca declined sin crear user.
func (s *Service) Decline(ctx context.Context, token uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE invitations SET status = 'declined'
		 WHERE token = $1 AND status = 'pending'`, token)
	if err != nil {
		return fmt.Errorf("decline: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotPending
	}
	return nil
}

// Revoke marca revoked (admin envió por error o quiere cancelar).
func (s *Service) Revoke(ctx context.Context, inviteID, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE invitations SET status = 'revoked'
		 WHERE id = $1 AND status = 'pending'`, inviteID)
	if err != nil {
		return fmt.Errorf("revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotPending
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "invitation.revoked",
			EntityType: "invitation",
			EntityID:   &inviteID,
		})
	}
	return nil
}

// ExpireOverdue marca como 'expired' las pending vencidas (job diario).
// Devuelve cuántas se actualizaron.
func (s *Service) ExpireOverdue(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE invitations SET status = 'expired'
		 WHERE status = 'pending' AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("expire overdue: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ListByOrg invitaciones de la org (todos status).
func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Invitation, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, invited_by_user_id, email, role, token,
		        status, expires_at, accepted_user_id, created_at
		 FROM invitations
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.OrganizationID, &inv.InvitedByUserID, &inv.Email, &inv.Role,
			&inv.Token, &inv.Status, &inv.ExpiresAt, &inv.AcceptedUserID, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Invitation, error) {
	var inv Invitation
	q := `SELECT id, organization_id, invited_by_user_id, email, role, token,
	        status, expires_at, accepted_user_id, created_at FROM invitations ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&inv.ID, &inv.OrganizationID, &inv.InvitedByUserID, &inv.Email, &inv.Role,
		&inv.Token, &inv.Status, &inv.ExpiresAt, &inv.AcceptedUserID, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &inv, nil
}
