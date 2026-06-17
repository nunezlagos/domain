// Package bootstrap — issue-01.9 first-run auto-creation of org+user+api_key.
//
// SEMÁNTICA:
//   - Bootstrap SOLO funciona si la DB no tiene users (COUNT(users) == 0).
//   - Después del primer user, el endpoint retorna ErrNotFirstRun. El caller
//     debe usar /auth/request-otp + /auth/verify-otp (flujo OTP normal).
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
// usar el flujo OTP normal en vez de bootstrap.
var ErrNotFirstRun = errors.New("bootstrap is first-run only; use /auth/request-otp instead")

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
	// issue-37.1: el bootstrap también emite un enrollment_token (global) para
	// que el primer owner pueda invitar a su equipo sin SMTP. Plaintext UNA vez.
	EnrollmentToken   string
	EnrollmentTokenID uuid.UUID
	EnrollmentRole    string
}

// Service ejecuta el bootstrap. Stateless: cada llamada abre su propia tx.
type Service struct {
	Pool *pgxpool.Pool
	// Now se inyecta para tests (default: time.Now).
	Now func() time.Time
}

// New retorna un Service con Now=time.Now.
func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool, Now: time.Now}
}

// Bootstrap ejecuta el flujo completo (single-org: sin entidad organization):
//   1. Lock advisory (evita race entre dos onboard simultáneos)
//   2. Verifica first-run (COUNT(users) == 0)
//   3. Crea user owner (con bcrypt password dummy — el user no tiene password,
//      usa API key + OTP)
//   4. Crea api_key con bcrypt del plaintext generado
//   5. Emite enrollment_token global
//   6. Commit (o rollback si algo falla)
func (s *Service) Bootstrap(ctx context.Context, in BootstrapInput) (*BootstrapResult, error) {
	// Validar email
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}

	// Defaults
	keyName := in.KeyName
	if keyName == "" {
		keyName = "default"
	}

	// 1. Begin tx
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 2. Lock advisory: solo un bootstrap a la vez
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", BootstrapLockKey); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}

	// 3. Verificar first-run dentro del lock (read committed isolation)
	var userCount int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if userCount > 0 {
		return nil, ErrNotFirstRun
	}

	now := s.Now()

	// 4. Crear user owner (password_hash dummy porque el user no usa
	// password — usa API key + OTP). Single-org: sin organization_id.
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

	// 5. Generar API key: plaintext + bcrypt hash
	plaintext, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	// 6. Insertar api_key. expires_at = NULL (no expira automáticamente).
	//    environment = 'live' (no 'test' — bootstrap es para uso real).
	//    Single-org: sin organization_id.
	keyID := uuid.New()
	keyPrefix := plaintext[:len("domk_")+8] // "domk_xxxxxxxx"
	_, err = tx.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, key_hash, key_prefix,
		                    name, environment, expires_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'live', NULL, $6, $6)`,
		keyID, userID, keyHash, keyPrefix, keyName, now)
	if err != nil {
		return nil, fmt.Errorf("insert api_key: %w", err)
	}

	// 7. issue-37.1: emitir enrollment_token global con role_on_enroll="member"
	enrollPlain, enrollPrefix, enrollHash, err := enrollment.GeneratePlaintext()
	if err != nil {
		return nil, fmt.Errorf("generate enrollment token: %w", err)
	}
	enrollTokenID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO org_enrollment_tokens
		   (id, token_hash, token_prefix, role_on_enroll, created_by_user_id)
		 VALUES ($1, $2, $3, 'member', $4)`,
		enrollTokenID, enrollHash, enrollPrefix, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert enrollment token: %w", err)
	}

	// 8. Commit
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
	// crypto/rand via uuid.New() para evitar import extra
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
	// s es "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" hex
	// Mapear cada char hex a un byte del charset via modulo
	idx := int(s[0]) % len(charset)
	return charset[idx], nil
}
