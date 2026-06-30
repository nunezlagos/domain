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
	"nunezlagos/domain/internal/service/enrollment/enrollmentdb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrInvalidToken = errors.New("enrollment token invalid or revoked")
	ErrInvalidEmail = errors.New("email format invalid")
	ErrInvalidRole  = errors.New("role must be one of: owner, admin, maintainer, member, viewer")
	ErrEmailTaken   = errors.New("email already in use within the organization")
	ErrOrgNotFound  = errors.New("organization not found")
	ErrNoActive     = errors.New("no active enrollment token for this organization")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var allowedRoles = map[string]bool{
	"owner":      true,
	"admin":      true,
	"maintainer": true,
	"member":     true,
	"viewer":     true,
}

type RotateResult struct {
	Plaintext    string
	Prefix       string
	RoleOnEnroll string
	CreatedAt    time.Time
}

type Metadata struct {
	Exists       bool
	Prefix       string
	RoleOnEnroll string
	CreatedAt    time.Time
}

type EnrollResult struct {
	UserID    uuid.UUID
	Email     string
	Name      string
	Role      string
	APIKey    string
	APIKeyID  uuid.UUID
	KeyPrefix string
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

func (s *Service) q(ctx context.Context) *enrollmentdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return enrollmentdb.New(tx)
	}
	return enrollmentdb.New(s.Pool)
}

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

	q := enrollmentdb.New(tx)

	if _, err := q.RevokeAllActive(ctx); err != nil {
		return nil, fmt.Errorf("revoke previous: %w", err)
	}

	var actorParam *uuid.UUID
	if actorID != uuid.Nil {
		actorParam = &actorID
	}
	createdAt, err := q.InsertToken(ctx, enrollmentdb.InsertTokenParams{
		TokenHash:       hash,
		TokenPrefix:     prefix,
		RoleOnEnroll:    role,
		CreatedByUserID: actorParam,
	})
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

func (s *Service) GetMetadata(ctx context.Context) (*Metadata, error) {
	m, err := s.q(ctx).GetActiveMetadata(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &Metadata{Exists: false}, nil
		}
		return nil, fmt.Errorf("query metadata: %w", err)
	}
	return &Metadata{
		Exists:       true,
		Prefix:       m.TokenPrefix,
		RoleOnEnroll: m.RoleOnEnroll,
		CreatedAt:    m.CreatedAt,
	}, nil
}

func (s *Service) Revoke(ctx context.Context, actorID uuid.UUID) error {
	n, err := s.q(ctx).RevokeAllActive(ctx)
	if err != nil {
		return fmt.Errorf("revoke: %w", err)
	}
	if n == 0 {
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

	candidates, err := s.q(ctx).FindTokensByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}

	var matched *enrollmentdb.FindTokensByPrefixRow
	if len(candidates) == 0 {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(plaintext))
	} else {
		for i := range candidates {
			if err := VerifyHash(plaintext, candidates[i].TokenHash); err == nil {
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

	q := enrollmentdb.New(tx)

	user, err := q.InsertUser(ctx, enrollmentdb.InsertUserParams{
		Email: email,
		Name:  name,
		Role:  matched.RoleOnEnroll,
	})
	if err != nil {
		if isEmailUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	keyID := uuid.New()
	err = q.InsertAPIKey(ctx, enrollmentdb.InsertAPIKeyParams{
		ID:        keyID,
		UserID:    user.ID,
		KeyHash:   apiHash,
		KeyPrefix: apiPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("insert api_key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &user.ID,
			ActorType:  audit.ActorUser,
			Action:     "user.self_enrolled",
			EntityType: "user",
			EntityID:   &user.ID,
			NewValues: map[string]any{
				"email":           email,
				"role":            matched.RoleOnEnroll,
				"enroll_token_id": matched.ID,
				"api_key_prefix":  apiPrefix,
			},
		})
	}

	return &EnrollResult{
		UserID:    user.ID,
		Email:     email,
		Name:      name,
		Role:      matched.RoleOnEnroll,
		APIKey:    apiPlain,
		APIKeyID:  keyID,
		KeyPrefix: apiPrefix,
	}, nil
}

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
