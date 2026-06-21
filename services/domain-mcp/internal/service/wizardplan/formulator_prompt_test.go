package wizardplan_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/wizardplan"
)

// recordingProvider captura el SystemPrompt recibido para verificar qué
// prompt usó el formulator (const vs loader).
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

func formulateWith(t *testing.T, prov llm.Provider, loader func(context.Context) (string, error)) {
	t.Helper()
	f := &wizardplan.LLMQuestionFormulator{Provider: prov, PromptLoader: loader}
	_, err := f.FormulateQuestion(context.Background(), wizardplan.FormulateInput{
		SlotKey:  wizardplan.SlotIntent,
		Envelope: wizardplan.NewEnvelope("el botón no funciona", "fix"),
	})
	require.NoError(t, err)
}

func TestFormulator_PromptLoader_OverridesConst(t *testing.T) {
	prov := &recordingProvider{response: "¿Qué tipo de cambio es?"}
	const custom = "PROMPT EDITADO DESDE EL DASHBOARD — preguntá corto"
	formulateWith(t, prov, func(_ context.Context) (string, error) {
		return custom, nil
	})
	require.Equal(t, custom, prov.gotSystemPrompt, "el loader debe pisar el const")
}

func TestFormulator_PromptLoader_EmptyFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: "ok"}
	formulateWith(t, prov, func(_ context.Context) (string, error) {
		return "   ", nil
	})
	require.Equal(t, wizardplan.DefaultFormulatorSystemPrompt, prov.gotSystemPrompt)
}

func TestFormulator_PromptLoader_ErrorFallsBackToConst(t *testing.T) {
	prov := &recordingProvider{response: "ok"}
	formulateWith(t, prov, func(_ context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	require.Equal(t, wizardplan.DefaultFormulatorSystemPrompt, prov.gotSystemPrompt)
}

func TestFormulator_NilLoader_UsesConst(t *testing.T) {
	prov := &recordingProvider{response: "ok"}
	formulateWith(t, prov, nil)
	require.Equal(t, wizardplan.DefaultFormulatorSystemPrompt, prov.gotSystemPrompt)
}
