package apikey

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeSignedResolver replica lo que hace PGStore.ResolveSigned pero en memoria:
// matchea por prefix y recomputa la firma con el secreto derivado.
type fakeSignedResolver struct {
	plaintext string
	principal *Principal
	err       error
	calls     int
}

func (f *fakeSignedResolver) ResolveSigned(_ context.Context, prefix, canonical, sigHex string) (*Principal, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	want, err := ParsePrefix(f.plaintext)
	if err != nil || want != prefix {
		return nil, ErrNotFound
	}
	if !SignatureMatches(SigningSecret(f.plaintext), canonical, sigHex) {
		return nil, ErrNotFound
	}
	return f.principal, nil
}

// fakeNonceBurner emula el INSERT ... ON CONFLICT DO NOTHING.
type fakeNonceBurner struct {
	mu    sync.Mutex
	seen  map[string]bool
	calls int
	err   error
}

func newFakeNonceBurner() *fakeNonceBurner {
	return &fakeNonceBurner{seen: map[string]bool{}}
}

func (f *fakeNonceBurner) BurnNonce(_ context.Context, apiKeyID, nonce string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	k := apiKeyID + "|" + nonce
	if f.seen[k] {
		return false, nil
	}
	f.seen[k] = true
	return true, nil
}

// echoBodyHandler devuelve el body que recibió: prueba que el middleware
// repone r.Body después de bufferearlo para hashearlo.
func echoBodyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}

// signedMW arma un middleware con los tres colaboradores del camino firmado.
func signedMW(plaintext string, at time.Time) (*Middleware, *fakeNonceBurner, *fakeSignedResolver) {
	burner := newFakeNonceBurner()
	sr := &fakeSignedResolver{
		plaintext: plaintext,
		principal: &Principal{UserID: "user-1", OrganizationID: "org-1", APIKeyID: "key-1", Role: "owner"},
	}
	m := &Middleware{
		Resolver:       &fakeResolver{},
		SignedResolver: sr,
		NonceBurner:    burner,
		Now:            func() time.Time { return at },
	}
	return m, burner, sr
}

// signReq firma una request in-place con el mismo algoritmo que el server.
func signReq(req *http.Request, plaintext string, body []byte, ts int64, nonce string) {
	prefix, err := ParsePrefix(plaintext)
	if err != nil {
		panic(err)
	}
	canonical := CanonicalRequest(req.Method, CanonicalTarget(req.URL), BodySHA256(body), ts, nonce)
	sig := Sign(SigningSecret(plaintext), canonical)
	req.Header.Set("Authorization", BuildSignatureHeader(prefix, ts, nonce, sig))
}

func TestMiddleware_Wrap_ValidSignature_PropagatesPrincipalAndBody(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, burner, _ := signedMW(pt, now)

	body := []byte(`{"slug":"demo"}`)
	req := httptest.NewRequest("POST", "/api/v1/projects?dry=1", strings.NewReader(string(body)))
	signReq(req, pt, body, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(echoBodyHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, string(body), rec.Body.String(), "el handler tiene que poder leer el body")
	require.Equal(t, 1, burner.calls, "el nonce se quema exactamente una vez")
}

func TestMiddleware_Wrap_ValidSignatureNoBody_Authenticates(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "user-1", rec.Header().Get("X-User-Id"))
}

func TestMiddleware_Wrap_TamperedBody_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, burner, _ := signedMW(pt, now)

	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{"slug":"evil"}`))
	// se firma otro body: el hash canónico no coincide
	signReq(req, pt, []byte(`{"slug":"demo"}`), now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(echoBodyHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Zero(t, burner.calls, "firma inválida NO debe quemar nonce (anti-flood)")
}

func TestMiddleware_Wrap_TamperedPath_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")
	req.URL.Path = "/api/v1/secrets"

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_TamperedQuery_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)

	req := httptest.NewRequest("GET", "/api/v1/projects?limit=1", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")
	req.URL.RawQuery = "limit=1000"

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_TamperedMethod_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")
	req.Method = "DELETE"

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_TimestampOutsideWindow_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	for name, skew := range map[string]time.Duration{
		"muy viejo":  -(SignatureMaxSkew + time.Second),
		"muy futuro": SignatureMaxSkew + time.Second,
	} {
		t.Run(name, func(t *testing.T) {
			m, burner, sr := signedMW(pt, now)
			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			signReq(req, pt, nil, now.Add(skew).Unix(), "nonce-1")

			rec := httptest.NewRecorder()
			m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.Zero(t, sr.calls, "el ts se valida antes de tocar la BD")
			require.Zero(t, burner.calls)
		})
	}
}

func TestMiddleware_Wrap_TimestampAtWindowEdge_Authenticates(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Add(-SignatureMaxSkew).Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_Wrap_ReplayedNonce_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, burner, _ := signedMW(pt, now)

	build := func() *http.Request {
		req := httptest.NewRequest("GET", "/api/v1/projects", nil)
		signReq(req, pt, nil, now.Unix(), "nonce-replay")
		return req
	}

	first := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(first, build())
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(second, build())
	require.Equal(t, http.StatusUnauthorized, second.Code, "replay exacto rechazado")
	require.Equal(t, 2, burner.calls)
}

func TestMiddleware_Wrap_NonceBurnerMissing_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)
	m.NonceBurner = nil

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code,
		"sin store de nonces el anti-replay no existe: fail closed")
}

func TestMiddleware_Wrap_NonceBurnerErrors_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, burner, _ := signedMW(pt, now)
	burner.err = errors.New("DB down")

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_SignedResolverMissing_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)
	m.SignedResolver = nil

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_BodyOverLimit_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, sr := signedMW(pt, now)
	m.MaxSignedBodyBytes = 8

	body := strings.Repeat("x", 64)
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	signReq(req, pt, []byte(body), now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(echoBodyHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Zero(t, sr.calls, "un body gigante se corta antes de firmar/consultar")
}

func TestMiddleware_Wrap_RequireSignatureWithBearer_Returns401(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	m := &Middleware{
		Resolver:         &fakeResolver{expected: pt, principal: &Principal{UserID: "u", OrganizationID: "o"}},
		RequireSignature: true,
	}
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+pt)

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_Wrap_RequireSignatureWithSignedRequest_Authenticates(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, _ := signedMW(pt, now)
	m.RequireSignature = true

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")

	rec := httptest.NewRecorder()
	m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_Wrap_RequireSignatureOnAllowlistedPath_SkipsAuth(t *testing.T) {
	m := &Middleware{Resolver: &fakeResolver{}, RequireSignature: true, Allowlist: []string{"/health"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_Wrap_SignedFailure_LogsAuthFailureWithoutSecrets(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	m, _, sr := signedMW(pt, now)
	fl := &fakeFailureLogger{}
	m.FailureLogger = fl
	sr.err = errors.New("DB down")

	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	signReq(req, pt, nil, now.Unix(), "nonce-1")
	req.Header.Set("User-Agent", "curl/8.0")

	m.Wrap(nextEchoHandler()).ServeHTTP(httptest.NewRecorder(), req)
	require.Len(t, fl.calls, 1)
	require.Contains(t, fl.calls[0].reason, "/api/v1/foo")
	// ni la key ni la firma pueden aparecer en el audit trail
	require.NotContains(t, fl.calls[0].reason, pt)
	require.NotContains(t, fl.calls[0].reason, req.Header.Get("Authorization"))
}

// Sabotaje: el 401 del camino firmado es indistinguible del 401 del Bearer,
// así no se puede enumerar prefixes ni descubrir qué chequeo falló.
func TestSabotage_SignedFailures_401BodyUniform(t *testing.T) {
	pt, _, _ := GeneratePlaintext("live")
	now := time.Unix(1750000000, 0)
	sig := strings.Repeat("ab", 32)

	headers := []string{
		HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=n,sig=" + sig,
		HMACScheme + " garbage",
		HMACScheme + " key=domk_live_aaaaaa,ts=" + time.Unix(1750000000, 0).Format("2006") + ",nonce=n,sig=" + sig,
	}
	for _, h := range headers {
		m, _, _ := signedMW(pt, now)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/x", nil)
		req.Header.Set("Authorization", h)
		m.Wrap(nextEchoHandler()).ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		body := rec.Body.String()
		require.Contains(t, body, "unauthorized")
		require.NotContains(t, body, "nonce")
		require.NotContains(t, body, "signature")
		require.NotContains(t, body, "key")
	}
}
