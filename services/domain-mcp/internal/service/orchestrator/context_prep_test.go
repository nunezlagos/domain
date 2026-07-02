package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
)

// fakeProvider implementa llm.Provider para tests de refineWithMinimax.
type fakeProvider struct {
	resp    string
	err     error
	called  bool
	gotOpts llm.CompletionOptions
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Complete(_ context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	f.called = true
	f.gotOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return &llm.Response{Content: f.resp}, nil
}
func (f *fakeProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestRefineWithMinimax_NoLLM_ReturnsRaw(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test") // LLM nil
	raw := "### Policies\n- x"
	require.Equal(t, raw, s.refineWithMinimax(context.Background(), "sdd-apply", raw),
		"sin LLM factory debe degradar al bloque crudo")
}

func TestRefineWithMinimax_ProviderMissing_ReturnsRaw(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test")
	s.LLM = llm.NewFactory() // factory vacío: Get("minimax") falla
	raw := "### Policies\n- x"
	require.Equal(t, raw, s.refineWithMinimax(context.Background(), "sdd-apply", raw),
		"sin provider minimax (falta LLM_API_KEY) debe degradar a crudo")
}

func TestRefineWithMinimax_ProviderError_ReturnsRaw(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test")
	f := llm.NewFactory()
	f.Register(anthropic.MiniMaxProviderName, &fakeProvider{err: errors.New("timeout")})
	s.LLM = f
	raw := "### Policies\n- x"
	require.Equal(t, raw, s.refineWithMinimax(context.Background(), "sdd-apply", raw),
		"error/timeout del provider debe degradar a crudo, NO abortar")
}

func TestRefineWithMinimax_EmptyResponse_ReturnsRaw(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test")
	f := llm.NewFactory()
	f.Register(anthropic.MiniMaxProviderName, &fakeProvider{resp: "   "})
	s.LLM = f
	raw := "### Policies\n- x"
	require.Equal(t, raw, s.refineWithMinimax(context.Background(), "sdd-apply", raw),
		"respuesta vacía debe degradar a crudo")
}

func TestRefineWithMinimax_Success_ReturnsRefined(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test")
	fp := &fakeProvider{resp: "### Filtrado\n- solo lo pertinente"}
	f := llm.NewFactory()
	f.Register(anthropic.MiniMaxProviderName, fp)
	s.LLM = f
	out := s.refineWithMinimax(context.Background(), "sdd-apply", "### Crudo\n- todo")
	require.Equal(t, "### Filtrado\n- solo lo pertinente", out)
	require.True(t, fp.called)
	require.Equal(t, anthropic.MiniMaxModel, fp.gotOpts.Model, "debe usar el modelo MiniMax")
	require.Contains(t, fp.gotOpts.SystemPrompt, "sdd-apply", "el system prompt debe mencionar la fase")
}

func TestPrepareContext_UnconfiguredPhase_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test")
	// sdd-judge no está en prepPhaseContext → no-op
	require.Equal(t, "", s.prepareContext(context.Background(), uuid.Nil, uuid.Nil, "sdd-judge"))
}

func TestPrepareContextRaw_NoServices_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, nil, "test") // sin ProjectPolicies/Skills/Observations
	// Aunque la fase quiera policies+skills, sin servicios inyectados degrada a "".
	require.Equal(t, "", s.prepareContextRaw(context.Background(), uuid.Nil, uuid.Nil, true, true, true))
}

func TestInjectPreparedContext_PrependsBlock(t *testing.T) {
	t.Parallel()
	out := injectPreparedContext("PROMPT ORIGINAL", "### ctx\n- a")
	require.True(t, strings.HasPrefix(out, "## Contexto preparado por el servidor"))
	require.Contains(t, out, "### ctx")
	require.Contains(t, out, "PROMPT ORIGINAL")
	require.Less(t, strings.Index(out, "### ctx"), strings.Index(out, "PROMPT ORIGINAL"),
		"el contexto va ANTES del prompt original")
}

func TestFirstLine(t *testing.T) {
	t.Parallel()
	require.Equal(t, "hola", firstLine("hola\nmundo"))
	require.Equal(t, "hola", firstLine("\n\n  hola  \nmundo"))
	require.Equal(t, "", firstLine("   \n  "))
	long := strings.Repeat("a", 200)
	require.Len(t, firstLine(long), 160, "debe truncar a 160 chars")
}
