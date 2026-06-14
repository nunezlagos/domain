package tokens_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm/tokens"
)

func TestStreamCounter_AccumulatesTokens(t *testing.T) {
	c := tokens.NewStreamCounter(0, time.Second)
	require.NoError(t, c.AddTokens(10))
	require.NoError(t, c.AddTokens(20))
	require.Equal(t, 30, c.Total())
}

func TestStreamCounter_BudgetExceeded(t *testing.T) {
	c := tokens.NewStreamCounter(50, time.Second)
	require.NoError(t, c.AddTokens(30))
	require.NoError(t, c.AddTokens(15))
	err := c.AddTokens(10) // total 55, over 50
	require.ErrorIs(t, err, tokens.ErrBudgetExceeded)
}

func TestStreamCounter_Remaining(t *testing.T) {
	c := tokens.NewStreamCounter(100, time.Second)
	c.AddTokens(40)
	require.Equal(t, 60, c.Remaining())
	c.AddTokens(60)
	require.Equal(t, 0, c.Remaining())
}

func TestStreamCounter_NoBudget(t *testing.T) {
	c := tokens.NewStreamCounter(0, time.Second)
	c.AddTokens(1_000_000)
	require.Greater(t, c.Remaining(), 1_000_000_000)
}

func TestStreamCounter_AddText_UsesEstimate(t *testing.T) {
	c := tokens.NewStreamCounter(0, time.Second)
	require.NoError(t, c.Add("hello world"))
	require.Greater(t, c.Total(), 0)
}

func TestStreamCounter_AddZeroNoOp(t *testing.T) {
	c := tokens.NewStreamCounter(10, time.Second)
	require.NoError(t, c.AddTokens(0))
	require.Equal(t, 0, c.Total())
}

func TestStreamCounter_Snapshot_BudgetUsed(t *testing.T) {
	c := tokens.NewStreamCounter(100, time.Second)
	c.AddTokens(25)
	s := c.Snapshot()
	require.Equal(t, 25, s.Total)
	require.InDelta(t, 0.25, s.BudgetUsed, 0.001)
}

func TestStreamCounter_Concurrent(t *testing.T) {
	c := tokens.NewStreamCounter(0, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.AddTokens(2)
		}()
	}
	wg.Wait()
	require.Equal(t, 100, c.Total())
}

// Sabotaje: si se permite AddTokens negativo, el budget se puede burlar.
// Verificamos que negativos son no-op (no rebajan el total).
func TestSabotage_NegativeTokensIgnored(t *testing.T) {
	c := tokens.NewStreamCounter(10, time.Second)
	require.NoError(t, c.AddTokens(8))
	require.NoError(t, c.AddTokens(-100)) // intenta resetear vía negativos
	require.Equal(t, 8, c.Total(), "negativos no deben rebajar el total")
	err := c.AddTokens(5) // 8+5 = 13 > 10
	require.ErrorIs(t, err, tokens.ErrBudgetExceeded)
}
