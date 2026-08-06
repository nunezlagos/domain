//go:build integration

package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	projsvc "nunezlagos/domain/internal/service/project"
	"nunezlagos/domain/internal/service/webhook"
	"nunezlagos/domain/internal/store/txctx"
)

type whFixture struct {
	svc       *webhook.Service
	orgID     uuid.UUID
	projectID uuid.UUID
	user      uuid.UUID
}

// enScope abre una tx con app.current_project_id seteado al proyecto del fixture y devuelve
// el ctx que la lleva, más su cierre.
//
// DOMAINSERV-240: la 000288 puso webhooks bajo RLS por app.current_project_id, y estos tests
// escribían contra un context.Background() pelado. Quedaron ROJOS desde esa migración con
// "new row violates row-level security policy for table webhooks": el service nunca se
// actualizó para escribir project_id ni para setear el GUC. Este es el mismo camino que usa
// producción vía rlsProyecto. El cierre es Rollback y no Commit porque cada test levanta su
// propio container: no hay nada que valga la pena persistir, y un rollback no deja la tx
// colgada reteniendo el lock que después espera el cleanup.
func (f *whFixture) enScope(t *testing.T) (context.Context, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.svc.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('app.current_project_id', $1, true)`, f.projectID.String())
	require.NoError(t, err)
	return txctx.WithTxContext(ctx, tx), func() { _ = tx.Rollback(ctx) }
}

func setupWebhook(t *testing.T) (*whFixture, func()) {
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
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	org, owner, err := seedOrgUser(ctx, pools.App, "WHOrg", "whorg", "o@x.com", "O")
	require.NoError(t, err)
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	proj, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "WHProj", Slug: "whproj", ActorID: owner.UserID,
	})
	require.NoError(t, err)

	key := make([]byte, crypto.MasterKeySize)
	_, err = rand.Read(key)
	require.NoError(t, err)
	cipherInst, err := crypto.NewCipher(key)
	require.NoError(t, err)

	// PoolPublic = pools.Auth (BYPASSRLS) igual que producción: el camino de recepción no
	// tiene proyecto con el que satisfacer el RLS, así que resuelve el slug sin filtro
	svc := &webhook.Service{
		Pool: pools.App, PoolPublic: pools.Auth, Audit: rec, Crypto: cipherInst,
	}
	return &whFixture{svc: svc, orgID: org.ID, projectID: proj.ID, user: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

// enScopeCommit corre fn en scope de proyecto y COMMITEA. Hace falta cuando lo escrito tiene
// que ser visible desde OTRO pool: el endpoint público resuelve por PoolPublic, así que una
// tx sin commitear le resulta invisible y el test mediría lo contrario de lo que cree.
func (f *whFixture) enScopeCommit(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.svc.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('app.current_project_id', $1, true)`, f.projectID.String())
	require.NoError(t, err)

	if err := fn(txctx.WithTxContext(ctx, tx)); err != nil {
		_ = tx.Rollback(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit(ctx))
}

func (f *whFixture) create(t *testing.T, slug, secret string) *webhook.Webhook {
	t.Helper()
	var hook *webhook.Webhook
	f.enScopeCommit(t, func(ctx context.Context) error {
		var err error
		hook, err = f.svc.Create(ctx, webhook.CreateInput{
			OrganizationID: f.orgID, ProjectID: f.projectID, Slug: slug, Name: "Hook " + slug,
			Secret: secret, SourceType: "generic", TargetType: "flow",
			TargetID: uuid.New(), ActorID: f.user,
		})
		return err
	})
	return hook
}

func TestWebhook_CreateResolve_HMACRoundTrip(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()
	ctx := context.Background()

	f.create(t, "ci-hook", "super-secret-token")

	hook, secret, err := f.svc.ResolveBySlug(ctx, "ci-hook")
	require.NoError(t, err)
	require.Equal(t, "ci-hook", hook.Slug)
	require.Equal(t, "super-secret-token", string(secret),
		"secret descifrado debe coincidir con el plaintext original")


	body := []byte(`{"event":"push"}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	require.True(t, webhook.VerifyHMAC(secret, body, sig))
	require.False(t, webhook.VerifyHMAC(secret, body, "sha256=deadbeef"))
	require.False(t, webhook.VerifyHMAC(secret, []byte(`{"tampered":1}`), sig),
		"body alterado debe invalidar la firma")
}

func TestWebhook_Management_NoSecretLeak(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()

	created := f.create(t, "managed", "s3cret")

	// el management va EN SCOPE de proyecto: es lo que cambió la 000288 y lo que este test
	// no reflejaba. Sin el GUC, List devuelve cero filas sin error — la falla silenciosa
	// que el RLS produce y que hacía ver este test como si el alta no funcionara
	ctx, cerrar := f.enScope(t)
	defer cerrar()

	list, err := f.svc.List(ctx, f.orgID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	got, err := f.svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "managed", got.Slug)



	// el toggle se COMMITEA porque ResolveBySlug va por PoolPublic e ignora la tx del
	// contexto a propósito: el camino público no tiene GUC de proyecto que satisfacer, así
	// que no puede leer dentro de la tx del management
	f.enScopeCommit(t, func(scoped context.Context) error {
		return f.svc.SetEnabled(scoped, created.ID, false)
	})
	_, _, err = f.svc.ResolveBySlug(context.Background(), "managed")
	require.ErrorIs(t, err, webhook.ErrNotFound,
		"un webhook deshabilitado no se resuelve: /receive responde como si no existiera")

	f.enScopeCommit(t, func(scoped context.Context) error {
		return f.svc.SetEnabled(scoped, created.ID, true)
	})
	require.NoError(t, f.svc.SoftDelete(ctx, created.ID, f.user))
	_, err = f.svc.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

func TestWebhook_Deliveries_LogAndGet(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()

	hook := f.create(t, "logged", "s")

	ctx, cerrar := f.enScope(t)
	defer cerrar()
	runID := uuid.New()
	require.NoError(t, f.svc.RecordDelivery(ctx, hook.ID,
		[]byte(`{"a":1}`), map[string]string{"X-Test": "1"}, "1.2.3.4",
		"triggered", &runID, ""))
	require.NoError(t, f.svc.RecordDelivery(ctx, hook.ID,
		[]byte(`{"b":2}`), nil, "1.2.3.4", "signature_invalid", nil, "HMAC mismatch"))

	ds, err := f.svc.Deliveries(ctx, hook.ID, 0)
	require.NoError(t, err)
	require.Len(t, ds, 2)
	require.Equal(t, "signature_invalid", ds[0].Status, "más reciente primero")
	require.Equal(t, "HMAC mismatch", ds[0].Error)
	require.Equal(t, "triggered", ds[1].Status)
	require.Equal(t, &runID, ds[1].TriggeredRunID)

	one, err := f.svc.GetDelivery(ctx, ds[0].ID)
	require.NoError(t, err)
	require.Equal(t, ds[0].ID, one.ID)


	got, err := f.svc.GetByID(ctx, hook.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastDeliveryAt)
}

// Sabotaje: secret cifrado at-rest — la fila en BD NUNCA contiene el plaintext.
func TestSabotage_Webhook_SecretEncryptedAtRest(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()
	ctx := context.Background()

	hook := f.create(t, "atrest", "plaintext-leak-check")

	// la query va por el pool de app_admin: con el de la app, el RLS de la 000288 devuelve
	// cero filas y el Scan falla con ErrNoRows. Un sabotaje que no puede leer la fila no
	// prueba nada sobre su contenido
	var raw []byte
	require.NoError(t, f.svc.PoolPublic.QueryRow(ctx,
		`SELECT secret_encrypted FROM webhooks WHERE id = $1`, hook.ID).Scan(&raw))
	require.NotContains(t, string(raw), "plaintext-leak-check",
		"el secret en BD debe estar cifrado, no en claro")
}
