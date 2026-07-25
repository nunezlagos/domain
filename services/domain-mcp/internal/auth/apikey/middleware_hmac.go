package apikey

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// DefaultMaxSignedBodyBytes tope del body que se bufferea para hashear una
// request firmada. /api/ solo recibe JSON (los uploads van a S3 presigned), así
// que 5 MiB sobra y acota el costo de una request firmada hostil.
const DefaultMaxSignedBodyBytes int64 = 5 << 20

// SignedResolver resuelve una request firmada: busca los candidatos por prefix
// —que NO es único, PrefixLen=16 y "domk_live_" ya come 10 chars—, recompone el
// secreto de cada uno y recomputa la firma. Devuelve el Principal del que
// matchea. Interfaz definida en el consumidor; la implementa PGStore.
type SignedResolver interface {
	ResolveSigned(ctx context.Context, keyPrefix, canonical, sigHex string) (*Principal, error)
}

// NonceBurner marca un nonce como usado de forma atómica. fresh=false si ese
// par (key, nonce) ya se había quemado, es decir, es un replay.
type NonceBurner interface {
	BurnNonce(ctx context.Context, apiKeyID, nonce string, signedAt time.Time) (fresh bool, err error)
}

// authenticateSigned valida el header `DOMAIN-HMAC-SHA256 ...`.
//
// Orden crítico: ts → firma → nonce. El nonce se quema DESPUÉS de validar la
// firma (si se quemara antes, cualquiera sin credencial inundaría la tabla) y
// ANTES de invocar el handler (si no, dos requests concurrentes con el mismo
// nonce pasarían las dos).
func (m *Middleware) authenticateSigned(w http.ResponseWriter, r *http.Request) (*http.Request, *Principal, bool) {
	if m.SignedResolver == nil || m.NonceBurner == nil {
		// sin resolver o sin store de nonces no hay anti-replay: fail closed
		m.logAPIKeyFailure(r, "signature_unsupported")
		return r, nil, false
	}
	params, err := ParseSignatureHeader(r.Header.Get("Authorization"))
	if err != nil {
		m.logAPIKeyFailure(r, "invalid_signature_format")
		return r, nil, false
	}
	if err := params.SkewFrom(m.now()); err != nil {
		m.logAPIKeyFailure(r, "signature_expired")
		return r, nil, false
	}
	body, err := m.readSignedBody(w, r)
	if err != nil {
		m.logAPIKeyFailure(r, "signed_body_too_large")
		return r, nil, false
	}
	canonical := CanonicalRequest(r.Method, CanonicalTarget(r.URL), BodySHA256(body),
		params.Timestamp, params.Nonce)
	p, err := m.SignedResolver.ResolveSigned(r.Context(), params.KeyPrefix, canonical, params.Signature)
	if err != nil || p == nil {
		m.logAPIKeyFailure(r, "invalid_credentials")
		return r, nil, false
	}
	if !m.burnNonce(r, p, params) {
		return r, nil, false
	}
	return r, p, true
}

// burnNonce quema el nonce ya validado. Cualquier fallo del store se trata como
// rechazo: sin anti-replay garantizado no se deja pasar la request.
func (m *Middleware) burnNonce(r *http.Request, p *Principal, params *SignatureParams) bool {
	fresh, err := m.NonceBurner.BurnNonce(r.Context(), p.APIKeyID, params.Nonce,
		time.Unix(params.Timestamp, 0))
	if err != nil {
		m.logAPIKeyFailure(r, "nonce_store_error")
		return false
	}
	if !fresh {
		m.logAPIKeyFailure(r, "nonce_replayed")
		return false
	}
	return true
}

// readSignedBody bufferea el body para poder hashearlo y lo repone para el
// handler. Este middleware es el primero de la cadena que puede ver el body,
// así que consumirlo acá es seguro; MaxBytesReader corta un body infinito.
func (m *Middleware) readSignedBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, m.maxSignedBody()))
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return body, nil
}

func (m *Middleware) maxSignedBody() int64 {
	if m.MaxSignedBodyBytes > 0 {
		return m.MaxSignedBodyBytes
	}
	return DefaultMaxSignedBodyBytes
}

// now reloj inyectable: los tests de la ventana temporal necesitan fijarlo.
func (m *Middleware) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}
