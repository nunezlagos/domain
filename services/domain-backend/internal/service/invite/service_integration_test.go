//go:build integration

// issue-21.2 invitations integration tests.

package invite_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/invite"
	orgsvc "nunezlagos/domain/internal/service/org"
)

type capturedMail struct {
	to, subject, body string
}

type recordingMailer struct {
	sent []capturedMail
}

func (m *recordingMailer) Send(ctx context.Context, to, subject, body string) error {
	m.sent = append(m.sent, capturedMail{to, subject, body})
	return nil
}

func setupInvite(t *testing.T) (*invite.Service, *orgsvc.Service, *recordingMailer, uuid.UUID, uuid.UUID, func()) {
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

	rec := &audit.PGRecorder{Pool: pool}
	orgS := &orgsvc.Service{Pool: pool, Audit: rec}
	mailer := &recordingMailer{}
	invS := &invite.Service{
		Pool: pool, Audit: rec, Mailer: mailer,
		AcceptURL: "https://app.test/accept",
	}

	org, owner, err := orgS.Create(ctx, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
	return invS, orgS, mailer, org.ID, owner.UserID, cleanup
}

// Escenario 1: Crear invitación + email enviado + audit.
func TestInvite_Create_SendsEmailAndAudits(t *testing.T) {
	invS, _, mailer, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()

	inv, err := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.NoError(t, err)
	require.Equal(t, "bob@x.com", inv.Email)
	require.Equal(t, "member", inv.Role)
	require.Equal(t, invite.StatusPending, inv.Status)
	require.True(t, inv.ExpiresAt.After(time.Now()),
		"expires_at debe ser futuro")
	require.True(t, inv.ExpiresAt.Before(time.Now().Add(8*24*time.Hour)),
		"expires_at <= 7d + slack")

	require.Len(t, mailer.sent, 1)
	require.Contains(t, mailer.sent[0].body, inv.Token.String(),
		"el email debe incluir el token de aceptación")

	var count int
	require.NoError(t, invS.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action='invitation.sent' AND organization_id=$1`,
		orgID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestInvite_Create_InvalidRole(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	_, err := invS.Create(ctx, orgID, ownerID, "bob@x.com", "superuser")
	require.ErrorIs(t, err, invite.ErrInvalidRole)
}

func TestInvite_Create_NoDoublePending(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	_, err := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.NoError(t, err)
	_, err = invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.ErrorIs(t, err, invite.ErrAlreadyPending)
}

// Escenario 2: Accept válido crea user + marca accepted + audit.
func TestInvite_Accept_Success(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()

	inv, err := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.NoError(t, err)

	userID, gotOrg, role, err := invS.Accept(ctx, inv.Token, "bob@x.com", "Bob")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, userID)
	require.Equal(t, orgID, gotOrg)
	require.Equal(t, "member", role)

	after, err := invS.GetByToken(ctx, inv.Token)
	require.NoError(t, err)
	require.Equal(t, invite.StatusAccepted, after.Status)
	require.NotNil(t, after.AcceptedUserID)
	require.Equal(t, userID, *after.AcceptedUserID)
}

// Escenario 4: Email mismatch.
func TestInvite_Accept_EmailMismatch(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	inv, _ := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	_, _, _, err := invS.Accept(ctx, inv.Token, "alice@x.com", "Alice")
	require.ErrorIs(t, err, invite.ErrEmailMismatch)
}

// Escenario 5: Token expirado.
func TestInvite_Accept_Expired(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	inv, _ := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	// Forzar expiración
	_, err := invS.Pool.Exec(ctx,
		`UPDATE invitations SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, inv.ID)
	require.NoError(t, err)

	_, _, _, err = invS.Accept(ctx, inv.Token, "bob@x.com", "Bob")
	require.ErrorIs(t, err, invite.ErrExpired)

	after, _ := invS.GetByToken(ctx, inv.Token)
	require.Equal(t, invite.StatusExpired, after.Status,
		"Accept sobre expirada debe transicionar status a expired")
}

// Escenario 3: Decline marca declined sin crear user.
func TestInvite_Decline(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	inv, _ := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.NoError(t, invS.Decline(ctx, inv.Token))
	after, _ := invS.GetByToken(ctx, inv.Token)
	require.Equal(t, invite.StatusDeclined, after.Status)

	// User no debe haberse creado
	var count int
	require.NoError(t, invS.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE email = 'bob@x.com'`).Scan(&count))
	require.Equal(t, 0, count)
}

// Escenario 6: Revoke.
func TestInvite_Revoke(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	inv, _ := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	require.NoError(t, invS.Revoke(ctx, inv.ID, ownerID))
	after, _ := invS.GetByToken(ctx, inv.Token)
	require.Equal(t, invite.StatusRevoked, after.Status)

	// Aceptación posterior falla con ErrNotPending
	_, _, _, err := invS.Accept(ctx, inv.Token, "bob@x.com", "Bob")
	require.ErrorIs(t, err, invite.ErrNotPending)
}

// Cron diario: ExpireOverdue transiciona pending vencidas.
func TestInvite_ExpireOverdue(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := invS.Create(ctx, orgID, ownerID, "a@x.com", "member")
	b, _ := invS.Create(ctx, orgID, ownerID, "b@x.com", "member")

	// Solo a expira
	_, err := invS.Pool.Exec(ctx,
		`UPDATE invitations SET expires_at = NOW() - INTERVAL '1 day' WHERE id = $1`, a.ID)
	require.NoError(t, err)

	n, err := invS.ExpireOverdue(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	aAfter, _ := invS.GetByToken(ctx, a.Token)
	bAfter, _ := invS.GetByToken(ctx, b.Token)
	require.Equal(t, invite.StatusExpired, aAfter.Status)
	require.Equal(t, invite.StatusPending, bAfter.Status, "b sigue pending")
}

func TestInvite_ListByOrg(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = invS.Create(ctx, orgID, ownerID, "a@x.com", "member")
	_, _ = invS.Create(ctx, orgID, ownerID, "b@x.com", "admin")
	list, err := invS.ListByOrg(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

// Sabotaje: token bypass — invitación accepted no debe poder aceptarse de nuevo.
func TestSabotage_DoubleAccept_Rejected(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	inv, _ := invS.Create(ctx, orgID, ownerID, "bob@x.com", "member")
	_, _, _, err := invS.Accept(ctx, inv.Token, "bob@x.com", "Bob")
	require.NoError(t, err)
	_, _, _, err = invS.Accept(ctx, inv.Token, "bob@x.com", "Bob")
	require.ErrorIs(t, err, invite.ErrNotPending,
		"segunda aceptación con mismo token DEBE fallar")
}

// Sabotaje: si Pending unique constraint fuera removido, doble pending crearía
// ambigüedad — este test prueba que el constraint efectivamente bloquea.
func TestSabotage_PendingUnique_Enforced(t *testing.T) {
	invS, _, _, orgID, ownerID, cleanup := setupInvite(t)
	defer cleanup()
	ctx := context.Background()
	_, err := invS.Create(ctx, orgID, ownerID, "dup@x.com", "member")
	require.NoError(t, err)
	_, err = invS.Create(ctx, orgID, ownerID, "dup@x.com", "admin")
	require.ErrorIs(t, err, invite.ErrAlreadyPending,
		"unique constraint pending (org,email) DEBE prevenir doble invite")
}
