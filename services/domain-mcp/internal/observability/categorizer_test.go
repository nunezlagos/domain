package observability

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCategorizer_Categorize_PgError42_ReturnsSQLError(t *testing.T) {
	err := &pgconn.PgError{Code: "42P01", Message: `relation "foo" does not exist`}
	if got := Categorize(err); got != CategorySQL {
		t.Fatalf("got %q, want %q", got, CategorySQL)
	}
}

func TestCategorizer_Categorize_PgError23_ReturnsSQLError(t *testing.T) {
	err := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	if got := Categorize(err); got != CategorySQL {
		t.Fatalf("got %q, want %q", got, CategorySQL)
	}
}

func TestCategorizer_Categorize_Timeout_ReturnsTimeout(t *testing.T) {
	if got := Categorize(errors.New("context deadline exceeded")); got != CategoryTimeout {
		t.Fatalf("got %q, want %q", got, CategoryTimeout)
	}
}

func TestCategorizer_Categorize_Panic_ReturnsPanic(t *testing.T) {
	err := errors.New("runtime error: invalid memory address or nil pointer dereference")
	if got := Categorize(err); got != CategoryPanic {
		t.Fatalf("got %q, want %q", got, CategoryPanic)
	}
}

func TestCategorizer_Categorize_Auth_ReturnsAuthError(t *testing.T) {
	if got := Categorize(errors.New("unauthorized: invalid token")); got != CategoryAuth {
		t.Fatalf("got %q, want %q", got, CategoryAuth)
	}
}

func TestCategorizer_Categorize_RateLimit_ReturnsRateLimit(t *testing.T) {
	if got := Categorize(errors.New("rate limit exceeded: too many requests")); got != CategoryRateLimit {
		t.Fatalf("got %q, want %q", got, CategoryRateLimit)
	}
}

func TestCategorizer_Categorize_External_ReturnsExternal(t *testing.T) {
	err := errors.New("dial tcp 1.2.3.4:5432: connection refused")
	if got := Categorize(err); got != CategoryExternal {
		t.Fatalf("got %q, want %q", got, CategoryExternal)
	}
}

func TestCategorizer_Categorize_Validation_ReturnsValidation(t *testing.T) {
	if got := Categorize(errors.New("field 'name' is required")); got != CategoryValidation {
		t.Fatalf("got %q, want %q", got, CategoryValidation)
	}
}

func TestCategorizer_Categorize_Nil_ReturnsEmpty(t *testing.T) {
	if got := Categorize(nil); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestCategorizer_Fingerprint_StableAcrossVolatileTokens(t *testing.T) {
	a := Fingerprint(CategorySQL, `relation "tmp_123" does not exist at 10:00`, "bootstrap", "pkg.Bootstrap")
	b := Fingerprint(CategorySQL, `relation "tmp_456" does not exist at 11:30`, "bootstrap", "pkg.Bootstrap")
	if !bytes.Equal(a, b) {
		t.Fatalf("fingerprints should collapse volatile tokens: %x vs %x", a, b)
	}
}

func TestCategorizer_Fingerprint_DiffersBySource(t *testing.T) {
	a := Fingerprint(CategorySQL, "same message", "sourceA", "stack")
	b := Fingerprint(CategorySQL, "same message", "sourceB", "stack")
	if bytes.Equal(a, b) {
		t.Fatal("fingerprints should differ by source")
	}
}

func TestCategorizer_Fingerprint_DiffersByCategory(t *testing.T) {
	a := Fingerprint(CategorySQL, "x", "s", "k")
	b := Fingerprint(CategoryTimeout, "x", "s", "k")
	if bytes.Equal(a, b) {
		t.Fatal("fingerprints should differ by category")
	}
}
