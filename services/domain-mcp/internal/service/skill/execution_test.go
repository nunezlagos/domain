package skill

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScrubParams_RedactsSensitiveKeys(t *testing.T) {
	in := map[string]any{
		"name":          "ok",
		"api_key":       "sk-123",
		"Authorization": "Bearer xyz",
		"otp_code":      "123456",
		"nested": map[string]any{
			"password": "hunter2",
			"city":     "stgo",
		},
	}
	out := ScrubParams(in)
	require.Equal(t, "ok", out["name"])
	require.Equal(t, "[REDACTED]", out["api_key"])
	require.Equal(t, "[REDACTED]", out["Authorization"])
	require.Equal(t, "[REDACTED]", out["otp_code"])
	nested := out["nested"].(map[string]any)
	require.Equal(t, "[REDACTED]", nested["password"])
	require.Equal(t, "stgo", nested["city"])

	require.Equal(t, "sk-123", in["api_key"])
}

// Sabotaje: una key sensible con sufijo igual se redacta (substring match).
func TestSabotage_ScrubParams_SubstringMatch(t *testing.T) {
	out := ScrubParams(map[string]any{"stripe_secret_key": "sk_live_x", "monkey": "ok"})
	require.Equal(t, "[REDACTED]", out["stripe_secret_key"])
	require.Equal(t, "[REDACTED]", out["monkey"], "contiene 'key' — preferimos sobre-redactar")
}
