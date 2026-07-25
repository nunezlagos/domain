package domain

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// envelope refleja el shape estándar de respuesta {"data": ..., "pagination": ...}.
type envelope struct {
	Data       json.RawMessage `json:"data"`
	Pagination *Pagination     `json:"pagination,omitempty"`
}

type apiErrorBody struct {
	Error struct {
		Code      string        `json:"code"`
		Message   string        `json:"message"`
		RequestID string        `json:"request_id"`
		Details   []ErrorDetail `json:"details"`
	} `json:"error"`
}

// do ejecuta la request, agrega auth + headers, decodifica el envelope y lo
// escribe en out (si no es nil). Devuelve la *Pagination cuando el response
// la trae (típico de endpoints de listing).
//
// path debe ser relativo a /api/v1 (ej: "/projects", no "/api/v1/projects").
// query es opcional y puede ser nil.
func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
) (*Pagination, error) {
	u := c.baseURL + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	var rawBody []byte
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		rawBody = buf
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if err := c.setAuthHeader(req, rawBody); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()


	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if len(raw) == 0 || out == nil {
			return nil, nil
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return nil, fmt.Errorf("decode data: %w", err)
			}
			return env.Pagination, nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return nil, fmt.Errorf("decode body: %w", err)
		}
		return nil, nil
	}

	return nil, parseAPIError(resp, raw)
}

// Contrato de firma del server (internal/auth/apikey/hmac.go). Está duplicado a
// propósito: el SDK es otro módulo Go y no puede importar internal/. Golden
// vector del contrato, si algo de esto cambia hay que revalidarlo:
//
//	key=domk_live_TESTKEYTESTKEY, POST /api/v1/projects, body {"slug":"demo"},
//	ts=1750000000, nonce=n-1
//	→ sig=d2ca225062b3783c9829c05bdf9f90847412cfcc27a1f4d8e7f8f90991dc5e91
const (
	hmacScheme      = "DOMAIN-HMAC-SHA256"
	hmacSecretLabel = "domain-hmac-v1"
	hmacPrefixLen   = 16
	hmacNonceBytes  = 16
)

// hmacSigningEnabled se resuelve una sola vez al cargar el package: con firma la
// API key NO viaja en el header, solo su prefix y una prueba de posesión válida
// para ese método+path+body+ts+nonce.
//
// El knob idiomático sería una Option domain.WithHMACSigning(); por ahora es
// env-only para no tocar client.go.
var hmacSigningEnabled = envBool("DOMAIN_HMAC_SIGNING")

func envBool(key string) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && b
}

// setAuthHeader elige el scheme: firma si está habilitada, Bearer si no.
func (c *Client) setAuthHeader(req *http.Request, rawBody []byte) error {
	if !hmacSigningEnabled {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		return nil
	}
	return signRequest(req, c.apiKey, rawBody, time.Now().Unix())
}

// signRequest firma method + path?query + sha256(body) + ts + nonce con el
// secreto derivado de la API key y escribe el header Authorization.
func signRequest(req *http.Request, apiKey string, rawBody []byte, ts int64) error {
	if len(apiKey) < hmacPrefixLen {
		return fmt.Errorf("domain: api key shorter than %d chars, cannot sign", hmacPrefixLen)
	}
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	body := sha256.Sum256(rawBody)
	canonical := strings.ToUpper(req.Method) + "\n" +
		canonicalTarget(req.URL) + "\n" +
		hex.EncodeToString(body[:]) + "\n" +
		strconv.FormatInt(ts, 10) + "\n" +
		nonce
	mac := hmac.New(sha256.New, signingSecret(apiKey))
	mac.Write([]byte(canonical))
	req.Header.Set("Authorization", hmacScheme+
		" key="+apiKey[:hmacPrefixLen]+
		",ts="+strconv.FormatInt(ts, 10)+
		",nonce="+nonce+
		",sig="+hex.EncodeToString(mac.Sum(nil)))
	return nil
}

// signingSecret deriva el secreto de firma: HMAC-SHA256 keyed por la API key
// sobre la etiqueta de versión. El server hace exactamente lo mismo.
func signingSecret(apiKey string) []byte {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(hmacSecretLabel))
	return mac.Sum(nil)
}

// canonicalTarget path escapado + query cruda, tal cual viaja en la
// request-line: se firman los bytes que se mandan, no una normalización.
func canonicalTarget(u *url.URL) string {
	target := u.EscapedPath()
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target
}

// newNonce nonce single-use. base64url raw no emite ',' ni '=', los dos
// separadores del header.
func newNonce() (string, error) {
	buf := make([]byte, hmacNonceBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("domain: nonce rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func parseAPIError(resp *http.Response, raw []byte) error {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	var body apiErrorBody
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	apiErr.Code = body.Error.Code
	apiErr.Message = body.Error.Message
	apiErr.Details = body.Error.Details
	if body.Error.RequestID != "" {
		apiErr.RequestID = body.Error.RequestID
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		apiErr.sentinel = ErrUnauthorized
	case http.StatusNotFound:
		apiErr.sentinel = ErrNotFound
	case http.StatusConflict:
		apiErr.sentinel = ErrConflict
	case http.StatusUnprocessableEntity:
		apiErr.sentinel = ErrValidation
	case http.StatusPaymentRequired:
		apiErr.sentinel = ErrQuota
	case http.StatusTooManyRequests:
		apiErr.sentinel = ErrRateLimited
		apiErr.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	}

	return apiErr
}

func parseRetryAfter(h string) int {
	if h == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
