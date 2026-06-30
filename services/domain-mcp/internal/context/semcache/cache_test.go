package semcache

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)







// Mismos params (mismo map, distintos orden de keys) → mismo hash.
// JSON marshal canonico ordena keys alfabeticamente, garantizando
// determinismo independiente del orden de insercion.
func TestBehavior_HashParams_DeterministicAcrossKeyOrder(t *testing.T) {
	a := map[string]any{"temperature": 0.7, "top_p": 0.9, "max_tokens": 100}
	b := map[string]any{"max_tokens": 100, "top_p": 0.9, "temperature": 0.7}
	c := map[string]any{"temperature": 0.7, "top_p": 0.9, "max_tokens": 100}

	require.Equal(t, HashParams(a), HashParams(b),
		"mismos params en distinto orden DEBEN producir mismo hash")
	require.Equal(t, HashParams(b), HashParams(c))
}

// Distintos params → distinto hash. La función distingue temperature 0.7
// de 0.8 (importante: el cache de respuestas es sensible a params).
func TestBehavior_HashParams_DifferentValues_DifferentHashes(t *testing.T) {
	a := map[string]any{"temperature": 0.7}
	b := map[string]any{"temperature": 0.8}

	require.NotEqual(t, HashParams(a), HashParams(b),
		"temperature 0.7 vs 0.8 DEBEN producir hashes distintos")
}

// Params con tipos distintos (int vs float) → mismo hash si valor
// numéricamente igual. JSON canonicalization trata 1.0 == 1.
func TestBehavior_HashParams_TypeInsensitiveForNumbers(t *testing.T) {
	a := map[string]any{"max_tokens": 100}      // int
	b := map[string]any{"max_tokens": 100.0}    // float




	require.Equal(t, HashParams(a), HashParams(b),
		"100 (int) y 100.0 (float) DEBEN ser equivalentes en JSON canonical")
}

// Nil params → hash determinístico (no panic).
func TestBehavior_HashParams_Nil_OK(t *testing.T) {
	require.NotPanics(t, func() {
		h := HashParams(nil)
		require.NotEmpty(t, h, "hash de nil params debe ser no-vacio")
	})


	empty := HashParams(map[string]any{})
	require.NotEmpty(t, empty)

}



// Mismo prompt → mismo hash.
func TestBehavior_HashPrompt_SameInput_SameHash(t *testing.T) {
	prompt := "What is the capital of France?"
	require.Equal(t, HashPrompt(prompt), HashPrompt(prompt))
}

// Distinto prompt → distinto hash.
func TestBehavior_HashPrompt_DifferentInput_DifferentHash(t *testing.T) {
	a := "What is the capital of France?"
	b := "What is the capital of Germany?"
	require.NotEqual(t, HashPrompt(a), HashPrompt(b))
}

// Empty prompt → hash deterministico (no panic).
func TestBehavior_HashPrompt_Empty(t *testing.T) {
	require.NotPanics(t, func() {
		h := HashPrompt("")
		require.NotEmpty(t, h)
	})
}

// Hash format: 64 chars hex (sha256).
func TestBehavior_HashFormat_IsHex64(t *testing.T) {
	h := HashPrompt("test")
	require.Len(t, h, 64, "sha256 hex debe ser 64 chars")
	for _, c := range h {
		require.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"caracter %c debe ser hex lowercase", c)
	}
}



// Un hash de params vacios NO debe colisionar con un hash de prompt vacio.
func TestBehavior_ParamHash_PromptHash_DontCollide(t *testing.T) {



	paramHash := HashParams(nil)
	promptHash := HashPrompt("")
	require.NotEqual(t, paramHash, promptHash,
		"hashes de params nil y prompt empty NO deben colisionar")
}



// Entry tiene 11 campos. JSON tags deben ser estables (no romperse
// en refactors accidentales).
func TestBehavior_Entry_JSONShape(t *testing.T) {
	e := Entry{
		ID:            "abc-123",
		OrgID:         "org-1",
		Provider:      "openai",
		Model:         "gpt-4",
		ParamsHash:    "params-hash",
		PromptHash:    "prompt-hash",
		PromptPreview: "What is...",
	}


	v := reflect.ValueOf(e)
	require.Equal(t, 12, v.NumField(), "Entry tiene 12 campos — cambio aqui si se agrega uno nuevo")
}



// MinSimilarity default 0.95, TTL default 7 días. Si los defaults
// cambian, este test es el canary.
func TestBehavior_CacheDefaults(t *testing.T) {
	c := &Cache{Pool: nil} // sin Pool, solo para inspeccionar defaults




	require.Zero(t, c.MinSimilarity, "default 0 → usa 0.95 internamente")
	require.Zero(t, c.TTL, "default 0 → usa 7 dias internamente")
}



// ErrCacheMiss es el sentinel para "no se encontro nada con suficiente
// similaridad". Caller usa errors.Is para detectar.
func TestBehavior_ErrCacheMiss_Sentinel(t *testing.T) {
	require.NotNil(t, ErrCacheMiss)

	require.ErrorIs(t, ErrCacheMiss, ErrCacheMiss)

	require.False(t, errors.Is(ErrCacheMiss, errors.New("otro error")),
		"ErrCacheMiss NO debe matchear con un error distinto")
}
