package skill_suggestions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
)

// fakeProvider implementa llm.Provider devolviendo un Content fijo (o error).
// Captura la ultima request para aserciones sobre el prompt.
type fakeProvider struct {
	content string
	err     error

	lastOpts llm.CompletionOptions
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) Complete(_ context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	p.lastOpts = opts
	if p.err != nil {
		return nil, p.err
	}
	return &llm.Response{Content: p.content, Model: opts.Model, FinishReason: "stop"}, nil
}

func (p *fakeProvider) CompleteStream(_ context.Context, _ llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

// judgeWith arma un LLMJudge cuyo provider 'minimax' devuelve `content`.
func judgeWith(content string, err error) (*LLMJudge, *fakeProvider) {
	f := llm.NewFactory()
	fp := &fakeProvider{content: content, err: err}
	f.Register(anthropic.MiniMaxProviderName, fp)
	return &LLMJudge{LLM: f}, fp
}

func TestJudge_Available_WithProvider(t *testing.T) {
	j, _ := judgeWith(`{"suggestions":[]}`, nil)
	if !j.Available() {
		t.Fatal("judge con provider minimax registrado debe estar disponible")
	}
}

// TestJudge_Evaluate_EachKind: el judge produce una sugerencia tipada por cada
// kind valido cuando el LLM la devuelve por encima del umbral.
func TestJudge_Evaluate_EachKind(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		payload string
	}{
		{"split", KindSplit, `{"children":[{"slug":"a","instruction":"x"},{"slug":"b","instruction":"y"}]}`},
		{"merge", KindMerge, `{"with":["otro"],"merged_slug":"m"}`},
		{"refine", KindRefine, `{"instruction":"mejorar","changelog":"c"}`},
		{"archive", KindArchive, `{"reason":"sin uso"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"suggestions":[{"kind":"` + tc.kind + `","confidence":0.85,"rationale":"r","payload":` + tc.payload + `}]}`
			j, _ := judgeWith(raw, nil)
			out, err := j.Evaluate(context.Background(), SkillInput{Slug: "target", Name: "Target"})
			if err != nil {
				t.Fatalf("Evaluate fallo: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("esperaba 1 sugerencia, obtuve %d", len(out))
			}
			ci := out[0]
			if ci.Kind != tc.kind {
				t.Fatalf("kind: got %q want %q", ci.Kind, tc.kind)
			}
			if ci.SkillSlug != "target" {
				t.Fatalf("skill_slug: got %q want target", ci.SkillSlug)
			}
			if ci.Confidence == nil || *ci.Confidence != 0.85 {
				t.Fatalf("confidence incorrecta: %v", ci.Confidence)
			}
			if ci.LLMModel == nil || *ci.LLMModel != anthropic.MiniMaxModel {
				t.Fatalf("llm_model esperado %q, got %v", anthropic.MiniMaxModel, ci.LLMModel)
			}
			if len(ci.Payload) == 0 || string(ci.Payload) == "{}" {
				t.Fatalf("payload vacio para kind %s", tc.kind)
			}
		})
	}
}

// TestJudge_Evaluate_DropsBelowThresholdAndBadKind end-to-end (no solo parse).
func TestJudge_Evaluate_DropsBelowThresholdAndBadKind(t *testing.T) {
	raw := `{"suggestions":[
	  {"kind":"refine","confidence":0.59,"rationale":"justo abajo","payload":{"instruction":"x"}},
	  {"kind":"refine","confidence":0.60,"rationale":"justo en el umbral","payload":{"instruction":"y"}},
	  {"kind":"teleport","confidence":0.99,"rationale":"kind alucinado","payload":{}}
	]}`
	j, _ := judgeWith(raw, nil)
	out, err := j.Evaluate(context.Background(), SkillInput{Slug: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("esperaba 1 (solo >=0.6 y kind valido), obtuve %d: %+v", len(out), out)
	}
	if out[0].Kind != KindRefine || *out[0].Confidence != 0.60 {
		t.Fatalf("sugerencia incorrecta sobrevivio: %+v", out[0])
	}
}

// TestJudge_Evaluate_NeverArchivesSeed: ARCHIVE de un seed_managed se descarta
// aunque el LLM lo proponga con alta confianza (refuerzo de regla dura).
func TestJudge_Evaluate_NeverArchivesSeed(t *testing.T) {
	raw := `{"suggestions":[{"kind":"archive","confidence":0.95,"rationale":"sin uso","payload":{"reason":"x"}}]}`
	j, _ := judgeWith(raw, nil)
	out, err := j.Evaluate(context.Background(), SkillInput{Slug: "seed-skill", SeedManaged: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("ARCHIVE de seed NO debe persistirse, obtuve %d", len(out))
	}
}

// TestJudge_Evaluate_EmptyListWhenNothingApplies: el judge tolera lista vacia.
func TestJudge_Evaluate_EmptyList(t *testing.T) {
	j, _ := judgeWith(`{"suggestions":[]}`, nil)
	out, err := j.Evaluate(context.Background(), SkillInput{Slug: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("esperaba 0 sugerencias, obtuve %d", len(out))
	}
}

// TestJudge_Evaluate_LLMErrorPropagates: si el provider falla, Evaluate falla
// (no inventa sugerencias).
func TestJudge_Evaluate_LLMErrorPropagates(t *testing.T) {
	j, _ := judgeWith("", errors.New("boom"))
	_, err := j.Evaluate(context.Background(), SkillInput{Slug: "s"})
	if err == nil {
		t.Fatal("esperaba error del LLM")
	}
	if errors.Is(err, ErrJudgeUnavailable) {
		t.Fatal("un fallo de Complete no es 'unavailable'; el provider SI estaba")
	}
}

// TestJudge_PromptIncludesSignals: el prompt user transporta las senales y los
// similares al modelo (asi el LLM puede aplicar las reglas).
func TestJudge_PromptIncludesSignals(t *testing.T) {
	j, fp := judgeWith(`{"suggestions":[]}`, nil)
	in := SkillInput{
		Slug: "alpha", Name: "Alpha", Description: "hace algo",
		Content: "contenido largo", SeedManaged: false,
		InvocationsPerDay: 12.5, FailureRate: 42.0, AvgDurationSeconds: 33.0,
		NegativeFeedback: 5, DaysSinceLastUse: 91,
		Similar: []SimilarSkill{{Slug: "beta", Name: "Beta", Score: 0.7}},
	}
	if _, err := j.Evaluate(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	user := ""
	if len(fp.lastOpts.Messages) > 0 {
		user = fp.lastOpts.Messages[0].Content
	}
	for _, want := range []string{"alpha", "42.0", "33.0", "beta", "91"} {
		if !strings.Contains(user, want) {
			t.Fatalf("el prompt user no contiene %q\n---\n%s", want, user)
		}
	}
	// Determinismo: temperature 0.
	if fp.lastOpts.Temperature != 0 {
		t.Fatalf("temperature debe ser 0, got %v", fp.lastOpts.Temperature)
	}
}

// TestJudge_RefineContent_RoundTrip: RefineContent devuelve el content generado.
func TestJudge_RefineContent_RoundTrip(t *testing.T) {
	j, _ := judgeWith("  NUEVO CONTENIDO  ", nil)
	out, err := j.RefineContent(context.Background(), "alpha", "viejo", "mejoralo")
	if err != nil {
		t.Fatal(err)
	}
	if out != "NUEVO CONTENIDO" {
		t.Fatalf("RefineContent: got %q", out)
	}
}

// TestJudge_RefineContent_EmptyIsError: content vacio del LLM => error (no pisa
// el skill con vacio).
func TestJudge_RefineContent_EmptyIsError(t *testing.T) {
	j, _ := judgeWith("   ", nil)
	if _, err := j.RefineContent(context.Background(), "a", "c", "i"); err == nil {
		t.Fatal("content vacio del LLM debe ser error")
	}
}

// TestAggregator_DegradesWithoutLLM: sin judge disponible, Run devuelve
// ErrJudgeUnavailable SIN tocar la DB (Pool nil no debe panic). Regla dura 7.
func TestAggregator_DegradesWithoutLLM(t *testing.T) {
	// Pool no-nil (nunca se usa: Run degrada en el check del judge ANTES de
	// tocar la DB). Si el orden cambiara y se usara, el pool vacio paniquea -> el
	// test atrapa la regresion.
	agg := &Aggregator{
		Pool:    &pgxpool.Pool{},
		Service: &Service{},
		Judge:   &LLMJudge{}, // sin Factory -> no disponible
	}
	_, err := agg.Run(context.Background())
	if !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("esperaba ErrJudgeUnavailable, obtuve %v", err)
	}
}

// TestAggregator_RequiresServiceAndPool: validacion de dependencias.
func TestAggregator_RequiresServiceAndPool(t *testing.T) {
	j, _ := judgeWith(`{"suggestions":[]}`, nil)
	agg := &Aggregator{Pool: nil, Service: nil, Judge: j}
	if _, err := agg.Run(context.Background()); err == nil {
		t.Fatal("esperaba error por dependencias faltantes")
	}
}

func TestSignalMath(t *testing.T) {
	if got := failureRate(0, 0); got != 0 {
		t.Fatalf("failureRate sin invocaciones debe ser 0, got %v", got)
	}
	if got := failureRate(100, 30); got != 30 {
		t.Fatalf("failureRate 30/100 debe ser 30, got %v", got)
	}
	if got := avgDurationSeconds(0, 0); got != 0 {
		t.Fatalf("avgDuration sin invocaciones debe ser 0, got %v", got)
	}
	// 10 invocaciones, peso total = 50000 ms => 5000ms/inv => 5s.
	if got := avgDurationSeconds(10, 50000); got != 5 {
		t.Fatalf("avgDuration: got %v want 5", got)
	}
}
