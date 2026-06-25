package enrollment

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
)

// Errores tipados expuestos por el service.
var (
	ErrInvalidToken = errors.New("enrollment token invalid or revoked")
	ErrInvalidEmail = errors.New("email format invalid")
	ErrInvalidRole  = errors.New("role must be one of: owner, admin, maintainer, member, viewer")
	ErrEmailTaken   = errors.New("email already in use within the organization")
	ErrOrgNotFound  = errors.New("organization not found")
	ErrNoActive     = errors.New("no active enrollment token for this organization")
)

// emailRegex simple — sin DNS check.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// allowedRoles cierra el conjunto de valores aceptados para role_on_enroll.
var allowedRoles = map[string]bool{
	"owner":      true,
	"admin":      true,
	"maintainer": true,
	"member":     true,
	"viewer":     true,
}

// RotateResult información que devolvemos al admin al rotar (plaintext UNA vez).
type RotateResult struct {
	Plaintext    string
	Prefix       string
	RoleOnEnroll string
	CreatedAt    time.Time
}

// Metadata estado del token activo de una org (sin plaintext).
type Metadata struct {
	Exists       bool
	Prefix       string
	RoleOnEnroll string
	CreatedAt    time.Time
}

// EnrollResult lo que recibe quien se auto-enrola: user + api key personal.
type EnrollResult struct {
	UserID    uuid.UUID
	Email     string
	Name      string
	Role      string
	APIKey    string // plaintext UNA vez
	APIKeyID  uuid.UUID
	KeyPrefix string
}

// Service expone los flows de rotación/revoke/enrollment.
type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

// Rotate revoca el token activo global (si lo hay) y crea uno nuevo. Atomic
// en una tx. El plaintext del nuevo token se devuelve UNA sola vez.
//
// Single-org (issue-37 global): un único token de enrollment activo, sin
// organization_id. `role` puede ser "" → default "member". Solo whitelisted.
func (s *Service) Rotate(ctx context.Context, actorID uuid.UUID, role string) (*RotateResult, error) {
	if role == "" {
		role = "member"
	}
	if !allowedRoles[role] {
		return nil, ErrInvalidRole
	}

	plaintext, prefix, hash, err := GeneratePlaintext()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE enrollment_tokens
		 SET revoked_at = NOW()
		 WHERE revoked_at IS NULL`,
	); err != nil {
		return nil, fmt.Errorf("revoke previous: %w", err)
	}

	var createdAt time.Time
	var actorParam any
	if actorID == uuid.Nil {
		actorParam = nil
	} else {
		actorParam = actorID
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO enrollment_tokens
		   (token_hash, token_prefix, role_on_enroll, created_by_user_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		hash, prefix, role, actorParam,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		actorPtr := &actorID
		if actorID == uuid.Nil {
			actorPtr = nil
		}
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    actorPtr,
			ActorType:  audit.ActorUser,
			Action:     "enrollment_token.rotated",
			EntityType: "enrollment_token",
			NewValues: map[string]any{
				"role_on_enroll": role,
				"key_prefix":     prefix,
			},
		})
	}

	return &RotateResult{
		Plaintext:    plaintext,
		Prefix:       prefix,
		RoleOnEnroll: role,
		CreatedAt:    createdAt,
	}, nil
}

// GetMetadata devuelve el estado del token activo global (si existe), SIN el
// plaintext.
func (s *Service) GetMetadata(ctx context.Context) (*Metadata, error) {
	var m Metadata
	err := s.Pool.QueryRow(ctx,
		`SELECT token_prefix, role_on_enroll, created_at
		 FROM enrollment_tokens
		 WHERE revoked_at IS NULL`,
	).Scan(&m.Prefix, &m.RoleOnEnroll, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Metadata{Exists: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query metadata: %w", err)
	}
	m.Exists = true
	return &m, nil
}

// Revoke marca el token activo global como revoked_at=NOW sin crear uno nuevo.
// Si no hay token activo, devuelve ErrNoActive.
func (s *Service) Revoke(ctx context.Context, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE enrollment_tokens
		 SET revoked_at = NOW()
		 WHERE revoked_at IS NULL`,
	)
	if err != nil {
		return fmt.Errorf("revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoActive
	}
	if s.Audit != nil {
		actorPtr := &actorID
		if actorID == uuid.Nil {
			actorPtr = nil
		}
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    actorPtr,
			ActorType:  audit.ActorUser,
			Action:     "enrollment_token.revoked",
			EntityType: "enrollment_token",
		})
	}
	return nil
}

// Enroll valida el plaintext del token, crea user + api_key en la org del
// token, devuelve la api_key personal UNA sola vez al enrollee.
//
// Anti-enumeration: si el token no existe o está revocado, devuelve
// ErrInvalidToken con timing comparable al match exitoso (bcrypt dummy).
func (s *Service) Enroll(ctx context.Context, plaintext, email, name string) (*EnrollResult, error) {
	prefix, perr := ParsePrefix(plaintext)
	if perr != nil {


		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(plaintext))
		return nil, ErrInvalidToken
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}


	rows, err := s.Pool.Query(ctx,
		`SELECT id, token_hash, role_on_enroll
		 FROM enrollment_tokens
		 WHERE token_prefix = $1 AND revoked_at IS NULL`,
		prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}
	type candidate struct {
		tokenID uuid.UUID
		hash    []byte
		role    string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.tokenID, &c.hash, &c.role); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	rows.Close()

	var matched *candidate
	if len(candidates) == 0 {

		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(plaintext))
	} else {
		for i := range candidates {
			if err := VerifyHash(plaintext, candidates[i].hash); err == nil {
				matched = &candidates[i]
				break
			}
		}
	}
	if matched == nil {
		return nil, ErrInvalidToken
	}


	apiPlain, apiPrefix, apiHash, err := apikey.Generate("live")
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	var createdAt time.Time
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, name, role)
		 VALUES ($1, NULLIF($2, ''), $3)
		 RETURNING id, created_at`,
		email, name, matched.role,
	).Scan(&userID, &createdAt)
	if err != nil {
		if isEmailUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	keyID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO auth_api_keys (id, user_id, key_hash, key_prefix,
		                        name, environment, expires_at)
		 VALUES ($1, $2, $3, $4, 'default', 'live', NULL)`,
		keyID, userID, apiHash, apiPrefix,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api_key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &userID,
			ActorType:  audit.ActorUser,
			Action:     "user.self_enrolled",
			EntityType: "user",
			EntityID:   &userID,
			NewValues: map[string]any{
				"email":           email,
				"role":            matched.role,
				"enroll_token_id": matched.tokenID,
				"api_key_prefix":  apiPrefix,
			},
		})
	}

	return &EnrollResult{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Role:      matched.role,
		APIKey:    apiPlain,
		APIKeyID:  keyID,
		KeyPrefix: apiPrefix,
	}, nil
}

// isEmailUniqueViolation reusa la heurística de orgsvc: matchea por substring
// porque pgx no expone PgError directo en todos los paths.
func isEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "duplicate key") {
		return false
	}
	return strings.Contains(msg, "email") ||
		(strings.Contains(msg, "users_") && strings.Contains(msg, "uniq"))
}
