package tokens

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTokenBudget_Validation(t *testing.T) {
	// Sabotaje: hard=0 → error inmediato
	_, err := NewTokenBudget(0, 0, 0, ModeError)
	require.Error(t, err)

	// hard > model.max_tokens → rechazado
	_, err = NewTokenBudget(0, 200_000, 100_000, ModeError)
	require.Error(t, err)

	// soft > hard → rechazado
	_, err = NewTokenBudget(150, 100, 0, ModeError)
	require.Error(t, err)

	// modo inválido → rechazado
	_, err = NewTokenBudget(50, 100, 0, "explode")
	require.Error(t, err)

	// válido (sin modelMax)
	m, err := NewTokenBudget(80, 100, 0, ModeError)
	require.NoError(t, err)
	require.NoError(t, m.Check(), "Check inicial debe ser ok")
}

func TestTrack_IncrementsAndState(t *testing.T) {
	m, err := NewTokenBudget(0, 100, 0, ModeError)
	require.NoError(t, err)
	require.NoError(t, m.Track(30))
	require.NoError(t, m.Track(20))
	st := m.State()
	require.Equal(t, 50, st.TokensUsed)
	require.Equal(t, 50, st.BudgetRemaining)
	require.InDelta(t, 50.0, st.Percentage, 0.001)
	require.False(t, st.Truncated)
}

func TestSoftLimit_CallbackFiresOnce(t *testing.T) {
	m, err := NewTokenBudget(50, 100, 0, ModeError)
	require.NoError(t, err)
	fired := 0
	m.OnSoftLimit = func(used, soft int) {
		fired++
		require.GreaterOrEqual(t, used, soft)
		require.Equal(t, 50, soft)
	}
	require.NoError(t, m.Track(40)) // 40 < 50
	require.Zero(t, fired)
	require.NoError(t, m.Track(15)) // 55 ≥ 50 → fire
	require.Equal(t, 1, fired)
	require.NoError(t, m.Track(10)) // no re-fire
	require.Equal(t, 1, fired)
}

func TestHardLimit_ErrorMode(t *testing.T) {
	m, err := NewTokenBudget(0, 100, 0, ModeError)
	require.NoError(t, err)
	require.ErrorIs(t, m.Track(120), ErrBudgetExceeded)
	require.ErrorIs(t, m.Check(), ErrBudgetExceeded, "Check posterior también bloquea")
	require.False(t, m.State().Truncated)
}

func TestHardLimit_TruncateMode(t *testing.T) {
	m, err := NewTokenBudget(0, 100, 0, ModeTruncate)
	require.NoError(t, err)
	require.ErrorIs(t, m.Track(120), ErrBudgetTruncated)
	require.True(t, m.State().Truncated)
	require.ErrorIs(t, m.Check(), ErrBudgetTruncated)
}

func TestReset(t *testing.T) {
	m, err := NewTokenBudget(50, 100, 0, ModeTruncate)
	require.NoError(t, err)
	_ = m.Track(120)
	require.True(t, m.State().Truncated)
	m.Reset()
	st := m.State()
	require.Zero(t, st.TokensUsed)
	require.False(t, st.Truncated)
	require.NoError(t, m.Check())
}
