package cost

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMAForecast_Basic(t *testing.T) {
	f := SMAForecast([]float64{10, 20, 30}, 100, 10, 30)
	require.Equal(t, 3, f.WindowDays)
	require.InDelta(t, 20.0, f.AvgDailyUSD, 0.001)
	require.InDelta(t, 600.0, f.Next30DaysUSD, 0.001)
	require.InDelta(t, 100.0+20.0*20, f.MonthEndUSD, 0.001, "MTD + avg*(días restantes)")
}

// Sabotaje escenario: forecast sin datos → 0, no panic.
func TestSMAForecast_NoData_ZeroNoPanic(t *testing.T) {
	f := SMAForecast(nil, 0, 15, 30)
	require.Zero(t, f.AvgDailyUSD)
	require.Zero(t, f.Next30DaysUSD)
	require.Zero(t, f.MonthEndUSD)
}

func TestSMAForecast_EndOfMonth_NoNegativeRemaining(t *testing.T) {
	f := SMAForecast([]float64{10}, 300, 31, 30)
	require.InDelta(t, 300.0, f.MonthEndUSD, 0.001, "días restantes nunca negativos")
}

func TestBudgetStatus_Detection(t *testing.T) {
	require.Equal(t, "ok", BudgetStatus(10, 100, 80))
	require.Equal(t, "warning", BudgetStatus(80, 100, 80))
	require.Equal(t, "warning", BudgetStatus(99.9, 100, 80))
	require.Equal(t, "exceeded", BudgetStatus(100, 100, 80))
	require.Equal(t, "exceeded", BudgetStatus(150, 100, 80))
	require.Equal(t, "ok", BudgetStatus(50, 0, 80), "amount 0 nunca alerta (defensivo)")
}
