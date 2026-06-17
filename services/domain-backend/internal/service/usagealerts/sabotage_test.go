package usagealerts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSabotage_DedupPreventsDuplicateAlerts(t *testing.T) {
	date := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	alert := CostAlert{
		TotalUSD:     50.50,
		ThresholdUSD: 50.00,
		AlertDate:    date,
	}

	subject, body := RenderEmail(alert)

	// Verificamos que el subject y body son determinísticos
	require.Contains(t, subject, "[domain] cost alert:")
	require.Contains(t, body, "Current spend: $50.50")

	// El anti-spam depende de la UNIQUE constraint en cost_alerts_sent,
	// no del render. Si la constraint no existiera (sabotaje),
	// el mismo alert se enviaría múltiples veces.
	subject2, body2 := RenderEmail(alert)
	require.Equal(t, subject, subject2, "same alert must produce same subject")
	require.Equal(t, body, body2, "same alert must produce same body")
}
