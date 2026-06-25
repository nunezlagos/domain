//go:build integration

package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"


	"github.com/google/uuid"
)

func b64key(t *testing.T) (string, []byte) {
	t.Helper()
	k := make([]byte, crypto.MasterKeySize)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(k), k
}

// Rotation end-to-end: secrets cifrados con v1 → keyring v1+v2 →
// ReEncryptAll los pasa a v2 y siguen legibles.
func TestReEncryptAll_RotatesToCurrentVersion(t *testing.T) {
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
	defer func() { _ = pgC.Terminate(ctx) }()
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	defer pools.Close()

	org, owner, err := seedOrgUser(ctx, pools.App, "SecOrg", "secorg", "s@x.com", "S")
	require.NoError(t, err)

	b641, _ := b64key(t)
	b642, _ := b64key(t)


	c1, err := crypto.LoadKeyring("1:" + b641)
	require.NoError(t, err)
	storeV1 := &PGStore{Pool: pools.Auth, Cipher: c1}

	var ids []uuid.UUID
	for _, slug := range []string{"api-token", "webhook-secret"} {
		sec, err := storeV1.Create(ctx, CreateInput{
			OrganizationID: org.ID, Slug: slug, Name: slug,
			Value: "valor-" + slug, CreatedBy: &owner.UserID,
		})
		require.NoError(t, err)
		require.Equal(t, 1, sec.EncryptionKeyVer)
		ids = append(ids, sec.ID)
	}


	c2, err := crypto.LoadKeyring("1:" + b641 + ",2:" + b642)
	require.NoError(t, err)
	storeV2 := &PGStore{Pool: pools.Auth, Cipher: c2}

	n, err := storeV2.ReEncryptAll(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)


	n, err = storeV2.ReEncryptAll(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, n)


	for i, id := range ids {
		sec, err := storeV2.GetByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, 2, sec.EncryptionKeyVer, "secret %d debe estar en v2", i)
		val, err := storeV2.GetValue(ctx, id)
		require.NoError(t, err)
		require.Contains(t, val, "valor-")
	}



	c3, err := crypto.LoadKeyring("2:" + b642)
	require.NoError(t, err)
	storeOnlyV2 := &PGStore{Pool: pools.Auth, Cipher: c3}
	val, err := storeOnlyV2.GetValue(ctx, ids[0])
	require.NoError(t, err, "post-rotation el keyring puede descartar la key vieja")
	require.Equal(t, "valor-api-token", val)
}
