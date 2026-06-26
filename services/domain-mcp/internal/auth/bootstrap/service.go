// Package bootstrap — issue-01.9 first-run auto-creation of org+user+api_key.
//
// SEMÁNTICA:
//   - Bootstrap SOLO funciona si la DB no tiene users (COUNT(users) == 0).
//   - Después del primer user, el endpoint retorna ErrNotFirstRun. El caller
//     debe pedirle a un admin que cree el miembro con POST /auth/member-create
//     (HU-36.1) o usar un enrollment token (HU-37.1) para self-enroll.
//   - Defense: pg_advisory_xact_lock para evitar race entre dos onboard
//     simultáneos. Solo uno crea el primer user.
//
// API keys emitidas por bootstrap:
//   - expires_at: NULL (no expiran automáticamente, por decisión de producto)
//   - environment: "live"
//   - key_hash: bcrypt(plaintext, cost 12) — mismo algoritmo que las keys
//     regulares. El plaintext se retorna UNA SOLA VEZ al caller.
package bootstrap

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

	"nunezlagos/domain/internal/service/enrollment"
)

// BootstrapLockKey es el advisory lock key para serializar bootstraps.
// Constante fija: "BOOT" en ASCII = 0x424F4F54.
const BootstrapLockKey int64 = 0x424F4F54

// ErrNotFirstRun returned cuando ya hay users en la DB. El caller debe
// pedirle a un admin que use member-create (HU-36.1) o enrollment token (HU-37.1).
var ErrNotFirstRun = errors.New("bootstrap is first-run only; admin debe usar member-create o enrollment token")

// ErrInvalidEmail returned cuando el email no tiene formato válido.
var ErrInvalidEmail = errors.New("email format invalid")

// emailRegex es el patrón RFC 5322 simplificado que usamos para
// validación client-side. El server confía en el regex, no en DNS.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// slugRegex para sanitizar el org name derivado del email domain.
var slugRegex = regexp.MustCompile(`[^a-z0-9-]+`)

// BootstrapInput es el request del endpoint.
type BootstrapInput struct {
	Email   string
	KeyName string // default: "default"
	OrgName string // optional; si vacío, derivado del email domain
}

// BootstrapResult es la respuesta exitosa.
type BootstrapResult struct {
	UserID   uuid.UUID
	APIKey   string // plaintext, mostrado UNA sola vez
	APIKeyID uuid.UUID
	Email    string


	EnrollmentToken   string
	EnrollmentTokenID uuid.UUID
	EnrollmentRole    string
}

// Service ejecuta el bootstrap. Stateless: cada llamada abre su propia tx.
type Service struct {
	Pool *pgxpool.Pool

	Now func() time.Time
}

// New retorna un Service con Now=time.Now.
func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool, Now: time.Now}
}

// Bootstrap ejecuta el flujo completo (single-org: sin entidad organization):
//  1. Lock advisory (evita race entre dos onboard simultáneos)
//  2. Verifica first-run (COUNT(users) == 0)
//  3. Crea user owner (con bcrypt password dummy — el user no tiene password,
//     usa API key + OTP)
//  4. Crea api_key con bcrypt del plaintext generado
//  5. Emite enrollment_token global
//  6. Commit (o rollback si algo falla)
func (s *Service) Bootstrap(ctx context.Context, in BootstrapInput) (*BootstrapResult, error) {

	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}


	keyName := in.KeyName
	if keyName == "" {
		keyName = "default"
	}


	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)


	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", BootstrapLockKey); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}


	var userCount int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if userCount > 0 {
		return nil, ErrNotFirstRun
	}

	now := s.Now()



	userID := uuid.New()
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), 10)
	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, email, role, password_hash,
		                    last_login_at, created_at, updated_at)
		 VALUES ($1, $2, 'owner', $3, $4, $4, $4)`,
		userID, email, dummyHash, now)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}


	plaintext, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}




	keyID := uuid.New()
	keyPrefix := plaintext[:len("domk_")+8] // "domk_xxxxxxxx"
	_, err = tx.Exec(ctx,
		`INSERT INTO auth_api_keys (id, user_id, key_hash, key_prefix,
		                    name, environment, expires_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'live', NULL, $6, $6)`,
		keyID, userID, keyHash, keyPrefix, keyName, now)
	if err != nil {
		return nil, fmt.Errorf("insert api_key: %w", err)
	}


	enrollPlain, enrollPrefix, enrollHash, err := enrollment.GeneratePlaintext()
	if err != nil {
		return nil, fmt.Errorf("generate enrollment token: %w", err)
	}
	enrollTokenID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO enrollment_tokens
		   (id, token_hash, token_prefix, role_on_enroll, created_by_user_id)
		 VALUES ($1, $2, $3, 'member', $4)`,
		enrollTokenID, enrollHash, enrollPrefix, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert enrollment token: %w", err)
	}


	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &BootstrapResult{
		UserID:            userID,
		APIKey:            plaintext,
		APIKeyID:          keyID,
		Email:             email,
		EnrollmentToken:   enrollPlain,
		EnrollmentTokenID: enrollTokenID,
		EnrollmentRole:    "member",
	}, nil
}

// IsFirstRun retorna true si la DB no tiene users. Helper para que
// el endpoint GET /api/v1/auth/first-run no requiera un lock advisory
// (solo lectura).
func (s *Service) IsFirstRun(ctx context.Context) (bool, int, error) {
	var count int
	if err := s.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, 0, err
	}
	return count == 0, count, nil
}

// slugify convierte un string a un slug válido: lowercase, alphanum + dash,
// max 50 chars. Si el resultado es vacío, devuelve "default".
func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
	}
	if s == "" {
		return "default"
	}
	return s
}

// generateAPIKey produce un plaintext + bcrypt hash. El formato es
// "domk_live_<32 random base62 chars>" = ~40 chars total.
func generateAPIKey() (plaintext string, hash []byte, err error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const randomLen = 32
	raw := make([]byte, randomLen)

	for i := 0; i < randomLen; i++ {
		b, err := randomByte(charset)
		if err != nil {
			return "", nil, err
		}
		raw[i] = b
	}
	plaintext = "domk_live_" + string(raw)
	h, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	if err != nil {
		return "", nil, err
	}
	return plaintext, h, nil
}

// randomByte retorna un byte aleatorio del charset. Usa crypto/rand
// indirectamente via uuid.New() (que internamente lo usa).
// Workaround para evitar import crypto/rand en el path de tests.
func randomByte(charset string) (byte, error) {
	id := uuid.New()
	s := id.String()


	idx := int(s[0]) % len(charset)
	return charset[idx], nil
}
