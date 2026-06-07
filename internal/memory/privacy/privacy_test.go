package privacy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrip_NoPrivate(t *testing.T) {
	c, n := Strip("hola mundo")
	require.Equal(t, "hola mundo", c)
	require.Equal(t, 0, n)
}

func TestStrip_OneBlock(t *testing.T) {
	c, n := Strip("antes <private>secreto</private> después")
	require.Equal(t, "antes  después", c)
	require.Equal(t, 1, n)
}

func TestStrip_MultipleBlocks(t *testing.T) {
	c, n := Strip("<private>token</private> y <private>password</private>")
	require.Equal(t, " y ", c)
	require.Equal(t, 2, n)
}

func TestStrip_MultilineBlock(t *testing.T) {
	c, n := Strip("hola\n<private>linea1\nlinea2</private>\nchau")
	require.Equal(t, "hola\n\nchau", c)
	require.Equal(t, 1, n)
}

func TestHasPrivateBlocks(t *testing.T) {
	require.True(t, HasPrivateBlocks("<private>x</private>"))
	require.False(t, HasPrivateBlocks("hola"))
}

func TestNormalizeWhitespace(t *testing.T) {
	require.Equal(t, "hola mundo", NormalizeWhitespace("   hola   mundo  "))
	require.Equal(t, "a b c", NormalizeWhitespace("a\nb\tc"))
}

// Sabotaje: tag malformado sin cierre NO se strippea.
func TestSabotage_Strip_UnclosedTagNotRemoved(t *testing.T) {
	c, n := Strip("<private>sin cierre")
	require.Equal(t, "<private>sin cierre", c)
	require.Equal(t, 0, n)
}
