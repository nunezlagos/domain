package skill_suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// fakeRefiner implementa Refiner para los tests sin LLM.
type fakeRefiner struct {
	out string
	err error
}

func (f fakeRefiner) RefineContent(_ context.Context, _, _, _ string) (string, error) {
	return f.out, f.err
}

func TestValidKind(t *testing.T) {
	for _, k := range []string{KindSplit, KindMerge, KindRefine, KindArchive} {
		if !validKind(k) {
			t.Fatalf("kind %q debe ser valido", k)
		}
	}
	for _, k := range []string{"", "delete", "Split", "rewrite"} {
		if validKind(k) {
			t.Fatalf("kind %q NO debe ser valido", k)
		}
	}
}

func TestCreate_InvalidKindRejected(t *testing.T) {
	s := &Service{} // sin Pool: debe fallar ANTES de tocar la DB
	_, err := s.Create(context.Background(), CreateInput{SkillSlug: "x", Kind: "bogus"})
	if !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("esperaba ErrInvalidKind, obtuve %v", err)
	}
}

func TestPayloadHash_StableAcrossKeyOrder(t *testing.T) {
	a := []byte(`{"a":1,"b":2}`)
	b := []byte(`{"b":2,"a":1}`)
	if payloadHash(a) != payloadHash(b) {
		t.Fatal("el hash debe ser estable ante reordenamiento de claves (canonicalizacion)")
	}
	c := []byte(`{"a":1,"b":3}`)
	if payloadHash(a) == payloadHash(c) {
		t.Fatal("payloads distintos deben hashear distinto")
	}
}

func TestPayloadHash_NonJSONFallsBackToRaw(t *testing.T) {
	// No debe panic con bytes no-JSON; hashea crudo.
	h := payloadHash([]byte("not json"))
	if len(h) != 64 { // sha256 hex
		t.Fatalf("hash invalido: %q", h)
	}
}

func TestAuditValues_NoRawPayloadLeak(t *testing.T) {
	conf := 0.8
	model := "MiniMax-M3"
	s := &Suggestion{SkillSlug: "alpha", Kind: KindRefine, Status: StatusPending, LLMConfidence: &conf, LLMModel: &model}
	payload := []byte(`{"new_content":"SECRETO PII no debe filtrarse"}`)
	v := auditValues(s, payload)
	blob, _ := json.Marshal(v)
	if string(blob) == "" {
		t.Fatal("audit values vacios")
	}
	// El payload crudo NUNCA debe estar en el audit; solo su hash.
	if containsSubstring(string(blob), "SECRETO PII") {
		t.Fatal("el payload crudo se filtro al audit_log")
	}
	if v["payload_hash"] != payloadHash(payload) {
		t.Fatal("payload_hash incorrecto en audit values")
	}
	if v["skill_slug"] != "alpha" || v["kind"] != KindRefine {
		t.Fatal("audit values incompletos")
	}
}

func TestDedupStrings(t *testing.T) {
	in := []string{"a", "b", "a", "", "  c  ", "b"}
	got := dedupStrings(in)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedup: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedup[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestParseJudgeResponse_TolerantToProseAndFences(t *testing.T) {
	raw := "Claro, aca van:\n```json\n{\"suggestions\":[{\"kind\":\"refine\",\"confidence\":0.9,\"rationale\":\"falla mucho\",\"payload\":{\"instruction\":\"mejorar\"}}]}\n```\nEso es todo."
	out, err := parseJudgeResponse(raw)
	if err != nil {
		t.Fatalf("parse fallo: %v", err)
	}
	if len(out) != 1 || out[0].Kind != "refine" || out[0].Confidence != 0.9 {
		t.Fatalf("parse incorrecto: %+v", out)
	}
}

func TestParseJudgeResponse_EmptyAndMalformed(t *testing.T) {
	if _, err := parseJudgeResponse(""); err == nil {
		t.Fatal("respuesta vacia debe fallar")
	}
	if _, err := parseJudgeResponse("sin json aca"); err == nil {
		t.Fatal("sin objeto JSON debe fallar")
	}
}

func TestJudge_Evaluate_FiltersByThresholdAndKind(t *testing.T) {
	// Sin LLM no podemos correr Evaluate end-to-end; validamos el filtrado
	// que aplica sobre una respuesta cruda simulada via parse + el mismo criterio.
	raw := `{"suggestions":[
	  {"kind":"refine","confidence":0.7,"rationale":"ok","payload":{"instruction":"x"}},
	  {"kind":"archive","confidence":0.4,"rationale":"baja conf","payload":{"reason":"y"}},
	  {"kind":"bogus","confidence":0.99,"rationale":"kind malo","payload":{}}
	]}`
	parsed, err := parseJudgeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, sg := range parsed {
		if validKind(sg.Kind) && sg.Confidence >= ConfidenceThreshold {
			kept++
		}
	}
	if kept != 1 {
		t.Fatalf("esperaba 1 sugerencia valida (>=0.6 y kind valido), obtuve %d", kept)
	}
}

func TestJudge_Unavailable_WithoutLLM(t *testing.T) {
	j := &LLMJudge{} // sin Factory
	if j.Available() {
		t.Fatal("judge sin Factory no debe estar disponible")
	}
	_, err := j.Evaluate(context.Background(), SkillInput{Slug: "x"})
	if !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("esperaba ErrJudgeUnavailable, obtuve %v", err)
	}
	_, err = j.RefineContent(context.Background(), "x", "c", "i")
	if !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("RefineContent esperaba ErrJudgeUnavailable, obtuve %v", err)
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
