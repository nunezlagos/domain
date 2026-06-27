package observation

import (
	"testing"

	"github.com/google/uuid"
)

func mkResult(content string) SearchResult {
	return SearchResult{Observation: Observation{ID: uuid.New(), Content: content}}
}

func TestParseRerankIDs(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{"clean json", `{"order":["a","b","c"]}`, []string{"a", "b", "c"}, false},
		{"with prose", "Acá está:\n{\"order\":[\"x\",\"y\"]}\ngracias", []string{"x", "y"}, false},
		{"fenced", "```json\n{\"order\":[\"1\"]}\n```", []string{"1"}, false},
		{"empty", "", nil, true},
		{"no json", "no hay json aca", nil, true},
		{"unterminated", `{"order":["a"`, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseRerankIDs(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("len mismatch: got %v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("idx %d: got %q want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestReorderByIDs(t *testing.T) {
	a := mkResult("a")
	b := mkResult("b")
	c := mkResult("c")
	candidates := []SearchResult{a, b, c}

	t.Run("full reorder", func(t *testing.T) {
		out := reorderByIDs(candidates, []string{c.ID.String(), a.ID.String(), b.ID.String()})
		assertOrder(t, out, c, a, b)
	})

	t.Run("partial order appends omitted in original order", func(t *testing.T) {
		// El modelo solo rankeó c; a y b se anexan en orden original.
		out := reorderByIDs(candidates, []string{c.ID.String()})
		assertOrder(t, out, c, a, b)
	})

	t.Run("hallucinated ids ignored", func(t *testing.T) {
		out := reorderByIDs(candidates, []string{"no-existe", b.ID.String(), uuid.NewString()})
		// b primero (válido), luego a y c en orden original.
		assertOrder(t, out, b, a, c)
	})

	t.Run("duplicate ids counted once", func(t *testing.T) {
		out := reorderByIDs(candidates, []string{a.ID.String(), a.ID.String(), b.ID.String()})
		assertOrder(t, out, a, b, c)
	})

	t.Run("empty order keeps original", func(t *testing.T) {
		out := reorderByIDs(candidates, nil)
		assertOrder(t, out, a, b, c)
	})

	t.Run("no results dropped", func(t *testing.T) {
		out := reorderByIDs(candidates, []string{c.ID.String()})
		if len(out) != len(candidates) {
			t.Fatalf("result count changed: got %d want %d", len(out), len(candidates))
		}
	})
}

// TestRerankWithLLMDegrades verifica que sin Factory inyectado el rerank degrada
// (ok=false) sin paniquear — la garantía de degradación elegante.
func TestRerankWithLLMDegradesWithoutFactory(t *testing.T) {
	s := &Service{} // LLM == nil
	out, ok := s.rerankWithLLM(t.Context(), "query", []SearchResult{mkResult("x")})
	if ok {
		t.Fatalf("expected ok=false when no LLM factory, got ok=true")
	}
	if out != nil {
		t.Fatalf("expected nil out on degrade, got %v", out)
	}
}

func assertOrder(t *testing.T, got []SearchResult, want ...SearchResult) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Fatalf("idx %d: got %q want %q", i, got[i].ID, want[i].ID)
		}
	}
}
