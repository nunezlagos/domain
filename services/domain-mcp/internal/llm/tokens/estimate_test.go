package tokens

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func TestEstimate_Empty(t *testing.T) {
	require.Equal(t, 0, Estimate(""))
}

func TestEstimate_ShortLatin(t *testing.T) {
	// "Hola mundo" = 10 chars latin → ~3 tokens
	require.GreaterOrEqual(t, Estimate("Hola mundo"), 2)
	require.LessOrEqual(t, Estimate("Hola mundo"), 4)
}

func TestEstimate_LongLatin(t *testing.T) {
	text := strings.Repeat("hola amigos como estan ", 50) // ~1150 chars
	tokens := Estimate(text)
	// 1150/4 ~ 287
	require.Greater(t, tokens, 200)
	require.Less(t, tokens, 350)
}

func TestEstimate_CJK(t *testing.T) {
	// CJK chars = ~0.5 tokens/char típicamente (cjk/2 conservative)
	require.GreaterOrEqual(t, Estimate("你好"), 1)
}

func TestEstimateMessages_Overhead(t *testing.T) {
	msgs := []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "bien tu"},
	}
	total := EstimateMessages("Eres un asistente", msgs)
	// system_prompt + 2 mensajes con overhead
	require.Greater(t, total, 15)
}

func TestEstimate_Monotonic(t *testing.T) {
	short := Estimate("una frase corta")
	long := Estimate(strings.Repeat("una frase corta ", 100))
	require.Greater(t, long, short*50)
}
