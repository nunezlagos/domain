package capturedprompt

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestHeatmap_ClusterOverThreshold_EmitsSuggestionWithoutPersisting(t *testing.T) {
	repo := newMockRepo()
	repo.clusters = []PromptCluster{
		{Key: "arreglar el login", Turns: 4, Tokens: 900, Sample: "arreglar el login que no anda"},
		{Key: "siempre validar input", Turns: 5, Tokens: 700, Sample: "siempre validar el input del usuario"},
		{Key: "consulta puntual", Turns: 1, Tokens: 50, Sample: "consulta puntual"},
	}
	svc := NewService(repo)

	res, err := svc.Heatmap(context.Background(), uuid.New(), uuid.New(), 0, 0)
	if err != nil {
		t.Fatalf("Heatmap: %v", err)
	}
	if len(res.Clusters) != 3 {
		t.Fatalf("Clusters=%d want 3", len(res.Clusters))
	}
	// solo los 2 clusters con turns>=3 generan sugerencia; el de turns=1 no
	if len(res.Suggestions) != 2 {
		t.Fatalf("Suggestions=%d want 2", len(res.Suggestions))
	}
	// clasificación: regla -> policy; tarea -> skill
	var gotPolicy, gotSkill bool
	for _, s := range res.Suggestions {
		switch s.Kind {
		case "policy":
			gotPolicy = true
		case "skill":
			gotSkill = true
		}
	}
	if !gotPolicy || !gotSkill {
		t.Errorf("esperaba una sugerencia policy y una skill; got policy=%v skill=%v", gotPolicy, gotSkill)
	}

	// read-only: el heatmap NO persiste (no Insert ni CompleteTurn)
	if len(repo.inserted) != 0 || len(repo.completed) != 0 {
		t.Errorf("heatmap no debe persistir: inserted=%d completed=%d", len(repo.inserted), len(repo.completed))
	}
}
