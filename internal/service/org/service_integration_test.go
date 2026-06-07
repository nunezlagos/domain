//go:build integration

// HU-21.1 organization service integration tests.

package org_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/saargo/domain/internal/audit"
	dmigrate "github.com/saargo/domain/internal/migrate"
	orgsvc "github.com/saargo/domain/internal/service/org"
)

func setupOrgSvc(t *testing.T) (*orgsvc.Service, func()) {
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
	svc := &orgsvc.Service{Pool: pool, Audit: &audit.PGRecorder{Pool: pool}}
	return svc, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// Escenario 1: CRUD org + owner user creado atómico + audit log.
func TestOrg_Create_OwnerAndAudit(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()

	org, owner, err := svc.Create(ctx, "Acme Inc", "acme", "alice@acme.com", "Alice")
	require.NoError(t, err)
	require.Equal(t, "Acme Inc", org.Name)
	require.Equal(t, "acme", org.Slug)
	require.Equal(t, "alice@acme.com", owner.Email)
	require.Equal(t, orgsvc.RoleOwner, owner.Role)

	var auditCount int
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE organization_id = $1 AND action = 'organization.created'`,
		org.ID).Scan(&auditCount))
	require.Equal(t, 1, auditCount, "audit_log debe tener entrada organization.created")
}

func TestOrg_Create_SlugTaken(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	_, _, err := svc.Create(ctx, "A", "shared", "a@x.com", "A")
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, "B", "shared", "b@x.com", "B")
	require.ErrorIs(t, err, orgsvc.ErrSlugTaken)
}

func TestOrg_Create_InvalidSlug(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	cases := []string{"UPPER", "with space", "_leading", "trailing_"}
	for _, s := range cases {
		_, _, err := svc.Create(ctx, "X", s, "x@x.com", "X")
		require.ErrorIs(t, err, orgsvc.ErrSlugInvalid, "slug: %q", s)
	}
}

// Escenario 2: UpdateSettings persists + audit con diff.
func TestOrg_UpdateSettings_PersistsAndAudits(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, err := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)

	updated, err := svc.UpdateSettings(ctx, org.ID, owner.UserID, map[string]any{
		"timezone":      "America/Argentina/Buenos_Aires",
		"default_model": "claude-sonnet-4-6",
	})
	require.NoError(t, err)
	require.Equal(t, "America/Argentina/Buenos_Aires", updated.Settings["timezone"])
	require.Equal(t, "claude-sonnet-4-6", updated.Settings["default_model"])

	var auditCount int
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE organization_id = $1 AND action = 'organization.updated'`,
		org.ID).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
}

// Escenario 3: ListMembers visible y excluye deleted.
func TestOrg_ListMembers(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, err := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)

	_, err = svc.AddMember(ctx, org.ID, "a@x.com", "A", orgsvc.RoleAdmin)
	require.NoError(t, err)
	_, err = svc.AddMember(ctx, org.ID, "b@x.com", "B", orgsvc.RoleMember)
	require.NoError(t, err)

	members, err := svc.ListMembers(ctx, org.ID)
	require.NoError(t, err)
	require.Len(t, members, 3, "owner + 2 invited")
	require.Equal(t, owner.UserID, members[0].UserID, "primer member es el owner (creado primero)")
}

// Escenario 4: Transfer ownership swap correcto, idempotente respecto a roles.
func TestOrg_TransferOwnership_Success(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, err := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)
	admin, err := svc.AddMember(ctx, org.ID, "a@x.com", "A", orgsvc.RoleAdmin)
	require.NoError(t, err)

	require.NoError(t, svc.TransferOwnership(ctx, org.ID, owner.UserID, admin.UserID))

	members, err := svc.ListMembers(ctx, org.ID)
	require.NoError(t, err)
	byID := map[uuid.UUID]string{}
	for _, m := range members {
		byID[m.UserID] = m.Role
	}
	require.Equal(t, orgsvc.RoleAdmin, byID[owner.UserID], "ex-owner queda como admin")
	require.Equal(t, orgsvc.RoleOwner, byID[admin.UserID], "admin promovido a owner")

	var auditCount int
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'organization.ownership_transferred' AND organization_id = $1`,
		org.ID).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
}

func TestOrg_TransferOwnership_TargetMustBeAdminOrMaintainer(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, _ := svc.Create(ctx, "A", "a", "o@x.com", "O")
	viewer, err := svc.AddMember(ctx, org.ID, "v@x.com", "V", orgsvc.RoleViewer)
	require.NoError(t, err)

	err = svc.TransferOwnership(ctx, org.ID, owner.UserID, viewer.UserID)
	require.ErrorIs(t, err, orgsvc.ErrTargetNotMember,
		"transferir a viewer debe rechazar (solo admin/maintainer eligibles)")
}

func TestOrg_TransferOwnership_FromMustBeOwner(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, _, _ := svc.Create(ctx, "A", "a", "o@x.com", "O")
	admin, _ := svc.AddMember(ctx, org.ID, "a@x.com", "A", orgsvc.RoleAdmin)
	target, _ := svc.AddMember(ctx, org.ID, "m@x.com", "M", orgsvc.RoleMaintainer)

	err := svc.TransferOwnership(ctx, org.ID, admin.UserID, target.UserID)
	require.ErrorIs(t, err, orgsvc.ErrNotOwner)
}

// Escenario 5: SoftDelete con confirm slug correcto + audit.
func TestOrg_SoftDelete_Success(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, _ := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, svc.SoftDelete(ctx, org.ID, owner.UserID, "acme"))

	after, err := svc.GetByID(ctx, org.ID)
	require.NoError(t, err)
	require.NotNil(t, after.DeletedAt, "deleted_at debe estar seteado")
}

func TestOrg_SoftDelete_WrongConfirm(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, _ := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	err := svc.SoftDelete(ctx, org.ID, owner.UserID, "WRONG")
	require.ErrorIs(t, err, orgsvc.ErrConfirmMismatch)
}

// Sabotaje: si rollback de creación falla, no debe quedar org sin owner.
func TestSabotage_Create_Atomic_NoOrphans(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()

	// Crear org A, después intentar otra con SAME slug → fail por ErrSlugTaken
	_, _, err := svc.Create(ctx, "First", "alpha", "f@x.com", "F")
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, "Second", "alpha", "s@x.com", "S")
	require.ErrorIs(t, err, orgsvc.ErrSlugTaken)

	// Verificar que NO se creó user 's@x.com' (tx rolled back)
	var count int
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE email = 's@x.com'`).Scan(&count))
	require.Equal(t, 0, count, "owner user NO debe persistir si Create falla")
}

// Sabotaje: SoftDelete es idempotente — segunda llamada no-op sin error.
func TestSabotage_SoftDelete_Idempotent(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	org, owner, _ := svc.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, svc.SoftDelete(ctx, org.ID, owner.UserID, "acme"))
	require.NoError(t, svc.SoftDelete(ctx, org.ID, owner.UserID, "acme"),
		"segunda SoftDelete debe ser no-op idempotente")
}

func TestOrg_GetByID_NotFound(t *testing.T) {
	svc, cleanup := setupOrgSvc(t)
	defer cleanup()
	ctx := context.Background()
	_, err := svc.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, orgsvc.ErrNotFound)
}
