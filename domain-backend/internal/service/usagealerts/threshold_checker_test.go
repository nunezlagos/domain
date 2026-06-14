package usagealerts

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRenderEmail_HasBreakdown(t *testing.T) {
	alert := CostAlert{
		OrganizationID: uuid.New(),
		TotalUSD:       50.50,
		ThresholdUSD:   50.00,
		AlertDate:      time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
		Breakdown: []CostBreakdownItem{
			{Provider: "Anthropic", Model: "Claude Sonnet", CostUSD: 30.00},
			{Provider: "Anthropic", Model: "Claude Haiku", CostUSD: 20.00},
			{Provider: "OpenAI", Model: "GPT-4", CostUSD: 0.50},
		},
	}
	subject, body := RenderEmail(alert)
	require.Contains(t, subject, "[domain] cost alert:")
	require.Contains(t, subject, "$50.00")
	require.Contains(t, subject, "$50.50")
	require.Contains(t, body, "Anthropic: $50.00")
	require.Contains(t, body, "Claude Sonnet: $30.00")
	require.Contains(t, body, "Claude Haiku: $20.00")
	require.Contains(t, body, "OpenAI: $0.50")
	require.Contains(t, body, "GPT-4: $0.50")
}

func TestRenderEmail_EmptyBreakdown(t *testing.T) {
	alert := CostAlert{
		OrganizationID: uuid.New(),
		TotalUSD:       10.00,
		ThresholdUSD:   5.00,
		AlertDate:      time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
	}
	_, body := RenderEmail(alert)
	require.Contains(t, body, "Threshold: $5.00")
	require.Contains(t, body, "Current spend: $10.00")
}

func TestCheckThresholds_IntegrationSkips(t *testing.T) {
	t.Skip("requires integration database")
}

func TestSendAlerts_IntegrationSkips(t *testing.T) {
	t.Skip("requires integration database")
}

func TestEnableCostThreshold_IntegrationSkips(t *testing.T) {
	t.Skip("requires integration database")
}
