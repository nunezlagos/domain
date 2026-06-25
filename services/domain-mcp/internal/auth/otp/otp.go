// Package otp — issue-02.7 passwordless OTP login.
//
// Flow:
//  1. Request: identifier (RUT o email) → genera código 6 dígitos, bcrypt hash,
//     persist en auth_otp_codes, envía via canal email (issue-20.2)
//  2. Verify: identifier + code + action (reveal | regenerate) → si OK devuelve
//     API key del user (reveal: actual decifrada; regenerate: nueva).
//
// Características:
//   - Anti-enumeration: response idéntica aunque user no exista
//   - Bcrypt cost 10 (vs 12 de API keys; OTP es ephemeral 10min)
//   - Max 5 attempts, single-use, TTL 10min
//   - SELECT FOR UPDATE para evitar race en verify concurrente
package otp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (

	CodeLength = 6

	BcryptCost = 10

	DefaultTTL = 10 * time.Minute

	DefaultMaxAttempts = 5
)

var (

	ErrNoActiveOTP = errors.New("no active otp")

	ErrOTPExpired = errors.New("otp expired")

	ErrOTPAlreadyUsed = errors.New("otp already used")

	ErrInvalidCode = errors.New("invalid code")

	ErrTooManyAttempts = errors.New("too many attempts")

	ErrUserNotFound = errors.New("user not found")
)

// User minimal para flow OTP (lookup por email o rut).
type User struct {
	ID    uuid.UUID
	Email string
	RUT   string
}

// UserLookup interface para resolver user desde identifier.
type UserLookup interface {
	ByEmail(ctx context.Context, email string) (*User, error)
	ByRUT(ctx context.Context, rut string) (*User, error)
}

// Mailer envía OTP code por email (canal issue-20.2).
type Mailer interface {
	SendOTP(ctx context.Context, to, code string, expiresIn time.Duration) error
}

// Service orquesta request + verify.
type Service struct {
	Pool          *pgxpool.Pool
	Users         UserLookup
	Mail          Mailer
	TTL           time.Duration     // 0 → DefaultTTL
	MaxAttempts   int               // 0 → DefaultMaxAttempts
	IdentifyAsRUT func(string) bool // hook custom; default: contiene "-" o todo dígitos
}

// generateCode crea código numérico de N dígitos crypto-seguro.
func generateCode(n int) (string, error) {
	maxVal := big.NewInt(1)
	for i := 0; i < n; i++ {
		maxVal.Mul(maxVal, big.NewInt(10))
	}
	num, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	s := num.String()
	for len(s) < n {
		s = "0" + s
	}
	return s, nil
}

// IsEmail simple heurística (no full RFC; suficiente para differentiation).
func IsEmail(s string) bool {
	return strings.Contains(s, "@")
}

// Request genera + persiste OTP + envía mail. Idempotente respecto a anti-enumeration:
// si user no existe NO error explícito (caller envía response 200 fake).
func (s *Service) Request(ctx context.Context, identifier, ipAddress, userAgent string) error {
	user, err := s.resolveUser(ctx, identifier)
	if err != nil {

		return ErrUserNotFound
	}

	code, err := generateCode(CodeLength)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), BcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}

	ttl := s.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	maxAtt := s.MaxAttempts
	if maxAtt == 0 {
		maxAtt = DefaultMaxAttempts
	}

	expiresAt := time.Now().Add(ttl)
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO auth_otp_codes (user_id, code_hash, max_attempts, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, hash, maxAtt, expiresAt, nullStr(ipAddress), nullStr(userAgent))
	if err != nil {
		return fmt.Errorf("insert otp: %w", err)
	}

	if s.Mail != nil {
		if err := s.Mail.SendOTP(ctx, user.Email, code, ttl); err != nil {
			return fmt.Errorf("send mail: %w", err)
		}
	}
	return nil
}

// VerifyResult contiene info para responder al cliente post-OTP.
type VerifyResult struct {
	UserID       uuid.UUID
	Email        string
	AttemptsLeft int // si falló y no llegó a límite
	OTPID        uuid.UUID
}

// Verify chequea code, marca used_at, retorna resultado.
// El caller (handler HTTP) usa UserID para emitir API key + responder JSON.
func (s *Service) Verify(ctx context.Context, identifier, code string) (*VerifyResult, error) {
	user, err := s.resolveUser(ctx, identifier)
	if err != nil {
		return nil, ErrInvalidCode
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		otpID       uuid.UUID
		hash        []byte
		attempts    int
		maxAttempts int
		expiresAt   time.Time
		usedAt      *time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT id, code_hash, attempts, max_attempts, expires_at, used_at
		FROM auth_otp_codes
		WHERE user_id = $1 AND used_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, user.ID).Scan(&otpID, &hash, &attempts, &maxAttempts, &expiresAt, &usedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveOTP
		}
		return nil, fmt.Errorf("select otp: %w", err)
	}
	if usedAt != nil {
		return nil, ErrOTPAlreadyUsed
	}
	if time.Now().After(expiresAt) {
		return nil, ErrOTPExpired
	}
	if attempts >= maxAttempts {
		return nil, ErrTooManyAttempts
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(code)); err != nil {

		_, e2 := tx.Exec(ctx, `UPDATE auth_otp_codes SET attempts = attempts + 1 WHERE id = $1`, otpID)
		if e2 == nil {
			_ = tx.Commit(ctx)
		}
		left := maxAttempts - (attempts + 1)
		if left <= 0 {
			return nil, ErrTooManyAttempts
		}
		return &VerifyResult{AttemptsLeft: left}, ErrInvalidCode
	}


	now := time.Now()
	_, err = tx.Exec(ctx, `UPDATE auth_otp_codes SET used_at = $1 WHERE id = $2`, now, otpID)
	if err != nil {
		return nil, fmt.Errorf("mark used: %w", err)
	}

	_, err = tx.Exec(ctx, `UPDATE users SET last_login_at = $1 WHERE id = $2`, now, user.ID)
	if err != nil {
		return nil, fmt.Errorf("update last_login: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &VerifyResult{
		UserID: user.ID,
		Email:  user.Email,
		OTPID:  otpID,
	}, nil
}

// resolveUser busca por email o RUT según formato del identifier.
func (s *Service) resolveUser(ctx context.Context, identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrUserNotFound
	}
	if IsEmail(identifier) {
		u, err := s.Users.ByEmail(ctx, strings.ToLower(identifier))
		if err != nil {
			return nil, ErrUserNotFound
		}
		return u, nil
	}


	u, err := s.Users.ByRUT(ctx, identifier)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
