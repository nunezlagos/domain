package flowrunner

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/flow"
)

func TestClassifyError_Matrix(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"context deadline exceeded", "timeout"},
		{"step timeout after 30s", "timeout"},
		{"anthropic 429: rate limit exceeded", "rate_limit"},
		{"too many requests", "rate_limit"},
		{"dial tcp: connection refused", "network"},
		{"unexpected EOF", "network"},
		{"validation failed: field x", "validation_error"},
		{"signal_name required (validation)", "validation_error"},
		{"skill not found", "not_found"},
		{"boom", "unknown"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, classifyError(errors.New(tc.msg)), tc.msg)
	}
	require.Equal(t, "", classifyError(nil))
}

func TestBuildRetryPlan_FixedBackoff(t *testing.T) {
	step := &flow.Step{ID: "s1", Retry: &flow.StepRetryPolicy{
		MaxRetries: 2, Backoff: "fixed", FixedDelayMs: 5000,
	}}
	plan := buildRetryPlan(step)
	require.Equal(t, 3, plan.maxAttempts)
	require.True(t, plan.rich)
	// Escenario 2: delays constantes
	require.Equal(t, 5*time.Second, plan.delay(1))
	require.Equal(t, 5*time.Second, plan.delay(2))
}

func TestBuildRetryPlan_ExponentialBackoff(t *testing.T) {
	step := &flow.Step{ID: "s1", Retry: &flow.StepRetryPolicy{
		MaxRetries: 3, Backoff: "exponential", InitialDelayMs: 1000,
	}}
	plan := buildRetryPlan(step)
	// Escenario 1: 1000, 2000, 4000ms
	require.Equal(t, 1*time.Second, plan.delay(1))
	require.Equal(t, 2*time.Second, plan.delay(2))
	require.Equal(t, 4*time.Second, plan.delay(3))
}

func TestBuildRetryPlan_RetryOnFilter(t *testing.T) {
	step := &flow.Step{ID: "s1", Retry: &flow.StepRetryPolicy{
		MaxRetries: 2, RetryOn: []string{"timeout", "rate_limit"},
	}}
	plan := buildRetryPlan(step)
	// Escenario 3
	require.True(t, plan.shouldRetry(errors.New("context deadline exceeded (timeout)")))
	require.True(t, plan.shouldRetry(errors.New("429 rate limit")))
	require.False(t, plan.shouldRetry(errors.New("validation failed")))
}

func TestBuildRetryPlan_EmptyRetryOn_RetriesAll(t *testing.T) {
	step := &flow.Step{ID: "s1", Retry: &flow.StepRetryPolicy{MaxRetries: 1}}
	plan := buildRetryPlan(step)
	require.True(t, plan.shouldRetry(errors.New("validation failed")),
		"retry_on vacío → TODOS los errores reintentables")
}

func TestBuildRetryPlan_CapsMaxRetries(t *testing.T) {
	step := &flow.Step{ID: "s1", Retry: &flow.StepRetryPolicy{MaxRetries: 50}}
	plan := buildRetryPlan(step)
	require.Equal(t, flow.MaxRetriesCap+1, plan.maxAttempts)
}

func TestBuildRetryPlan_LegacyUsesTransientGate(t *testing.T) {
	step := &flow.Step{ID: "s1", Retries: 2}
	plan := buildRetryPlan(step)
	require.False(t, plan.rich)
	require.Equal(t, 3, plan.maxAttempts)
	require.False(t, plan.shouldRetry(errors.New("validation failed")),
		"legacy path mantiene gate de errores transient")
	require.True(t, plan.shouldRetry(errors.New("connection reset")))
}
