package secrets

import (
	"strings"
	"testing"
)

func TestGeneratePassword_LenYDistintos(t *testing.T) {
	a, err := GeneratePassword(32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratePassword(32)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two GeneratePassword calls produced same value (entropy issue)")
	}

	if len(a) < 40 || len(a) > 50 {
		t.Fatalf("unexpected length: %d", len(a))
	}
}

func TestGeneratePassword_RechazaMenosDe16Bytes(t *testing.T) {
	_, err := GeneratePassword(8)
	if err == nil {
		t.Fatal("expected error for nBytes < 16")
	}
}

func TestIsValidRoleName(t *testing.T) {
	cases := map[string]bool{
		"app_user":    true,
		"app_admin":   true,
		"x":           true,
		"role_123":    true,
		"":            false,
		"with space":  false,
		"with-dash":   false,
		"with;semi":   false, // SQL injection attempt
		"UPPER_CASE":  false,
		"emoji_😀":     false,
	}
	for in, want := range cases {
		if got := isValidRoleName(in); got != want {
			t.Fatalf("%q: got %v, want %v", in, got, want)
		}
	}
}

func TestPgQuoteIdent_EscapaQuotes(t *testing.T) {
	cases := map[string]string{
		"app_user":   `"app_user"`,
		"with\"quot": `"with""quot"`,
	}
	for in, want := range cases {
		if got := pgQuoteIdent(in); got != want {
			t.Fatalf("pgQuoteIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPgQuoteLiteral_DollarQuoted(t *testing.T) {
	got := pgQuoteLiteral("password_with_'quotes'_and_$dollars$")
	if !strings.HasPrefix(got, "$domain_pw$") || !strings.HasSuffix(got, "$domain_pw$") {
		t.Fatalf("not dollar-quoted: %s", got)
	}
}
