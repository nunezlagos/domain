package promptrouter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/promptrouter"
)

// stubProvider responde con canned content. NO hace red.
type stubProvider struct {
	name     string
	response string
	err      error
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Complete(_ context.Context, _ llm.CompletionOptions) (*llm.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &llm.Response{Content: s.response, Model: "stub"}, nil
}
func (s *stubProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func TestLLMClassifier_ValidJSON(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", response: `{"intent":"fix","confidence":0.92,"reasoning":"clear bug report with steps"}`},
		Fallback: promptrouter.HeuristicClassifier{},
	}
	intent, conf, reason, err := c.Classify(context.Background(), "el botón no funciona")
	require.NoError(t, err)
	require.Equal(t, promptrouter.IntentFix, intent)
	require.InDelta(t, 0.92, conf, 0.001)
	require.Contains(t, reason, "clear bug")
}

func TestLLMClassifier_JSONWithFences(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", response: "```json\n{\"intent\":\"feature\",\"confidence\":0.8,\"reasoning\":\"feat request\"}\n```"},
	}
	intent, _, _, err := c.Classify(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, promptrouter.IntentFeature, intent)
}

func TestLLMClassifier_InvalidIntent_FallsBackToHeuristic(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", response: `{"intent":"unknown","confidence":0.5,"reasoning":"x"}`},
		Fallback: promptrouter.HeuristicClassifier{},
	}
	intent, _, reason, err := c.Classify(context.Background(), "El botón no funciona ya pasé screenshot")
	require.NoError(t, err)
	// Heurística clasificaría como fix.
	require.Equal(t, promptrouter.IntentFix, intent)
	require.Contains(t, reason, "fallback")
}

func TestLLMClassifier_LLMError_FallsBack(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", err: errors.New("api rate limit")},
		Fallback: promptrouter.HeuristicClassifier{},
	}
	intent, _, reason, err := c.Classify(context.Background(), "URGENTE production caída")
	require.NoError(t, err)
	require.Equal(t, promptrouter.IntentHotfix, intent)
	require.Contains(t, reason, "fallback after LLM error")
}

func TestLLMClassifier_NoProviderNoFallback_Errors(t *testing.T) {
	c := &promptrouter.LLMClassifier{}
	_, _, _, err := c.Classify(context.Background(), "x")
	require.Error(t, err)
}

func TestLLMClassifier_ConfidenceClampedTo01(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", response: `{"intent":"chat","confidence":1.5,"reasoning":"over"}`},
	}
	_, conf, _, _ := c.Classify(context.Background(), "x")
	require.Equal(t, 1.0, conf)

	c.Provider = &stubProvider{name: "stub", response: `{"intent":"chat","confidence":-0.3,"reasoning":"neg"}`}
	_, conf, _, _ = c.Classify(context.Background(), "x")
	require.Equal(t, 0.0, conf)
}

// Sabotaje: si LLM devuelve garbage (texto no JSON), debe fallback NO crashear.
func TestSabotage_LLMReturnsGarbage(t *testing.T) {
	c := &promptrouter.LLMClassifier{
		Provider: &stubProvider{name: "stub", response: "I'm not JSON, just text."},
		Fallback: promptrouter.HeuristicClassifier{},
	}
	intent, _, _, err := c.Classify(context.Background(), "quiero exportar CSV")
	require.NoError(t, err, "fallback debe absorber el error de parse")
	require.Equal(t, promptrouter.IntentFeature, intent, "heurística clasifica como feature")
}
