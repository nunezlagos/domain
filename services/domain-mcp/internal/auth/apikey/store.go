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

// ISSUE-21.6: UUID canónico para single-org. Se usa para poblar
// APIKeyInfo.OrgID y variables orgID locales ahora que la columna
// u.organization_id no se selecciona (Fase C).
var canonicalOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// PGStore Postgres-backed.
type PGStore struct {
	Pool *pgxpool.Pool
}

// Issue genera nueva key para el user, persiste hash + prefix, retorna plaintext.
// Plaintext SOLO disponible aquí; subsequente lookup retorna hash, no plaintext.
// orgID se mantiene en la firma por compat de callers pero ya NO se persiste:
// el org de una key se deriva de su user (users.organization_id) en Resolve/List.
func (s *PGStore) Issue(ctx context.Context, orgID, userID uuid.UUID, name, env string) (plaintext string, keyID uuid.UUID, err error) {
	_ = orgID
	plaintext, prefix, hash, err := Generate(env)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("generate: %w", err)
	}
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO auth_api_keys (user_id, key_hash, key_prefix, name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		userID, hash, prefix, name,
	).Scan(&keyID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("insert api_key: %w", err)
	}
	return plaintext, keyID, nil
}

// APIKeyInfo representación pública de una API key (sin hash ni plaintext).
type APIKeyInfo struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"organization_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// List retorna keys activas (no revoked). ISSUE-21.6 Fase D clean:
// single-org, WHERE sin organization_id (la tabla auth_api_keys.organization_id
// es nullable tras Fase B; el JOIN a users.organization_id se conserva
// solo para devolver el campo en APIKeyInfo.OrgID).
func (s *PGStore) List(ctx context.Context, orgID uuid.UUID) ([]APIKeyInfo, error) {
	_ = orgID
	// ISSUE-21.6: SELECT sin u.organization_id (dropeado en Fase C);
	// se sigue retornando OrgID en APIKeyInfo con default canónico.
	rows, err := s.Pool.Query(ctx, `
		SELECT k.id, k.user_id, k.name, k.key_prefix,
		       k.last_used_at, k.expires_at, k.revoked_at, k.created_at
		FROM auth_api_keys k
		WHERE k.revoked_at IS NULL
		ORDER BY k.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var keys []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		// ISSUE-21.6: OrgID canónico single-org (la columna
		// u.organization_id no se selecciona de la DB).
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix,
			&k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		k.OrgID = canonicalOrgID
		keys = append(keys, k)
	}
	return keys, nil
}

// Rotate genera nueva key, persiste, y revoca la anterior en una transacción.
// Retorna plaintext de la nueva key.
func (s *PGStore) Rotate(ctx context.Context, oldKeyID uuid.UUID, orgID, userID uuid.UUID, name, env string) (newPlaintext string, newKeyID uuid.UUID, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_ = orgID // ya no se persiste en auth_api_keys; el org se deriva del user.
	newPlaintext, prefix, hash, err := Generate(env)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("generate: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO auth_api_keys (user_id, key_hash, key_prefix, name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		userID, hash, prefix, name,
	).Scan(&newKeyID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("insert new key: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE auth_api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, oldKeyID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("revoke old key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", uuid.Nil, ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return "", uuid.Nil, fmt.Errorf("commit rotate: %w", err)
	}
	return newPlaintext, newKeyID, nil
}

// Revoke marca soft-delete sobre la key.
func (s *PGStore) Revoke(ctx context.Context, keyID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE auth_api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, keyID)
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
	// ISSUE-21.6: SELECT sin u.organization_id (dropeado en Fase C).
	rows, err := s.Pool.Query(ctx,
		`SELECT k.id, k.user_id, k.key_hash, COALESCE(u.role,'viewer')
		 FROM auth_api_keys k
		 JOIN users u ON u.id = k.user_id
		 WHERE k.key_prefix = $1
		   AND k.revoked_at IS NULL
		   AND (k.expires_at IS NULL OR k.expires_at > NOW())
		   AND u.deleted_at IS NULL`,
		prefix)
	if err != nil {
		return nil, fmt.Errorf("query auth_api_keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		// ISSUE-21.6: orgID local al Scan (ya no se selecciona de la DB).
		var (
			id     uuid.UUID
			userID uuid.UUID
			hash   []byte
			role   string
		)
		_ = id // no-op: id local al Scan
		if err := rows.Scan(&id, &userID, &hash, &role); err != nil {
			return nil, err
		}
		orgID := canonicalOrgID
		_ = orgID
		if Verify(plaintext, hash) == nil {
			// touch last_used_at (best effort, no bloqueante)
			go func(id uuid.UUID) {
				ctx2, cancel := context.WithTimeout(context.Background(), 1e9) // 1s
				defer cancel()
				_, _ = s.Pool.Exec(ctx2,
					`UPDATE auth_api_keys SET last_used_at = NOW() WHERE id = $1`, id)
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
	// futura migración issue-02.7 puede agregarla. Por ahora: NotFound.
	return nil, ErrNotFound
}
