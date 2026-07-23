package systemcron

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashEmail_NonEmpty_Returns8HexChars(t *testing.T) {
	got := hashEmail("victim@example.com")
	require.Len(t, got, 8)
	require.Regexp(t, "^[0-9a-f]{8}$", got)
}

func TestHashEmail_Empty_ReturnsEmpty(t *testing.T) {
	require.Equal(t, "", hashEmail(""), "apikey failure (email NULL) no produce hash")
}

func TestHashEmail_Deterministic(t *testing.T) {
	require.Equal(t, hashEmail("a@b.com"), hashEmail("a@b.com"))
	require.NotEqual(t, hashEmail("a@b.com"), hashEmail("c@d.com"))
}
