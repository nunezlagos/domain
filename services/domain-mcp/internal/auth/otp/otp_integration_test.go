//go:build integration



package otp_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/auth/otp"
	dmigrate "nunezlagos/domain/internal/migrate"
)

func setupOTP(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func seedUser(t *testing.T, pool *pgxpool.Pool, email, rut string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('T', 't') ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	var userID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO users (organization_id, email, rut, role)
		VALUES ($1, $2, $3, 'member') RETURNING id
	`, orgID, email, nullIfEmpty(rut)).Scan(&userID)
	require.NoError(t, err)
	return userID
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fakeLookup implementa UserLookup contra DB real.
type fakeLookup struct{ Pool *pgxpool.Pool }

func (f *fakeLookup) ByEmail(ctx context.Context, email string) (*otp.User, error) {
	var u otp.User
	var rut *string
	err := f.Pool.QueryRow(ctx,
		`SELECT id, email, rut FROM users WHERE email = $1 AND deleted_at IS NULL`,
		email,
	).Scan(&u.ID, &u.Email, &rut)
	if err != nil {
		return nil, err
	}
	if rut != nil {
		u.RUT = *rut
	}
	return &u, nil
}

func (f *fakeLookup) ByRUT(ctx context.Context, r string) (*otp.User, error) {
	var u otp.User
	err := f.Pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(rut, '') FROM users WHERE rut = $1 AND deleted_at IS NULL`,
		r,
	).Scan(&u.ID, &u.Email, &u.RUT)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// fakeMailer captura código para tests + count calls.
type fakeMailer struct {
	calls    atomic.Int32
	lastCode string
	lastTo   string
}

func (f *fakeMailer) SendOTP(_ context.Context, to, code string, _ time.Duration) error {
	f.calls.Add(1)
	f.lastCode = code
	f.lastTo = to
	return nil
}

// Escenario 1: Request happy path — código generado + email enviado.
func TestOTP_Request_HappyPath(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}

	err := svc.Request(context.Background(), "alice@example.com", "1.2.3.4", "test/1.0")
	require.NoError(t, err)
	require.Equal(t, int32(1), mail.calls.Load())
	require.Equal(t, "alice@example.com", mail.lastTo)
	require.Len(t, mail.lastCode, 6)


	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM auth_otp_codes WHERE used_at IS NULL`,
	).Scan(&count))
	require.Equal(t, 1, count)
}

// Escenario 2: Request con user inexistente — anti-enumeration.
func TestOTP_Request_UnknownIdentifier_AntiEnum(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}

	err := svc.Request(context.Background(), "ghost@example.com", "", "")
	require.ErrorIs(t, err, otp.ErrUserNotFound, "uso interno; caller responde 200 fake al cliente")
	require.Equal(t, int32(0), mail.calls.Load(), "no debe enviar email")
}

// Escenario 3: Verify happy path.
func TestOTP_Verify_HappyPath(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}

	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))
	code := mail.lastCode

	result, err := svc.Verify(context.Background(), "alice@example.com", code)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, result.UserID)
	require.Equal(t, "alice@example.com", result.Email)


	var used *time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT used_at FROM auth_otp_codes WHERE id = $1`, result.OTPID,
	).Scan(&used))
	require.NotNil(t, used)


	var lastLogin *time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT last_login_at FROM users WHERE email = $1`, "alice@example.com",
	).Scan(&lastLogin))
	require.NotNil(t, lastLogin)
}

// Escenario 4: Verify código incorrecto incrementa attempts.
func TestOTP_Verify_WrongCode_Attempts(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}
	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))

	res, err := svc.Verify(context.Background(), "alice@example.com", "000000")
	require.ErrorIs(t, err, otp.ErrInvalidCode)
	require.Equal(t, 4, res.AttemptsLeft, "max 5, 1 attempt used")


	var attempts int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT attempts FROM auth_otp_codes WHERE user_id = (SELECT id FROM users WHERE email='alice@example.com')`,
	).Scan(&attempts))
	require.Equal(t, 1, attempts)
}

// Escenario 5: Demasiados intentos.
func TestOTP_Verify_TooManyAttempts(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail, MaxAttempts: 3}
	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))


	for i := 0; i < 3; i++ {
		_, err := svc.Verify(context.Background(), "alice@example.com", "000000")
		require.Error(t, err)
	}

	_, err := svc.Verify(context.Background(), "alice@example.com", "000000")
	require.ErrorIs(t, err, otp.ErrTooManyAttempts)
}

// Escenario 6: OTP expirado.
func TestOTP_Verify_Expired(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	userID := seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail, TTL: 100 * time.Millisecond}
	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))


	_, err := pool.Exec(context.Background(),
		`UPDATE auth_otp_codes SET expires_at = NOW() - interval '1 second' WHERE user_id = $1`, userID)
	require.NoError(t, err)

	_, err = svc.Verify(context.Background(), "alice@example.com", mail.lastCode)
	require.ErrorIs(t, err, otp.ErrOTPExpired)
}

// Escenario 7: OTP ya usado.
func TestOTP_Verify_AlreadyUsed(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}
	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))

	_, err := svc.Verify(context.Background(), "alice@example.com", mail.lastCode)
	require.NoError(t, err)


	_, err = svc.Verify(context.Background(), "alice@example.com", mail.lastCode)
	require.True(t, errors.Is(err, otp.ErrOTPAlreadyUsed) || errors.Is(err, otp.ErrNoActiveOTP),
		"second verify debe rechazar; got: %v", err)
}

// Escenario 8: Verify por RUT.
func TestOTP_Verify_ByRUT(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "bob@example.com", "12345678-5")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail}

	require.NoError(t, svc.Request(context.Background(), "12345678-5", "", ""))
	res, err := svc.Verify(context.Background(), "12345678-5", mail.lastCode)
	require.NoError(t, err)
	require.Equal(t, "bob@example.com", res.Email)
}

// Sabotaje: invalid code 6x → todos rechazan + último es TooManyAttempts.
func TestSabotage_OTP_BruteForceBlocked(t *testing.T) {
	pool, cleanup := setupOTP(t)
	defer cleanup()
	seedUser(t, pool, "alice@example.com", "")

	mail := &fakeMailer{}
	svc := &otp.Service{Pool: pool, Users: &fakeLookup{Pool: pool}, Mail: mail, MaxAttempts: 5}
	require.NoError(t, svc.Request(context.Background(), "alice@example.com", "", ""))


	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = svc.Verify(context.Background(), "alice@example.com", "999999")
	}
	require.Error(t, lastErr)


	_, err := svc.Verify(context.Background(), "alice@example.com", "999999")
	require.ErrorIs(t, err, otp.ErrTooManyAttempts)


	_, err = svc.Verify(context.Background(), "alice@example.com", mail.lastCode)
	require.ErrorIs(t, err, otp.ErrTooManyAttempts, "código correcto NO debe funcionar después de N attempts")
}
