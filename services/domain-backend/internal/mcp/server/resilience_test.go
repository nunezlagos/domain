package mcpserver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestResilientWrapper_NoBudget_AllowsAll(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{}) // unlimited
	var calls atomic.Int64
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("test_tool", h)
	for i := 0; i < 50; i++ {
		_, err := wrapped(context.Background(), mcp.CallToolRequest{})
		require.NoError(t, err)
	}
	require.EqualValues(t, 50, calls.Load())
}

func TestResilientWrapper_RateLimit(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CallsPerMinute: 5})
	var calls atomic.Int64
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("limited", h)
	for i := 0; i < 5; i++ {
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	}
	// Sixth call debe ser rate limited
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].(mcp.TextContent).Text, "rate limit")
	require.EqualValues(t, 5, calls.Load(), "handler NO debe invocarse cuando rate limited")
}

func TestResilientWrapper_Retry_OnTransient(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{MaxRetries: 2, RetryBackoff: 1 * time.Millisecond})
	var attempts atomic.Int64
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n := attempts.Add(1)
		if n < 3 {
			return nil, errors.New("connection reset by peer")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("flaky", h)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.EqualValues(t, 3, attempts.Load())
}

func TestResilientWrapper_Retry_NonTransientNoRetry(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{MaxRetries: 3, RetryBackoff: 1 * time.Millisecond})
	var attempts atomic.Int64
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		attempts.Add(1)
		return nil, errors.New("invalid input: bad json")
	}
	wrapped := r.Wrap("hard_fail", h)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.Error(t, err)
	require.EqualValues(t, 1, attempts.Load(), "errores no-transient no se reintentan")
}

func TestResilientWrapper_SetBudgetPerTool(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CallsPerMinute: 100}) // default permisivo
	r.SetBudget("specific_tool", ToolBudget{CallsPerMinute: 2})

	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("specific_tool", h)

	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	result, _ := wrapped(context.Background(), mcp.CallToolRequest{})
	require.True(t, result.IsError, "tercer call debe rate limited (per-tool override)")
}

// Sabotaje: timeouts en wrapper no causan deadlock infinito
func TestSabotage_RateLimitWindow_Compacts(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CallsPerMinute: 3})
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("window", h)
	for i := 0; i < 3; i++ {
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	}
	// Llenar window y verificar que NO crece sin bound
	state := r.state("window")
	state.mu.Lock()
	windowSize := len(state.window)
	state.mu.Unlock()
	require.LessOrEqual(t, windowSize, 3)
}

func failHandler(calls *atomic.Int64) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return nil, errors.New("invalid input: boom")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CBThreshold: 3, CBCooldown: time.Hour})
	var calls atomic.Int64
	wrapped := r.Wrap("cb_tool", failHandler(&calls))

	// 3 fallos consecutivos → breaker abre
	for i := 0; i < 3; i++ {
		_, err := wrapped(context.Background(), mcp.CallToolRequest{})
		require.Error(t, err)
	}
	// 4ta call: circuit open, handler NO se invoca
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].(mcp.TextContent).Text, "circuit open")
	require.EqualValues(t, 3, calls.Load(), "handler no debe invocarse con breaker abierto")
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CBThreshold: 2, CBCooldown: time.Minute})
	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	current := base
	r.now = func() time.Time { return current }

	var fails, oks atomic.Int64
	failing := true
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if failing {
			fails.Add(1)
			return nil, errors.New("invalid input: boom")
		}
		oks.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("cb_recover", h)

	// Abrir breaker
	for i := 0; i < 2; i++ {
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	}
	result, _ := wrapped(context.Background(), mcp.CallToolRequest{})
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].(mcp.TextContent).Text, "circuit open")

	// Avanzar reloj pasado el cooldown → half-open: la call pasa y el éxito resetea
	current = base.Add(2 * time.Minute)
	failing = false
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.EqualValues(t, 1, oks.Load())

	// Breaker reseteado: siguientes calls pasan normal
	result, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	require.False(t, result.IsError)
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CBThreshold: 2, CBCooldown: time.Minute})
	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	current := base
	r.now = func() time.Time { return current }

	var calls atomic.Int64
	wrapped := r.Wrap("cb_reopen", failHandler(&calls))

	for i := 0; i < 2; i++ {
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	}
	// Half-open trial falla → re-abre con UN solo fallo
	current = base.Add(2 * time.Minute)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.Error(t, err)
	require.EqualValues(t, 3, calls.Load())

	result, _ := wrapped(context.Background(), mcp.CallToolRequest{})
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].(mcp.TextContent).Text, "circuit open")
	require.EqualValues(t, 3, calls.Load(), "re-abierto: handler no se invoca")
}

// Sabotaje: un éxito intercalado resetea el contador — el breaker NUNCA debe
// abrir si los fallos no son consecutivos.
func TestSabotage_CircuitBreaker_NonConsecutiveFailuresDontOpen(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{CBThreshold: 3, CBCooldown: time.Hour})
	var calls atomic.Int64
	fail := true
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		if fail {
			return nil, errors.New("invalid input: boom")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	}
	wrapped := r.Wrap("cb_mixed", h)

	// fallo, fallo, ÉXITO, fallo, fallo, ÉXITO — nunca 3 consecutivos
	for i := 0; i < 2; i++ {
		fail = true
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
		fail = false
		_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	}
	require.EqualValues(t, 6, calls.Load(), "breaker jamás debió abrir")
}
