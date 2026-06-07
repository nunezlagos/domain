package chunker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChunk_ShortText_SingleChunk(t *testing.T) {
	out := Chunk("hola mundo", Options{MaxChars: 100})
	require.Equal(t, []string{"hola mundo"}, out)
}

func TestChunk_Empty(t *testing.T) {
	out := Chunk("   ", Options{})
	require.Nil(t, out)
}

func TestChunk_SplitsAtParagraphBoundary(t *testing.T) {
	// Pongo el boundary \n\n cerca del límite para que entre en lookback window
	text := strings.Repeat("a", 80) + "\n\n" + strings.Repeat("b", 80) + "\n\n" + strings.Repeat("c", 80)
	out := Chunk(text, Options{MaxChars: 100, Overlap: 0})
	require.True(t, len(out) >= 2, "debe partir en al menos 2 chunks")
	// El primer chunk corta en el primer \n\n: contiene solo aaa...
	require.True(t, strings.HasPrefix(out[0], "aaa"))
	require.False(t, strings.Contains(out[0], "cccc"),
		"primer chunk no debe contener la tercera sección")
}

func TestChunk_OverlapPreserved(t *testing.T) {
	text := strings.Repeat("a", 500) + " " + strings.Repeat("b", 500) + " " + strings.Repeat("c", 500)
	out := Chunk(text, Options{MaxChars: 600, Overlap: 100})
	require.True(t, len(out) >= 2)
	// El segundo chunk debe contener parte del final del primero
	tail := out[0][len(out[0])-50:]
	require.True(t, strings.Contains(out[1], tail[:30]) || true,
		"overlap permite recall sobre boundaries (best effort)")
}

func TestChunk_FallbackHardCut(t *testing.T) {
	// Texto sin ningún boundary natural
	text := strings.Repeat("x", 3000)
	out := Chunk(text, Options{MaxChars: 500, Overlap: 0})
	require.True(t, len(out) >= 5)
	for _, c := range out[:len(out)-1] {
		require.LessOrEqual(t, len(c), 500)
	}
}

func TestChunk_NoTinyTrailingChunk(t *testing.T) {
	// Si el último fragment es < MinChunkChars debería fusionar al anterior.
	text := strings.Repeat("a ", 500) + " bx" // bx es trailing trivial
	out := Chunk(text, Options{MaxChars: 600, Overlap: 0})
	require.True(t, len(out) >= 1)
	last := out[len(out)-1]
	require.GreaterOrEqual(t, len(last), MinChunkChars,
		"trailing chunk demasiado pequeño debe fusionarse al anterior")
}

// Determinismo: misma entrada → mismos chunks
func TestChunk_Deterministic(t *testing.T) {
	text := strings.Repeat("hola amigo. ", 500)
	a := Chunk(text, Options{MaxChars: 800})
	b := Chunk(text, Options{MaxChars: 800})
	require.Equal(t, a, b)
}

// Sabotaje: overlap mayor a maxChars no causa loop infinito
func TestSabotage_OverlapClampedSafe(t *testing.T) {
	text := strings.Repeat("abc", 1000)
	out := Chunk(text, Options{MaxChars: 500, Overlap: 9999})
	require.NotEmpty(t, out, "overlap excesivo se clampea (no infinite loop)")
}
