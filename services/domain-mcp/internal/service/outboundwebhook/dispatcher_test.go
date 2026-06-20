package outboundwebhook

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMatchesFilters_TopLevelAndNested(t *testing.T) {
	data := json.RawMessage(`{"status":"completed","flow":{"slug":"deploy","steps":[{"id":"s1"}]}}`)

	require.True(t, matchesFilters(json.RawMessage(`{}`), data), "sin filtros → match")
	require.True(t, matchesFilters(json.RawMessage(`{"status":"completed"}`), data))
	require.False(t, matchesFilters(json.RawMessage(`{"status":"failed"}`), data))
	require.True(t, matchesFilters(json.RawMessage(`{"flow.slug":"deploy"}`), data), "path anidado")
	require.False(t, matchesFilters(json.RawMessage(`{"flow.slug":"otro"}`), data))
	require.True(t, matchesFilters(json.RawMessage(`{"flow.steps.0.id":"s1"}`), data), "índice de array")
	require.False(t, matchesFilters(json.RawMessage(`{"flow.steps.5.id":"s1"}`), data), "índice fuera de rango")
	require.False(t, matchesFilters(json.RawMessage(`{"no.existe":"x"}`), data))
}

// Sabotaje: el filtro nunca evalúa expresiones — un "filtro" malicioso es
// solo una key inexistente, no código.
func TestSabotage_Filters_NoEval(t *testing.T) {
	data := json.RawMessage(`{"a":1}`)
	require.False(t, matchesFilters(json.RawMessage(`{"a; DROP TABLE":"1"}`), data))
	require.False(t, matchesFilters(json.RawMessage(`{"__proto__.polluted":"1"}`), data))
}

func TestCircuitOpen_Threshold(t *testing.T) {
	now := time.Now()
	d := &Dispatcher{Now: func() time.Time { return now }}

	recent := now.Add(-10 * time.Minute)
	old := now.Add(-2 * time.Hour)

	require.False(t, d.circuitOpen(&Subscription{FailureCount: CBThreshold - 1, LastFailureAt: &recent}),
		"bajo el threshold → cerrado")
	require.True(t, d.circuitOpen(&Subscription{FailureCount: CBThreshold, LastFailureAt: &recent}),
		"threshold + fallo reciente → abierto")
	require.False(t, d.circuitOpen(&Subscription{FailureCount: CBThreshold + 5, LastFailureAt: &old}),
		"cooldown vencido → half-open (vuelve a intentar)")
	require.False(t, d.circuitOpen(&Subscription{FailureCount: CBThreshold}),
		"sin last_failure_at → cerrado")
}
