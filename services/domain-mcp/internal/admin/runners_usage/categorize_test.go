package runners_usage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCategorize_ZeroIsNeverUsed(t *testing.T) {
	require.Equal(t, CategoryNeverUsed, Categorize(0, 30))
}

func TestCategorize_Boundaries(t *testing.T) {
	cases := []struct {
		name  string
		total int
		days  int
		want  Category
	}{

		{"30d total=0 → NUNCA", 0, 30, CategoryNeverUsed},
		{"30d total=1 → POCO", 1, 30, CategoryLowUse},
		{"30d total=9 → POCO", 9, 30, CategoryLowUse},
		{"30d total=10 → USADO (boundary)", 10, 30, CategoryUsed},
		{"30d total=15 → USADO", 15, 30, CategoryUsed},
		{"30d total=100 → USADO", 100, 30, CategoryUsed},

		{"total negativo → NUNCA", -1, 30, CategoryNeverUsed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Categorize(tc.total, tc.days))
		})
	}
}

func TestCategorize_AdaptsToShortWindow(t *testing.T) {


	require.Equal(t, CategoryUsed, Categorize(1, 5))
	require.Equal(t, CategoryUsed, Categorize(3, 5))
	require.Equal(t, CategoryNeverUsed, Categorize(0, 5))

	require.Equal(t, CategoryUsed, Categorize(1, 3))
	require.Equal(t, CategoryNeverUsed, Categorize(0, 3))
}

func TestCategorize_LongWindowStillTen(t *testing.T) {

	require.Equal(t, CategoryUsed, Categorize(10, 60))
	require.Equal(t, CategoryLowUse, Categorize(9, 60))
	require.Equal(t, CategoryUsed, Categorize(10, 365))
}

func TestCategorize_ZeroDaysIsNeverUsed(t *testing.T) {

	require.Equal(t, CategoryNeverUsed, Categorize(0, 0))
}
