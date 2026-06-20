// Package pii — issue-02.5 PII redaction.
//
// Detecta y redacta PII común en strings: email, RUT chileno, número telefónico,
// API key Domain, números tarjeta crédito básicos, bearer tokens.
// Útil para responses HTTP de error, logs cuando user content escapa, exports.
//
// Regex-based (no AST); más rápido pero menos preciso que detection NER.
// Para logs estructurados, preferir keys whitelist + slog (issue-17.3).
package pii

import (
	"regexp"
	"strings"
)

// Patterns regex compilados (orden importa: más específicos primero).
var (
	// Email común — RFC 5322 simplified, suficiente para PII detection.
	emailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)

	// RUT chileno con guión OBLIGATORIO para evitar matchear UUIDs/IDs random.
	// Acepta: NN.NNN.NNN-X | NNNNNNNN-X (con guión) | N.NNN.NNN-X (RUT corto).
	rutRe = regexp.MustCompile(`\b(?:\d{1,2}\.\d{3}\.\d{3}-[\dkK]|\d{7,8}-[\dkK])\b`)

	// API key Domain: domk_{env}_{43chars b64url}
	apiKeyRe = regexp.MustCompile(`\bdomk_(?:live|test|dev)_[A-Za-z0-9_\-]{20,}\b`)

	// Bearer token genérico en header.
	bearerRe = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{20,}`)

	// Phone Chile: +56 9 NNNN NNNN | +569NNNNNNNN | etc.
	phoneRe = regexp.MustCompile(`\+?56\s?9\s?\d{4}\s?\d{4}`)

	// Tarjeta crédito básico (4 grupos de 4 dígitos, opcional separadores).
	// NO valida Luhn — solo pattern superficial.
	creditCardRe = regexp.MustCompile(`\b(?:\d{4}[\s\-]?){3}\d{4}\b`)
)

// Redact reemplaza PII detectada por placeholders.
// Reemplazos:
//   email      → [EMAIL]
//   rut        → [RUT]
//   api_key    → [API_KEY]
//   bearer     → Bearer [TOKEN]
//   phone      → [PHONE]
//   cc         → [CC]
func Redact(s string) string {
	if s == "" {
		return s
	}
	// Orden: más específico primero (api key tiene "domk_" prefix antes que rutas
	// genéricas)
	s = apiKeyRe.ReplaceAllString(s, "[API_KEY]")
	s = bearerRe.ReplaceAllString(s, "Bearer [TOKEN]")
	s = emailRe.ReplaceAllString(s, "[EMAIL]")
	s = rutRe.ReplaceAllString(s, "[RUT]")
	s = phoneRe.ReplaceAllString(s, "[PHONE]")
	s = creditCardRe.ReplaceAllString(s, "[CC]")
	return s
}

// ContainsPII true si Redact() cambiaría el string.
func ContainsPII(s string) bool {
	return apiKeyRe.MatchString(s) ||
		bearerRe.MatchString(s) ||
		emailRe.MatchString(s) ||
		rutRe.MatchString(s) ||
		phoneRe.MatchString(s) ||
		creditCardRe.MatchString(s)
}

// RedactMap aplica Redact a todos los string values de un map (shallow).
// Útil para sanitize HTTP query params o headers map.
func RedactMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = Redact(v)
	}
	return out
}

// RedactHeader sanitiza un http.Header-like map[string][]string in-place safe.
func RedactHeader(headers map[string][]string) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		// Headers sensibles enteramente redacted (no aplicar regex, valor entero es secret).
		lowKey := strings.ToLower(k)
		if isSensitiveHeader(lowKey) {
			out[k] = []string{"[REDACTED]"}
			continue
		}
		clean := make([]string, len(vals))
		for i, v := range vals {
			clean[i] = Redact(v)
		}
		out[k] = clean
	}
	return out
}

// sensitiveHeaders headers que siempre se redact enteros (no parcial).
var sensitiveHeaders = map[string]bool{
	"authorization":   true,
	"cookie":          true,
	"set-cookie":      true,
	"x-api-key":       true,
	"x-auth-token":    true,
	"proxy-authorization": true,
}

func isSensitiveHeader(lowerName string) bool {
	return sensitiveHeaders[lowerName]
}
