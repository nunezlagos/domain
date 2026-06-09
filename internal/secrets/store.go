package secrets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/crypto"
)

var (
	ErrNotFound    = errors.New("secret not found")
	ErrSlugExists  = errors.New("secret slug already exists in organization")
	ErrExpired     = errors.New("secret has expired")
)

type Secret struct {
	ID                  uuid.UUID  `json:"id"`
	OrganizationID      uuid.UUID  `json:"organization_id"`
	Slug                string     `json:"slug"`
	Name                string     `json:"name"`
	EncryptionKeyVer    int        `json:"encryption_key_version"`
	Description         *string    `json:"description,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	RotatedAt           *time.Time `json:"rotated_at,omitempty"`
	CreatedBy           *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
}

type PGStore struct {
	Pool   *pgxpool.Pool
	Cipher *crypto.Cipher
}

type CreateInput struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Value          string
	Description    *string
	ExpiresAt      *time.Time
	CreatedBy      *uuid.UUID
}

type UpdateInput struct {
	Name        *string
	Value       *string
	Description **string
	ExpiresAt   **time.Time
}

const _listSelectCols = `id, organization_id, slug, name, encryption_key_version, description, expires_at, rotated_at, created_by, created_at, updated_at, deleted_at`

func scanSecret(scanner interface {
	Scan(dest ...any) error
}) (Secret, error) {
	var s Secret
	err := scanner.Scan(
		&s.ID, &s.OrganizationID, &s.Slug, &s.Name,
		&s.EncryptionKeyVer, &s.Description, &s.ExpiresAt,
		&s.RotatedAt, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		&s.DeletedAt,
	)
	return s, err
}

func (s *PGStore) Create(ctx context.Context, in CreateInput) (*Secret, error) {
	encVal, err := s.Cipher.Encrypt([]byte(in.Value))
	if err != nil {
		return nil, fmt.Errorf("encrypt value: %w", err)
	}

	row := s.Pool.QueryRow(ctx, `
		INSERT INTO secrets (organization_id, slug, name, encrypted_value, encryption_key_version, description, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+_listSelectCols,
		in.OrganizationID, in.Slug, in.Name, encVal, int(s.Cipher.CurrentVersion()),
		in.Description, in.ExpiresAt, in.CreatedBy,
	)

	secret, err := scanSecret(row)
	if err != nil {
		if pgErr := pgErrUniqueViolation(err); pgErr != "" {
			return nil, ErrSlugExists
		}
		return nil, fmt.Errorf("insert secret: %w", err)
	}
	return &secret, nil
}

func (s *PGStore) GetByID(ctx context.Context, id uuid.UUID) (*Secret, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+_listSelectCols+` FROM secrets WHERE id = $1 AND deleted_at IS NULL`, id)
	secret, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get secret: %w", err)
	}
	return &secret, nil
}

func (s *PGStore) GetValue(ctx context.Context, id uuid.UUID) (string, error) {
	var encVal []byte
	var ver int
	err := s.Pool.QueryRow(ctx, `
		SELECT encrypted_value, encryption_key_version
		FROM secrets WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&encVal, &ver)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get secret value: %w", err)
	}

	plain, err := s.Cipher.Decrypt(encVal)
	if err != nil {
		return "", fmt.Errorf("decrypt secret value: %w", err)
	}
	return string(plain), nil
}

func (s *PGStore) ListByOrg(ctx context.Context, orgID uuid.UUID, includeExpired bool) ([]Secret, error) {
	q := `SELECT ` + _listSelectCols + ` FROM secrets WHERE organization_id = $1 AND deleted_at IS NULL`
	if !includeExpired {
		q += ` AND (expires_at IS NULL OR expires_at > NOW())`
	}
	q += ` ORDER BY name ASC`

	rows, err := s.Pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()

	var result []Secret
	for rows.Next() {
		s, err := scanSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (s *PGStore) GetByOrgAndSlug(ctx context.Context, orgID uuid.UUID, slug string) (*Secret, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+_listSelectCols+` FROM secrets
		WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
	secret, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get secret by slug: %w", err)
	}
	return &secret, nil
}

func (s *PGStore) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Secret, error) {
	set := ""
	args := []any{}
	argN := 1

	if in.Name != nil {
		set += fmt.Sprintf("name = $%d, ", argN)
		args = append(args, *in.Name)
		argN++
	}
	if in.Value != nil {
		encVal, err := s.Cipher.Encrypt([]byte(*in.Value))
		if err != nil {
			return nil, fmt.Errorf("encrypt value: %w", err)
		}
		set += fmt.Sprintf("encrypted_value = $%d, encryption_key_version = $%d, ", argN, argN+1)
		args = append(args, encVal, int(s.Cipher.CurrentVersion()))
		argN += 2
	}
	if in.Description != nil {
		if *in.Description == nil {
			set += fmt.Sprintf("description = NULL, ")
		} else {
			set += fmt.Sprintf("description = $%d, ", argN)
			args = append(args, **in.Description)
			argN++
		}
	}
	if in.ExpiresAt != nil {
		if *in.ExpiresAt == nil {
			set += fmt.Sprintf("expires_at = NULL, ")
		} else {
			set += fmt.Sprintf("expires_at = $%d, ", argN)
			args = append(args, **in.ExpiresAt)
			argN++
		}
	}

	if len(args) == 0 {
		return s.GetByID(ctx, id)
	}

	set = set[:len(set)-2]
	args = append(args, id)

	row := s.Pool.QueryRow(ctx, `
		UPDATE secrets SET `+set+` WHERE id = $`+fmt.Sprintf("%d", argN)+` AND deleted_at IS NULL
		RETURNING `+_listSelectCols,
		args...,
	)

	secret, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update secret: %w", err)
	}
	return &secret, nil
}

func (s *PGStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE secrets SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func pgErrUniqueViolation(err error) string {
	const code = "23505"
	if err == nil {
		return ""
	}
	if len(err.Error()) < 6 {
		return ""
	}
	for i := 0; i < len(err.Error())-5; i++ {
		if err.Error()[i:i+5] == code {
			return code
		}
	}
	return ""
}
