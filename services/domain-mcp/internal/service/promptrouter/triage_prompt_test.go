package promptrouter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/promptrouter"
)

// recordingProvider captura el SystemPrompt recibido para verificar qué
// prompt usó el classifier (const vs loader).
type recordingProvider struct {
	gotSystemPrompt string
	response        string
}

func (p *recordingProvider) Name() string { return "recording" }
func (p *recordingProvider) Complete(_ context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	p.gotSystemPrompt = opts.SystemPrompt
	return &llm.Response{Content: p.response, Model: "stub"}, nil
}
func (p *recordingProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func TestLLMClassifier_PromptLoader_OverridesConst(t *testing.T) {
	prov := &recordingProvider{response: `{"intent":"chat","confidence":0.9,"reasoning":"x"}`}
	const custom = "PROMPT EDITADO DESDE EL DASHBOARD — clasificá todo como chat"
	c := &promptrouter.LLMClassifier{
		Provider: prov,
		PromptLoader: func(_ context.Context) (string, error) {
			return custom, nil
		},
	}
	_, _, _, err := c.Classify(context.Background(), "el botón no funciona")
	require.NoError(t, err)
	require.Equal(t, custom, prov.gotSystemPrompt, "el loader debe pisar el const")
}

func TestLLMClassifier_PromptLoader_EmptyFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: `{"intent":"chat","confidence":0.9,"reasoning":"x"}`}
	c := &promptrouter.LLMClassifier{
		Provider: prov,
		// Loader devuelve vacío → usa el const por defecto.
		PromptLoader: func(_ context.Context) (string, error) {
			return "   ", nil
		},
	}
	_, _, _, err := c.Classify(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, promptrouter.DefaultTriageSystemPrompt, prov.gotSystemPrompt)
}

func TestLLMClassifier_PromptLoader_ErrorFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: `{"intent":"chat","confidence":0.9,"reasoning":"x"}`}
	c := &promptrouter.LLMClassifier{
		Provider: prov,
		PromptLoader: func(_ context.Context) (string, error) {
			return "", context.DeadlineExceeded
		},
	}
	_, _, _, err := c.Classify(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, promptrouter.DefaultTriageSystemPrompt, prov.gotSystemPrompt)
}

func TestLLMClassifier_NilLoader_UsesConst(t *testing.T) {
	prov := &recordingProvider{response: `{"intent":"chat","confidence":0.9,"reasoning":"x"}`}
	c := &promptrouter.LLMClassifier{Provider: prov}
	_, _, _, err := c.Classify(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, promptrouter.DefaultTriageSystemPrompt, prov.gotSystemPrompt)
}

func TestParseIntent(t *testing.T) {
	require.Nil(t, promptrouter.ParseIntent(""))
	require.Nil(t, promptrouter.ParseIntent("garbage"))
	got := promptrouter.ParseIntent("FIX")
	require.NotNil(t, got)
	require.Equal(t, promptrouter.IntentFix, *got)
	got = promptrouter.ParseIntent("  analysis  ")
	require.NotNil(t, got)
	require.Equal(t, promptrouter.IntentAnalysis, *got)
}

// spyClassifier registra si fue invocado. Sirve para probar que el intent
// override SALTEA la clasificación.
type spyClassifier struct {
	called bool
	intent promptrouter.Intent
}

func (s *spyClassifier) Classify(_ context.Context, _ string) (promptrouter.Intent, float64, string, error) {
	s.called = true
	return s.intent, 1.0, "spy", nil
}

func TestRouteWithIntent_Override_SkipsClassification(t *testing.T) {
	spy := &spyClassifier{intent: promptrouter.IntentFeature}
	r := &promptrouter.Router{Classifier: spy}

	// Override a 'chat' → Route responde directo sin tocar services ni clasificar.
	override := promptrouter.IntentChat
	resp, err := r.RouteWithIntent(context.Background(), "el botón no funciona", nil, nil, nil, &override)
	require.NoError(t, err)
	require.False(t, spy.called, "el classifier NO debe invocarse cuando hay override")
	require.Equal(t, promptrouter.IntentChat, resp.Intent)
	require.Equal(t, promptrouter.OutcomeChat, resp.Outcome)
}

func TestRouteWithIntent_NilOverride_Classifies(t *testing.T) {
	spy := &spyClassifier{intent: promptrouter.IntentChat}
	r := &promptrouter.Router{Classifier: spy}

	resp, err := r.RouteWithIntent(context.Background(), "cualquier cosa", nil, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, spy.called, "sin override el classifier debe invocarse")
	require.Equal(t, promptrouter.IntentChat, resp.Intent)
}

func TestRouteWithIntent_InvalidOverride_FallsBackToClassifier(t *testing.T) {
	spy := &spyClassifier{intent: promptrouter.IntentChat}
	r := &promptrouter.Router{Classifier: spy}

	// Un *Intent con valor inválido NO es del enum → se ignora, clasifica.
	bad := promptrouter.Intent("garbage")
	resp, err := r.RouteWithIntent(context.Background(), "x", nil, nil, nil, &bad)
	require.NoError(t, err)
	require.True(t, spy.called)
	require.Equal(t, promptrouter.IntentChat, resp.Intent)
}
