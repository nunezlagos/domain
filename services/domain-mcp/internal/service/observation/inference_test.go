package observation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestParseInferenceDecisions(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantLen int
		wantErr bool
	}{
		{
			name:    "clean json",
			raw:     `{"results":[{"source_id":"a","target_id":"b","edge_type":"supersedes","confidence":0.9,"reason":"x"}]}`,
			wantLen: 1,
		},
		{
			name:    "with prose around",
			raw:     "Acá va:\n{\"results\":[{\"source_id\":\"a\",\"target_id\":\"b\",\"edge_type\":\"none\"}]}\nlisto",
			wantLen: 1,
		},
		{
			name:    "fenced",
			raw:     "```json\n{\"results\":[]}\n```",
			wantLen: 0,
		},
		{"empty", "", 0, true},
		{"no json", "no hay json", 0, true},
		{"unterminated", `{"results":[`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseInferenceDecisions(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != c.wantLen {
				t.Fatalf("len mismatch: got %d want %d", len(got), c.wantLen)
			}
		})
	}
}

func TestValidEdgeType(t *testing.T) {
	valid := []string{"supersedes", "contradicts", "derived_from", "depends_on", "relates_to"}
	for _, v := range valid {
		if !validEdgeType(v) {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range []string{"none", "", "foo", "RELATES_TO"} {
		if validEdgeType(v) {
			t.Errorf("%q should be invalid", v)
		}
	}
}

func TestInferenceConfidence(t *testing.T) {
	cases := []struct {
		in   float64
		want float32
	}{
		{0, 0.6},
		{-1, 0.6},
		{1.5, 0.6},
		{0.8, 0.8},
		{1, 1},
	}
	for _, c := range cases {
		if got := inferenceConfidence(c.in); got != c.want {
			t.Errorf("inferenceConfidence(%v)=%v want %v", c.in, got, c.want)
		}
	}
}

// fakeLinker captura los Link y permite simular ErrEdgeExists.
type fakeLinker struct {
	calls   []LinkInput
	existAt map[string]bool // pairKey -> devolver ErrEdgeExists
}

func (f *fakeLinker) Link(_ context.Context, in LinkInput) (*Edge, error) {
	f.calls = append(f.calls, in)
	if f.existAt[pairKey(in.SourceID, in.TargetID)] {
		return nil, ErrEdgeExists
	}
	return &Edge{ID: uuid.New(), SourceID: in.SourceID, TargetID: in.TargetID, EdgeType: in.EdgeType}, nil
}

func TestApplyInferenceDecisions(t *testing.T) {
	s := &Service{}
	a, b := uuid.New(), uuid.New()
	c := uuid.New()
	pairs := []CandidatePair{
		{SourceID: a, TargetID: b},
		{SourceID: a, TargetID: c},
	}

	hallucinated := uuid.New()
	decisions := []inferenceDecision{
		{SourceID: a.String(), TargetID: b.String(), EdgeType: "supersedes", Confidence: 0.9, Reason: "r1"},
		{SourceID: a.String(), TargetID: c.String(), EdgeType: "none"},                  // skipped
		{SourceID: a.String(), TargetID: hallucinated.String(), EdgeType: "relates_to"}, // par no enviado -> ignorado
		{SourceID: "bad-uuid", TargetID: b.String(), EdgeType: "relates_to"},            // uuid invalido -> ignorado
		{SourceID: b.String(), TargetID: a.String(), EdgeType: "weird_type"},            // tipo invalido (par valido invertido) -> skipped
	}

	linker := &fakeLinker{}
	res, err := s.applyInferenceDecisions(context.Background(), linker, pairs, decisions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Created != 1 {
		t.Errorf("Created=%d want 1", res.Created)
	}
	if res.Skipped != 2 { // "none" + tipo invalido
		t.Errorf("Skipped=%d want 2", res.Skipped)
	}
	if len(linker.calls) != 1 {
		t.Errorf("Link calls=%d want 1 (solo el par valido con tipo valido)", len(linker.calls))
	}
	if res.Candidates != len(decisions) {
		t.Errorf("Candidates=%d want %d", res.Candidates, len(decisions))
	}
}

func TestApplyInferenceDecisions_IdempotentExisting(t *testing.T) {
	s := &Service{}
	a, b := uuid.New(), uuid.New()
	pairs := []CandidatePair{{SourceID: a, TargetID: b}}
	decisions := []inferenceDecision{
		{SourceID: a.String(), TargetID: b.String(), EdgeType: "relates_to", Reason: "r"},
	}
	linker := &fakeLinker{existAt: map[string]bool{pairKey(a, b): true}}
	res, err := s.applyInferenceDecisions(context.Background(), linker, pairs, decisions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Existing != 1 {
		t.Errorf("Existing=%d want 1", res.Existing)
	}
	if res.Created != 0 {
		t.Errorf("Created=%d want 0", res.Created)
	}
}

// TestInferEdgesLLM_DegradesWithoutMiniMax verifica que sin Factory LLM (sin
// MINIMAX_API_KEY) InferEdgesLLM devuelve ErrInferenceUnavailable y no crashea.
func TestInferEdgesLLM_DegradesWithoutMiniMax(t *testing.T) {
	s := &Service{} // LLM == nil
	_, err := s.InferEdgesLLM(context.Background(), &fakeLinker{}, InferEdgesLLMInput{
		ProjectID: uuid.New(),
	})
	if !errors.Is(err, ErrInferenceUnavailable) {
		t.Fatalf("want ErrInferenceUnavailable, got %v", err)
	}
}

func TestClipAndOneLine(t *testing.T) {
	if got := clip("abcdef", 3); got != "abc" {
		t.Errorf("clip got %q", got)
	}
	if got := clip("ab", 5); got != "ab" {
		t.Errorf("clip short got %q", got)
	}
	if got := oneLine("a\nb\rc"); got != "a b c" {
		t.Errorf("oneLine got %q", got)
	}
}
