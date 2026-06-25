

package pii

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedact_Email(t *testing.T) {
	require.Equal(t, "user [EMAIL] sent message", Redact("user alice@example.com sent message"))
	require.Equal(t, "[EMAIL]", Redact("alice+tag@sub.example.co"))
	require.Equal(t, "no email here", Redact("no email here"))
}

func TestRedact_RUT(t *testing.T) {
	cases := []struct{ in, want string }{
		{"RUT 12.345.678-5", "RUT [RUT]"},
		{"RUT 12345678-5", "RUT [RUT]"},
		{"RUT 12345678-K", "RUT [RUT]"},
		{"RUT 12345678-k", "RUT [RUT]"},
		{"sin RUT", "sin RUT"},
		{"id 12345678k sin guion", "id 12345678k sin guion"}, // NO matchea sin guión
	}
	for _, c := range cases {
		require.Equal(t, c.want, Redact(c.in), "input: %q", c.in)
	}
}

func TestRedact_APIKey(t *testing.T) {
	require.Contains(t,
		Redact("key=domk_live_abc123def456ghi789jkl012mno345"),
		"[API_KEY]")
	require.Contains(t,
		Redact("uses domk_test_xyzabcdefghijklmnopqrstuvwxyz"),
		"[API_KEY]")
	require.NotContains(t, Redact("not a key"), "[API_KEY]")
}

func TestRedact_Bearer(t *testing.T) {
	require.Contains(t,
		Redact("Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"),
		"Bearer [TOKEN]")

	require.Contains(t,
		Redact("BEARER abc123def456ghi789jkl012"),
		"[TOKEN]")
}

func TestRedact_Phone(t *testing.T) {
	require.Contains(t, Redact("call +56 9 1234 5678"), "[PHONE]")
	require.Contains(t, Redact("call +56912345678"), "[PHONE]")
	require.NotContains(t, Redact("no phone here"), "[PHONE]")
}

func TestRedact_CreditCard(t *testing.T) {
	require.Contains(t, Redact("card 4111 1111 1111 1111"), "[CC]")
	require.Contains(t, Redact("card 4111-1111-1111-1111"), "[CC]")
	require.Contains(t, Redact("card 4111111111111111"), "[CC]")
}

func TestRedact_Multiple(t *testing.T) {
	in := "user alice@x.com (RUT 12.345.678-5) llamó al +56 9 1234 5678 con key domk_live_abcdefghijklmnopqrstuvwxyz"
	out := Redact(in)
	require.Contains(t, out, "[EMAIL]")
	require.Contains(t, out, "[RUT]")
	require.Contains(t, out, "[PHONE]")
	require.Contains(t, out, "[API_KEY]")
	require.NotContains(t, out, "alice@x.com")
	require.NotContains(t, out, "12.345.678-5")
}

func TestRedact_Empty(t *testing.T) {
	require.Equal(t, "", Redact(""))
}

func TestContainsPII(t *testing.T) {
	require.True(t, ContainsPII("alice@x.com"))
	require.True(t, ContainsPII("12.345.678-5"))
	require.True(t, ContainsPII("domk_live_abcdefghijklmnopqrstuvwxyz"))
	require.False(t, ContainsPII("hello world"))
	require.False(t, ContainsPII(""))
}

func TestRedactMap(t *testing.T) {
	in := map[string]string{
		"name":  "alice",
		"email": "alice@example.com",
		"id":    "12.345.678-5",
	}
	out := RedactMap(in)
	require.Equal(t, "alice", out["name"])
	require.Equal(t, "[EMAIL]", out["email"])
	require.Equal(t, "[RUT]", out["id"])
}

func TestRedactHeader_SensitiveHeaders(t *testing.T) {
	in := map[string][]string{
		"Authorization": {"Bearer abc123def456ghi789jkl012"},
		"Cookie":        {"session=xxx"},
		"X-Api-Key":     {"domk_live_xyzabcdefghijklmnopqrstuvwxyz"},
		"User-Agent":    {"curl/8.0"},
	}
	out := RedactHeader(in)
	require.Equal(t, []string{"[REDACTED]"}, out["Authorization"])
	require.Equal(t, []string{"[REDACTED]"}, out["Cookie"])
	require.Equal(t, []string{"[REDACTED]"}, out["X-Api-Key"])
	require.Equal(t, []string{"curl/8.0"}, out["User-Agent"], "non-sensitive header preserved")
}

func TestRedactHeader_AppliesRegexToNonSensitive(t *testing.T) {
	in := map[string][]string{
		"X-Forwarded-For": {"alice@example.com"},
	}
	out := RedactHeader(in)
	require.Equal(t, []string{"[EMAIL]"}, out["X-Forwarded-For"])
}

// Sabotaje: emails con +tag son detectados.
func TestSabotage_EmailWithPlusTag(t *testing.T) {
	require.True(t, ContainsPII("user+tag@example.com"))
}

// Sabotaje: NO falsos positivos sobre IDs UUIDs/timestamps.
func TestSabotage_NoFalsePositives(t *testing.T) {
	cases := []string{
		"01234567-89ab-cdef-0123-456789abcdef",  // UUID
		"2026-06-07T12:00:00Z",                    // timestamp ISO
		"v1.2.3",                                  // version
		"http://example.com/path",                 // URL sin user
		"abc-def-ghi",                             // slug
	}
	for _, s := range cases {
		require.Falsef(t, ContainsPII(s), "false positive on: %q", s)
	}
}
