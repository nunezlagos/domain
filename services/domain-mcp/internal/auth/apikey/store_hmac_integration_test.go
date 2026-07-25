//go:build integration

package apikey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// signedFor arma la cadena canónica + firma para una key en claro.
func signedFor(plaintext, method, target string, body []byte, ts int64, nonce string) (canonical, sig string) {
	canonical = CanonicalRequest(method, target, BodySHA256(body), ts, nonce)
	return canonical, Sign(SigningSecret(plaintext), canonical)
}

func TestStore_ResolveSigned_ValidSignature_ReturnsPrincipal(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	prefix, err := ParsePrefix(plaintext)
	require.NoError(t, err)

	canonical, sig := signedFor(plaintext, "POST", "/api/v1/projects", []byte(`{"slug":"demo"}`), 1750000000, "n-1")
	p, err := s.ResolveSigned(ctx, prefix, canonical, sig)
	require.NoError(t, err)
	require.Equal(t, userID.String(), p.UserID)
	require.Equal(t, keyID.String(), p.APIKeyID)
	require.Equal(t, canonicalOrgID.String(), p.OrganizationID)
	require.Equal(t, "owner", p.Role)
}

func TestStore_ResolveSigned_TamperedCanonical_ReturnsNotFound(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, _, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	prefix, err := ParsePrefix(plaintext)
	require.NoError(t, err)

	_, sig := signedFor(plaintext, "GET", "/api/v1/projects", nil, 1750000000, "n-1")
	other := CanonicalRequest("GET", "/api/v1/secrets", BodySHA256(nil), 1750000000, "n-1")
	_, err = s.ResolveSigned(ctx, prefix, other, sig)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestStore_ResolveSigned_RevokedKey_ReturnsNotFound(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	require.NoError(t, s.Revoke(ctx, keyID))
	prefix, err := ParsePrefix(plaintext)
	require.NoError(t, err)

	canonical, sig := signedFor(plaintext, "GET", "/api/v1/projects", nil, 1750000000, "n-1")
	_, err = s.ResolveSigned(ctx, prefix, canonical, sig)
	require.ErrorIs(t, err, ErrNotFound)
}

// Las keys emitidas antes de 000168 tienen key_ciphertext NULL: no hay plaintext
// recuperable (bcrypt no se invierte) y NO pueden firmar. El store lo reporta
// distinto de "no existe" para que el operador sepa que hay que rotarlas.
func TestStore_ResolveSigned_KeyWithoutCiphertext_ReturnsKeyNotSignable(t *testing.T) {
	s, _, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, prefix, hash, err := Generate("live")
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx,
		`INSERT INTO auth_api_keys (user_id, key_hash, key_prefix, name) VALUES ($1,$2,$3,$4)`,
		userID, hash, prefix, "legacy")
	require.NoError(t, err)

	canonical, sig := signedFor(plaintext, "GET", "/api/v1/projects", nil, 1750000000, "n-1")
	_, err = s.ResolveSigned(ctx, prefix, canonical, sig)
	require.ErrorIs(t, err, ErrKeyNotSignable)
}

// El prefix NO es único: dos keys pueden compartirlo y el store tiene que
// iterar hasta encontrar la que reproduce la firma.
func TestStore_ResolveSigned_PrefixCollision_PicksTheSigningKey(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	first, firstID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	prefix, err := ParsePrefix(first)
	require.NoError(t, err)

	second, secondID, err := s.Issue(ctx, orgID, userID, "k2", "live")
	require.NoError(t, err)
	// se fuerza la colisión: misma columna key_prefix para las dos keys
	_, err = s.Pool.Exec(ctx, `UPDATE auth_api_keys SET key_prefix = $1 WHERE id = $2`, prefix, secondID)
	require.NoError(t, err)

	canonical, sig := signedFor(second, "GET", "/api/v1/projects", nil, 1750000000, "n-1")
	p, err := s.ResolveSigned(ctx, prefix, canonical, sig)
	require.NoError(t, err)
	require.Equal(t, secondID.String(), p.APIKeyID)
	require.NotEqual(t, firstID.String(), p.APIKeyID)
}

func TestStore_BurnNonce_FirstUse_ReturnsFreshThenReplay(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	_, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	now := time.Now()

	fresh, err := s.BurnNonce(ctx, keyID.String(), "n-1", now)
	require.NoError(t, err)
	require.True(t, fresh)

	fresh, err = s.BurnNonce(ctx, keyID.String(), "n-1", now)
	require.NoError(t, err)
	require.False(t, fresh, "el segundo intento con el mismo nonce es replay")

	fresh, err = s.BurnNonce(ctx, keyID.String(), "n-2", now)
	require.NoError(t, err)
	require.True(t, fresh, "otro nonce de la misma key sigue siendo válido")
}

func TestStore_BurnNonce_UnknownAPIKey_ReturnsError(t *testing.T) {
	s, _, _, cleanup := setupKeyStore(t)
	defer cleanup()

	_, err := s.BurnNonce(context.Background(), uuid.NewString(), "n-1", time.Now())
	require.Error(t, err, "la FK a auth_api_keys evita quemar nonces de keys inexistentes")
}

// La tabla no tiene cron de limpieza: el propio statement de quemado poda los
// nonces fuera de la ventana, si no crece sin techo.
func TestStore_BurnNonce_StaleNonces_ArePrunedByNextBurn(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	_, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)

	stale := time.Now().Add(-time.Hour)
	fresh, err := s.BurnNonce(ctx, keyID.String(), "viejo", stale)
	require.NoError(t, err)
	require.True(t, fresh)

	_, err = s.BurnNonce(ctx, keyID.String(), "nuevo", time.Now())
	require.NoError(t, err)

	var n int
	require.NoError(t, s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_hmac_nonces WHERE nonce = 'viejo'`).Scan(&n))
	require.Zero(t, n, "el nonce fuera de ventana se podó")

	require.NoError(t, s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_hmac_nonces WHERE nonce = 'nuevo'`).Scan(&n))
	require.Equal(t, 1, n)
}

// El middleware completo contra BD real: firma válida entra, replay exacto no.
func TestMiddleware_Wrap_SignedAgainstRealStore_RejectsReplay(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, _, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)

	now := time.Now()
	m := &Middleware{SignedResolver: s, NonceBurner: s, Now: func() time.Time { return now }}
	build := func() *http.Request {
		req := httptest.NewRequest("GET", "/api/v1/projects", nil)
		signReq(req, plaintext, nil, now.Unix(), "nonce-real")
		return req
	}

	first := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(first, build())
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(second, build())
	require.Equal(t, http.StatusUnauthorized, second.Code)
}
