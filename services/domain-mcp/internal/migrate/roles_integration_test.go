//go:build integration



package migrate_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	dmigrate "nunezlagos/domain/internal/migrate"
)

// helper: setea password a un role para poder loguearse en tests.
func setRolePassword(t *testing.T, adminDSN, role, password string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN)
	require.NoError(t, err)
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx,
		"ALTER ROLE "+role+" WITH LOGIN PASSWORD '"+password+"'")
	require.NoError(t, err)
}

// connect via specific role.
func connectAs(t *testing.T, adminDSN, role, password string) *pgx.Conn {
	t.Helper()


	at := strings.Index(adminDSN, "@")
	require.GreaterOrEqual(t, at, 0)
	after := adminDSN[at:]
	prefix := "postgres://"
	dsn := prefix + role + ":" + password + after
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err)
	return conn
}

// Escenario 1: 4 roles existen post-migrate.
func TestRoles_AllFourExist(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	expected := []string{"app_user", "app_admin", "app_migrator", "app_readonly"}
	for _, role := range expected {
		var exists bool
		err := conn.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role,
		).Scan(&exists)
		require.NoError(t, err)
		require.Truef(t, exists, "role %s no existe", role)
	}
}

// Escenario 2: app_user NO puede DDL (CREATE TABLE).
func TestRoles_AppUser_CannotDDL(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	setRolePassword(t, dsn, "app_user", "testpass")

	conn := connectAs(t, dsn, "app_user", "testpass")
	defer conn.Close(context.Background())

	_, err := conn.Exec(context.Background(),
		"CREATE TABLE evil_foo (id INT)")
	require.Error(t, err, "app_user no debe poder CREATE TABLE")
	require.Contains(t, err.Error(), "permission denied")
}

// Escenario 3: app_user NO puede UPDATE audit_log.
func TestRoles_AppUser_CannotUpdateAuditLog(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	setRolePassword(t, dsn, "app_user", "testpass")


	adminConn, _ := pgx.Connect(context.Background(), dsn)
	defer adminConn.Close(context.Background())
	_, err := adminConn.Exec(context.Background(), `
		INSERT INTO audit_log (actor_id, actor_type, action, entity_type)
		VALUES ('00000000-0000-0000-0000-000000000001', 'system', 'test', 'test')
	`)
	require.NoError(t, err)

	conn := connectAs(t, dsn, "app_user", "testpass")
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(),
		"UPDATE audit_log SET action='hidden' WHERE 1=1")
	require.Error(t, err, "app_user UPDATE audit_log debe fallar")
}

// Escenario 4: app_user PUEDE INSERT en audit_log.
func TestRoles_AppUser_CanInsertAuditLog(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	setRolePassword(t, dsn, "app_user", "testpass")

	conn := connectAs(t, dsn, "app_user", "testpass")
	defer conn.Close(context.Background())

	_, err := conn.Exec(context.Background(), `
		INSERT INTO audit_log (actor_id, actor_type, action, entity_type)
		VALUES ('00000000-0000-0000-0000-000000000002', 'user', 'login', 'session')
	`)
	require.NoError(t, err, "app_user INSERT audit_log debe funcionar")
}

// Escenario 5: app_readonly solo SELECT.
func TestRoles_AppReadonly_OnlySelect(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	setRolePassword(t, dsn, "app_readonly", "testpass")

	conn := connectAs(t, dsn, "app_readonly", "testpass")
	defer conn.Close(context.Background())


	_, err := conn.Exec(context.Background(),
		"SELECT count(*) FROM organizations")
	require.NoError(t, err)


	_, err = conn.Exec(context.Background(),
		"INSERT INTO organizations (name, slug) VALUES ('X', 'x')")
	require.Error(t, err, "app_readonly INSERT debe fallar")
}

// Escenario 6: PUBLIC role NO puede CREATE en public schema.
func TestRoles_Public_CannotCreateInPublic(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))


	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	_, _ = conn.Exec(ctx, "CREATE ROLE guest WITH LOGIN PASSWORD 'guest'")
	_, _ = conn.Exec(ctx, "GRANT CONNECT ON DATABASE test TO guest")
	conn.Close(ctx)

	gc := connectAs(t, dsn, "guest", "guest")
	defer gc.Close(ctx)

	_, err = gc.Exec(ctx, "CREATE TABLE guest_table (id INT)")
	require.Error(t, err, "PUBLIC no debería poder CREATE en public schema")
}

// Sabotaje: TRUNCATE app_user → denegado.
func TestSabotage_AppUser_TruncateDenied(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))
	setRolePassword(t, dsn, "app_user", "testpass")

	conn := connectAs(t, dsn, "app_user", "testpass")
	defer conn.Close(context.Background())

	_, err := conn.Exec(context.Background(), "TRUNCATE TABLE organizations")
	require.Error(t, err, "TRUNCATE requiere TRUNCATE privilege que app_user no tiene")
}
