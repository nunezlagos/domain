// Package observability: este archivo cubre la categorizacion y el
// fingerprint de errores para early-error-reporting.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Category es la clasificacion de un error. Extensible via tabla known_errors.
type Category string

const (
	CategorySQL        Category = "SQL_ERROR"
	CategoryAuth       Category = "AUTH_ERROR"
	CategoryValidation Category = "VALIDATION_ERROR"
	CategoryTimeout    Category = "TIMEOUT"
	CategoryPanic      Category = "PANIC"
	CategoryExternal   Category = "EXTERNAL_SERVICE_ERROR"
	CategoryRateLimit  Category = "RATE_LIMIT_EXCEEDED"
	CategoryUnknown    Category = "UNKNOWN"
)

// categoryMatchers se evaluan en orden: del mas especifico al mas generico.
// VALIDATION_ERROR va ultimo porque su patron ("invalid") es el mas amplio.
var categoryMatchers = []struct {
	re  *regexp.Regexp
	cat Category
}{
	{regexp.MustCompile(`panic:|runtime error|nil pointer`), CategoryPanic},
	{regexp.MustCompile(`timeout|deadline exceeded|context deadline`), CategoryTimeout},
	{regexp.MustCompile(`rate limit|too many requests|\b429\b`), CategoryRateLimit},
	{regexp.MustCompile(`unauthorized|forbidden|invalid token|authentication|permission denied`), CategoryAuth},
	{regexp.MustCompile(`connection refused|no such host|dial tcp|\b50[23]\b|bad gateway`), CategoryExternal},
	{regexp.MustCompile(`sqlstate|relation .* does not exist|column .* does not exist|syntax error at`), CategorySQL},
	{regexp.MustCompile(`validation|invalid|required|malformed|out of range`), CategoryValidation},
}

// Categorize clasifica un error. Los pgconn.PgError siempre son SQL_ERROR
// (cualquier SQLSTATE). El resto se resuelve por regexp sobre el mensaje.
func Categorize(err error) Category {
	if err == nil {
		return ""
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return CategorySQL
	}
	msg := strings.ToLower(err.Error())
	for _, m := range categoryMatchers {
		if m.re.MatchString(msg) {
			return m.cat
		}
	}
	return CategoryUnknown
}

// volatileTokens colapsa secuencias de digitos (ids, timestamps, ports) que
// cambian entre ocurrencias del mismo error logico.
var volatileTokens = regexp.MustCompile(`\d+`)

// normalizeMessage estabiliza el mensaje para que el mismo error logico
// produzca el mismo fingerprint pese a ids/timestamps distintos.
func normalizeMessage(msg string) string {
	out := strings.ToLower(strings.TrimSpace(msg))
	out = volatileTokens.ReplaceAllString(out, "#")
	return strings.Join(strings.Fields(out), " ")
}

// FirstStackLine devuelve la primera linea con contenido del stack trace,
// usada como parte estable del fingerprint.
func FirstStackLine(stack string) string {
	for _, line := range strings.Split(stack, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}

// Fingerprint identifica un error logico de forma estable:
// sha256(category | normalized_message | source | first_stack_line).
func Fingerprint(cat Category, message, source, stack string) []byte {
	h := sha256.New()
	h.Write([]byte(cat))
	h.Write([]byte{'|'})
	h.Write([]byte(normalizeMessage(message)))
	h.Write([]byte{'|'})
	h.Write([]byte(source))
	h.Write([]byte{'|'})
	h.Write([]byte(FirstStackLine(stack)))
	return h.Sum(nil)
}
