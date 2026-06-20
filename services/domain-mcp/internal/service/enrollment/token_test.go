package enrollment

import (
	"strings"
	"testing"
)

func TestGeneratePlaintext_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pt, prefix, hash, err := GeneratePlaintext()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !strings.HasPrefix(pt, TokenPrefix) {
			t.Errorf("plaintext no empieza con %q: %q", TokenPrefix, pt)
		}
		if len(prefix) != PrefixLen {
			t.Errorf("prefix len = %d, want %d", len(prefix), PrefixLen)
		}
		if !strings.HasPrefix(pt, prefix) {
			t.Errorf("plaintext no incluye prefix: %q ⊄ %q", prefix, pt)
		}
		if len(hash) == 0 {
			t.Errorf("hash vacío")
		}
		if seen[pt] {
			t.Errorf("plaintext colision en iter %d: %q", i, pt)
		}
		seen[pt] = true
	}
}

func TestParsePrefix_Valid(t *testing.T) {
	pt, prefix, _, err := GeneratePlaintext()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePrefix(pt)
	if err != nil {
		t.Fatal(err)
	}
	if got != prefix {
		t.Errorf("got %q, want %q", got, prefix)
	}
}

func TestParsePrefix_Invalid(t *testing.T) {
	cases := []string{
		"",
		"et_",
		"et_short",
		"foo_abcdefghij",
		"sin_prefix",
	}
	for _, tc := range cases {
		if _, err := ParsePrefix(tc); err == nil {
			t.Errorf("%q debería ser inválido", tc)
		}
	}
}

func TestVerifyHash_Match(t *testing.T) {
	pt, _, hash, err := GeneratePlaintext()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyHash(pt, hash); err != nil {
		t.Errorf("verify match falló: %v", err)
	}
}

func TestVerifyHash_NoMatch(t *testing.T) {
	pt1, _, hash1, _ := GeneratePlaintext()
	pt2, _, _, _ := GeneratePlaintext()
	if pt1 == pt2 {
		t.Fatal("colisión inesperada")
	}
	if err := VerifyHash(pt2, hash1); err == nil {
		t.Errorf("verify no-match: bcrypt.Compare debería rechazar")
	}
}
