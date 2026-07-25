// Primitivas de autenticación por firma HMAC (DOMAINSERV-129).
//
// Motivación: con `Authorization: Bearer domk_live_...` la credencial viaja en
// claro en cada request; sin TLS cualquier hop la captura y la reusa para
// siempre. Firmando, el cliente prueba que posee la key sin transmitirla, y la
// firma solo vale para ESE método+path+body+ts+nonce.
//
// Header: Authorization: DOMAIN-HMAC-SHA256 key=<prefix>,ts=<unix>,nonce=<rand>,sig=<hex>
//
// Compatibilidad: el secreto de firma se DERIVA del plaintext de la key, que ya
// está recuperable desde auth_api_keys.key_ciphertext (pgp_sym_encrypt,
// migración 000168). No hace falta columna nueva. Las keys emitidas antes de
// 000168 tienen key_ciphertext NULL y NO pueden firmar: bcrypt no se invierte,
// hay que rotarlas.
package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HMACScheme es el scheme del header Authorization de una request firmada.
const HMACScheme = "DOMAIN-HMAC-SHA256"

// hmacSecretLabel separa el dominio de la derivación: el secreto de firma NO es
// la API key, así que filtrarlo no entrega la credencial. El sufijo de versión
// permite rotar el algoritmo sin ambigüedad.
const hmacSecretLabel = "domain-hmac-v1"

// SignatureMaxSkew ventana de tolerancia del ts contra el reloj del server
// (±5 min). Fuera de ella la firma se descarta sin tocar la BD.
const SignatureMaxSkew = 5 * time.Minute

// MaxNonceLen tope del nonce. Acota el tamaño del header y de la fila que se
// inserta en auth_hmac_nonces.
const MaxNonceLen = 128

// sigHexLen largo del hex de un HMAC-SHA256.
const sigHexLen = sha256.Size * 2

// nonceBytes entropía del nonce que genera NewNonce (22 chars en base64url).
const nonceBytes = 16

var (
	// ErrInvalidSignatureHeader header ausente, mal formado o con campos fuera
	// de rango. Nunca se propaga al cliente con detalle (anti-enumeration).
	ErrInvalidSignatureHeader = errors.New("invalid signature header")

	// ErrSignatureExpired el ts está fuera de SignatureMaxSkew.
	ErrSignatureExpired = errors.New("signature timestamp outside window")

	// ErrNonceReplayed el nonce ya fue quemado por esa misma key.
	ErrNonceReplayed = errors.New("nonce already used")

	// ErrKeyNotSignable la key existe pero se emitió antes de 000168
	// (key_ciphertext NULL): no hay plaintext recuperable, hay que rotarla.
	ErrKeyNotSignable = errors.New("api key predates ciphertext storage; rotate it to enable hmac auth")
)

// SigningSecret deriva el secreto de firma desde el plaintext de la API key.
// Determinista (cliente y server llegan al mismo valor sin coordinarse) y de
// una sola vía.
func SigningSecret(plaintext string) []byte {
	mac := hmac.New(sha256.New, []byte(plaintext))
	mac.Write([]byte(hmacSecretLabel))
	return mac.Sum(nil)
}

// SignatureParams son los campos del header Authorization firmado.
type SignatureParams struct {
	KeyPrefix string
	Timestamp int64
	Nonce     string
	Signature string
}

// ParseSignatureHeader parsea el header firmado. Los cuatro campos son
// obligatorios, no se aceptan duplicados y el orden es libre.
func ParseSignatureHeader(header string) (*SignatureParams, error) {
	rest, ok := cutScheme(header)
	if !ok {
		return nil, ErrInvalidSignatureHeader
	}
	fields := map[string]string{}
	for _, part := range strings.Split(rest, ",") {
		k, v, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found || v == "" {
			return nil, ErrInvalidSignatureHeader
		}
		if _, dup := fields[k]; dup {
			return nil, ErrInvalidSignatureHeader
		}
		fields[k] = v
	}
	if len(fields) != 4 {
		return nil, ErrInvalidSignatureHeader
	}
	ts, err := strconv.ParseInt(fields["ts"], 10, 64)
	if err != nil {
		return nil, ErrInvalidSignatureHeader
	}
	p := &SignatureParams{
		KeyPrefix: fields["key"],
		Timestamp: ts,
		Nonce:     fields["nonce"],
		Signature: fields["sig"],
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// validate rechaza campos fuera de contrato antes de tocar la BD.
func (p *SignatureParams) validate() error {
	if _, err := ParsePrefix(p.KeyPrefix); err != nil || len(p.KeyPrefix) != PrefixLen {
		return ErrInvalidSignatureHeader
	}
	if p.Nonce == "" || len(p.Nonce) > MaxNonceLen {
		return ErrInvalidSignatureHeader
	}
	if len(p.Signature) != sigHexLen {
		return ErrInvalidSignatureHeader
	}
	if _, err := hex.DecodeString(p.Signature); err != nil {
		return ErrInvalidSignatureHeader
	}
	return nil
}

// SkewFrom valida el ts contra el reloj del server.
func (p *SignatureParams) SkewFrom(now time.Time) error {
	delta := now.Sub(time.Unix(p.Timestamp, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > SignatureMaxSkew {
		return ErrSignatureExpired
	}
	return nil
}

// cutScheme separa el scheme del resto del header. Case-insensitive porque
// RFC 7235 define el scheme así.
func cutScheme(header string) (string, bool) {
	if len(header) <= len(HMACScheme) {
		return "", false
	}
	if !strings.EqualFold(header[:len(HMACScheme)], HMACScheme) || header[len(HMACScheme)] != ' ' {
		return "", false
	}
	rest := strings.TrimSpace(header[len(HMACScheme)+1:])
	if rest == "" {
		return "", false
	}
	return rest, true
}

// HasHMACScheme true si el header pide autenticación firmada.
func HasHMACScheme(header string) bool {
	_, ok := cutScheme(header)
	return ok
}

// CanonicalRequest arma la cadena a firmar. El orden de los campos y los \n son
// parte del contrato con los SDKs: cambiarlos invalida toda firma emitida.
func CanonicalRequest(method, target, bodySHA256Hex string, ts int64, nonce string) string {
	return strings.ToUpper(method) + "\n" +
		target + "\n" +
		bodySHA256Hex + "\n" +
		strconv.FormatInt(ts, 10) + "\n" +
		nonce
}

// CanonicalTarget path escapado + query cruda, tal cual viajó en la
// request-line. Se usa RawQuery sin re-ordenar: el cliente firma los bytes que
// manda, no una normalización.
func CanonicalTarget(u *url.URL) string {
	target := u.EscapedPath()
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target
}

// BodySHA256 hex del sha256 del body. Un body vacío hashea igual que nil, así
// que GET y DELETE no necesitan caso especial.
func BodySHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Sign firma la cadena canónica con el secreto derivado.
func Sign(secret []byte, canonical string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureMatches recomputa la firma y compara en tiempo constante. NUNCA
// usar == sobre el hex: filtra el prefijo correcto por timing.
func SignatureMatches(secret []byte, canonical, gotHex string) bool {
	got, err := hex.DecodeString(gotHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return hmac.Equal(mac.Sum(nil), got)
}

// NewNonce genera un nonce single-use. base64url no produce ',' ni '=' (raw),
// los dos separadores del header.
func NewNonce() (string, error) {
	buf := make([]byte, nonceBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("nonce rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// BuildSignatureHeader serializa el header firmado. Vive del lado del server
// para tener una sola definición del formato que los tests puedan ejercitar.
func BuildSignatureHeader(keyPrefix string, ts int64, nonce, sigHex string) string {
	return HMACScheme + " key=" + keyPrefix +
		",ts=" + strconv.FormatInt(ts, 10) +
		",nonce=" + nonce +
		",sig=" + sigHex
}
