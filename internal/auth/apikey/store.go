// Store/Resolver Postgres para API keys.
//
// Issue: emite key plaintext (devuelta UNA vez), persiste hash bcrypt + prefix.
// Resolve: lookup por prefix (cheap indexed) + verify bcrypt (costoso pero acotado).

package apikey

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("api key not found")

// PGStore Postgres-backed.
type PGStore struct {
	Pool *pgxpool.Pool
}

// Issue genera nueva key para user/org, persiste hash + prefix, retorna plaintext.
// Plaintext SOLO disponible aquí; subsequente lookup retorna hash, no plaintext.
func (s *PGStore) Issue(ctx context.Context, orgID, userID uuid.UUID, name, env string) (plaintext string, keyID uuid.UUID, err error) {
	plaintext, prefix, hash, err := Generate(env)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("generate: %w", err)
	}
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO api_keys (organization_id, user_id, key_hash, key_prefix, name)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		orgID, userID, hash, prefix, name,
	).Scan(&keyID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("insert api_key: %w", err)
	}
	return plaintext, keyID, nil
}

// APIKeyInfo representación pública de una API key (sin hash ni plaintext).
type APIKeyInfo struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"organization_id"`
	UserID      uuid.UUID  `json:"user_id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// List retorna keys activas de una org (no revoked).
func (s *PGStore) List(ctx context.Context, orgID uuid.UUID) ([]APIKeyInfo, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, organization_id, user_id, name, key_prefix,
		       last_used_at, expires_at, revoked_at, created_at
		FROM api_keys
		WHERE organization_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var keys []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		if err := rows.Scan(&k.ID, &k.OrgID, &k.UserID, &k.Name, &k.Prefix,
			&k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// Revoke marca soft-delete sobre la key.
func (s *PGStore) Revoke(ctx context.Context, keyID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, keyID)
	if err != nil {
		return fmt.Errorf("revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Resolve lookup por prefix (indexed), verifica bcrypt, retorna principal.
// Implementa apikey.Resolver para Middleware.
func (s *PGStore) Resolve(ctx context.Context, plaintext string) (*Principal, error) {
	prefix, err := ParsePrefix(plaintext)
	if err != nil {
		return nil, ErrNotFound
	}

	// Buscar candidates por prefix (puede haber colision; prefix incluye 7 chars random)
	rows, err := s.Pool.Query(ctx,
		`SELECT k.id, k.organization_id, k.user_id, k.key_hash, COALESCE(u.role,'viewer')
		 FROM api_keys k
		 JOIN users u ON u.id = k.user_id
		 WHERE k.key_prefix = $1
		   AND k.revoked_at IS NULL
		   AND (k.expires_at IS NULL OR k.expires_at > NOW())
		   AND u.deleted_at IS NULL`,
		prefix)
	if err != nil {
		return nil, fmt.Errorf("query api_keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id     uuid.UUID
			orgID  uuid.UUID
			userID uuid.UUID
			hash   []byte
			role   string
		)
		if err := rows.Scan(&id, &orgID, &userID, &hash, &role); err != nil {
			return nil, err
		}
		if Verify(plaintext, hash) == nil {
			// touch last_used_at (best effort, no bloqueante)
			go func(id uuid.UUID) {
				ctx2, cancel := context.WithTimeout(context.Background(), 1e9) // 1s
				defer cancel()
				_, _ = s.Pool.Exec(ctx2,
					`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, id)
			}(id)
			return &Principal{
				UserID:         userID.String(),
				OrganizationID: orgID.String(),
				APIKeyID:       id.String(),
				Role:           role,
			}, nil
		}
	}
	return nil, ErrNotFound
}

// UserLookup adapter para otp.UserLookup interface.
type UserLookup struct {
	Pool *pgxpool.Pool
}

type UserRow struct {
	ID    uuid.UUID
	Email string
	RUT   string
}

func (u *UserLookup) ByEmail(ctx context.Context, email string) (*UserRow, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var ur UserRow
	err := u.Pool.QueryRow(ctx,
		`SELECT id, email FROM users WHERE LOWER(email) = $1 AND deleted_at IS NULL LIMIT 1`,
		email,
	).Scan(&ur.ID, &ur.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ur, nil
}

func (u *UserLookup) ByRUT(ctx context.Context, rut string) (*UserRow, error) {
	// Tabla users no tiene columna RUT explícita en migration 000003;
	// futura migración HU-02.7 puede agregarla. Por ahora: NotFound.
	return nil, ErrNotFound
}
