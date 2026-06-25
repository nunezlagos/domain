package bootstrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)







func TestBehavior_EmailValidation(t *testing.T) {
	valid := []string{
		"admin@saargo.com",
		"user.name+tag@sub.example.co.uk",
		"x@y.io",
		"a.b.c@d.e.f.gh",
	}
	for _, e := range valid {
		t.Run(e, func(t *testing.T) {
			require.True(t, emailRegex.MatchString(strings.ToLower(e)),
				"email %q debe ser valido", e)
		})
	}

	invalid := []string{
		"",
		"no-at-sign",
		"@",
		"two@@at.com",
		"trailing@",
		"leading.dot@.com",
		"spaces in@email.com",
		"no-tld@example",
	}
	for _, e := range invalid {
		t.Run("invalid:"+e, func(t *testing.T) {
			require.False(t, emailRegex.MatchString(strings.ToLower(e)),
				"email %q debe ser rechazado", e)
		})
	}
}



// Comportamiento: org name derivado del email domain (parte antes del
// primer punto del domain), title-cased.
func TestBehavior_OrgNameFromEmail(t *testing.T) {
	cases := []struct {
		email    string
		expected string
	}{
		{"admin@saargo.com", "Saargo"},
		{"user@example.org", "Example"},
		{"a@b.io", "B"},
		{"foo+bar@multi.level.domain.com", "Multi"},
		{"x@y.z", "Y"},
	}
	for _, tc := range cases {
		t.Run(tc.email, func(t *testing.T) {
			parts := strings.Split(tc.email, "@")
			if len(parts) != 2 {
				t.Skip("email malformado en test setup")
			}
			domain := parts[1]
			base := strings.SplitN(domain, ".", 2)[0]
			got := strings.Title(strings.ToLower(base))
			require.Equal(t, tc.expected, got)
		})
	}
}



func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Saargo", "saargo"},
		{"Example Co", "example-co"},
		{"multi word  slug", "multi-word-slug"},
		{"trailing-dash-", "trailing-dash"},
		{"-leading-dash", "leading-dash"},
		{"special!@#chars", "special-chars"},
		{"with_underscores", "with-underscores"},
		{"", "default"},                 // vacío → default
		{"   ", "default"},               // solo espacios → default
		{"@@@", "default"},               // todo special → default
		{"valid-name-123", "valid-name-123"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, slugify(tc.in))
		})
	}
}

// Slug max length: 50 chars (defensa contra abuse con nombres largos).
func TestSlugify_MaxLength(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := slugify(long)
	require.LessOrEqual(t, len(got), 50)
	require.Equal(t, strings.Repeat("a", 50), got)
}



func TestSentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrNotFirstRun,
		ErrInvalidEmail,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			require.NotEqual(t, a, b)
		}
	}
}

// ErrNotFirstRun debe ser detectable via errors.Is
func TestSentinels_ErrorsIs(t *testing.T) {
	wrapped := errors.Join(ErrNotFirstRun, errors.New("DB has 5 users"))
	require.True(t, errors.Is(wrapped, ErrNotFirstRun))
}



// generateAPIKey produce un plaintext con prefijo "domk_live_" + 32 chars random.
// Total: 10 + 32 = 42 chars.
func TestGenerateAPIKey_Format(t *testing.T) {
	plain, hash, err := generateAPIKey()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(plain, "domk_live_"),
		"plaintext debe empezar con domk_live_, got: %s", plain)
	require.Equal(t, 10+32, len(plain), "10 (domk_live_) + 32 random = 42 chars")


	require.GreaterOrEqual(t, len(hash), 59, "bcrypt hash >= 59 chars")
	require.True(t, strings.HasPrefix(string(hash), "$2"),
		"bcrypt hash debe empezar con $2")
}

// Dos keys generadas son distintas (probabilidad de colision negligible).
func TestGenerateAPIKey_Unique(t *testing.T) {
	a, _, _ := generateAPIKey()
	b, _, _ := generateAPIKey()
	require.NotEqual(t, a, b, "dos keys generadas deben ser distintas")
}



// El lock key es una constante fija. Si alguien lo cambia, el lock deja
// de funcionar (race condition reintroducida).
func TestBootstrapLockKey_IsStable(t *testing.T) {
	require.Equal(t, int64(0x424F4F54), BootstrapLockKey,
		"BootstrapLockKey debe ser la constante 0x424F4F54 ('BOOT')")
}
