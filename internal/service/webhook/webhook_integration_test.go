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
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	orgsvc "nunezlagos/domain/internal/service/org"
	"nunezlagos/domain/internal/service/webhook"
)

type whFixture struct {
	svc   *webhook.Service
	orgID uuid.UUID
	user  uuid.UUID
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
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, err := orgS.Create(ctx, "WHOrg", "whorg", "o@x.com", "O")
	require.NoError(t, err)

	key := make([]byte, crypto.MasterKeySize)
	_, err = rand.Read(key)
	require.NoError(t, err)
	cipherInst, err := crypto.NewCipher(key)
	require.NoError(t, err)

	svc := &webhook.Service{Pool: pools.App, Audit: rec, Crypto: cipherInst}
	return &whFixture{svc: svc, orgID: org.ID, user: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func (f *whFixture) create(t *testing.T, slug, secret string) *webhook.Webhook {
	t.Helper()
	hook, err := f.svc.Create(context.Background(), webhook.CreateInput{
		OrganizationID: f.orgID, Slug: slug, Name: "Hook " + slug,
		Secret: secret, SourceType: "generic", TargetType: "flow",
		TargetID: uuid.New(), ActorID: f.user,
	})
	require.NoError(t, err)
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

	// Firma válida pasa, firma inválida no
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
	ctx := context.Background()

	created := f.create(t, "managed", "s3cret")

	list, err := f.svc.List(ctx, f.orgID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	got, err := f.svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "managed", got.Slug)
	// El struct Webhook no tiene campo Secret: imposible leakear por GET/List.

	// Disable → receive deja de resolver (anti-enumeration)
	require.NoError(t, f.svc.SetEnabled(ctx, created.ID, false))
	_, _, err = f.svc.ResolveBySlug(ctx, "managed")
	require.ErrorIs(t, err, webhook.ErrNotFound)

	require.NoError(t, f.svc.SetEnabled(ctx, created.ID, true))
	require.NoError(t, f.svc.SoftDelete(ctx, created.ID, f.user))
	_, err = f.svc.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

func TestWebhook_Deliveries_LogAndGet(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()
	ctx := context.Background()

	hook := f.create(t, "logged", "s")
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

	// last_delivery_at actualizado
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

	var raw []byte
	require.NoError(t, f.svc.Pool.QueryRow(ctx,
		`SELECT secret_encrypted FROM webhooks WHERE id = $1`, hook.ID).Scan(&raw))
	require.NotContains(t, string(raw), "plaintext-leak-check",
		"el secret en BD debe estar cifrado, no en claro")
}
