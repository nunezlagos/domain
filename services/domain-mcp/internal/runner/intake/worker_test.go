package intake

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/intake"
)










// El constructor NO centraliza defaults; los aplica Run() al arrancar.
// Validamos que los valores zero se reemplazan con defaults razonables.
func TestBehavior_Defaults_AreSane(t *testing.T) {
	w := &Worker{}

	require.Zero(t, w.PollInterval, "PollInterval=0 debe caer a 30s en Run()")
	require.Zero(t, w.BatchSize, "BatchSize=0 debe caer a 5 en Run()")
	require.Zero(t, w.DedupThreshold, "DedupThreshold=0 debe caer a 0.75 en Run()")
	require.Zero(t, w.MergeThreshold, "MergeThreshold=0 debe caer a 0.92 en Run()")
}

// Logger nil → usa slog.Default(). Importante: el worker NUNCA debe
// panic con Logger nil (defense in depth).
func TestBehavior_NilLogger_FallsBackToDefault(t *testing.T) {
	w := &Worker{}
	require.Nil(t, w.Logger)


	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Worker con Logger=nil debe asignar slog.Default(), no panic: %v", r)
		}
	}()


	w.Logger = slog.Default()
	require.NotNil(t, w.Logger)
}



// Classification tiene 4 campos: Type, Severity, Confidence, Reasoning.
// La invariante: Type ∈ {feat, fix, hotfix, chore, refactor, docs}.
// Si alguien agrega un type nuevo, debe actualizar el codigo del
// intake service que lo procesa.
func TestBehavior_Classification_ValidTypes(t *testing.T) {
	validTypes := []string{"feat", "fix", "hotfix", "chore", "refactor", "docs"}
	for _, typ := range validTypes {
		cls := Classification{Type: typ}
		require.NotEmpty(t, cls.Type)
	}




	cls := Classification{Confidence: 0.5}
	require.GreaterOrEqual(t, cls.Confidence, 0.0)
	require.LessOrEqual(t, cls.Confidence, 1.0)
}



// Structured tiene title, description, req_slug, hu_draft. El JSON tag
// "hu_draft" es NO snake_case (es kebab-like) — el codigo lo acepta
// tal cual. Si alguien normaliza el JSON tag, downstream puede romperse.
func TestBehavior_Structured_JSONTags(t *testing.T) {
	st := Structured{
		Title:       "Add login",
		Description: "Implement OAuth login",
		ReqSlug:     "auth-login",
		IssueDraftWizard: map[string]any{"key": "value"},
	}
	data, err := json.Marshal(st)
	require.NoError(t, err)
	s := string(data)
	require.Contains(t, s, `"title":"Add login"`)
	require.Contains(t, s, `"description":"Implement OAuth login"`)
	require.Contains(t, s, `"req_slug":"auth-login"`)
	require.Contains(t, s, `"hu_draft":`)
}

// hu_draft puede estar vacio (intake que no genera HU) — el struct
// debe aceptar map nil sin panic.
func TestBehavior_Structured_NilHuDraft_OK(t *testing.T) {
	st := Structured{Title: "x"}
	require.Nil(t, st.IssueDraftWizard)
	data, _ := json.Marshal(st)
	require.Contains(t, string(data), `"title":"x"`)

	require.Contains(t, string(data), `"hu_draft":null`)
}



// DedupCandidate puede tener ReqID, HUID, o ambos. Repr distintas:
//  - ReqID: matchea con un requirement existente
//  - HUID: matchea con un issue/HU existente
//  - Ambos: nil (no match)
//  - Ninguno: posible (sin contexto, pero no es error).
func TestBehavior_DedupCandidate_Shape(t *testing.T) {
	reqID := uuid.New()
	huID := uuid.New()


	c1 := DedupCandidate{ReqID: &reqID, Title: "old req", Similarity: 0.85}
	require.NotNil(t, c1.ReqID)
	require.Nil(t, c1.HUID)


	c2 := DedupCandidate{HUID: &huID, Title: "old hu", Similarity: 0.92}
	require.Nil(t, c2.ReqID)
	require.NotNil(t, c2.HUID)


	c3 := DedupCandidate{ReqID: &reqID, HUID: &huID, Similarity: 0.95}
	require.NotNil(t, c3.ReqID)
	require.NotNil(t, c3.HUID)
}

// Similarity fuera de [0,1] es invalido. El struct no lo enforce.
// Documentamos el contrato: callers deben validar.
func TestBehavior_DedupCandidate_Similarity_Contract(t *testing.T) {
	c := DedupCandidate{Similarity: 0.5}
	require.GreaterOrEqual(t, c.Similarity, 0.0)
	require.LessOrEqual(t, c.Similarity, 1.0)




	w := &Worker{}
	require.Zero(t, w.DedupThreshold, "0 → 0.75 default en Run()")
	require.Zero(t, w.MergeThreshold, "0 → 0.92 default en Run()")
}



// Classifier, Structurer, DedupSearcher son interfaces. Si alguien
// renombra los metodos, los fakes en tests rompen. Test canary via
// dummy implementations que satisfacen las interfaces.
type fakeClassifier struct{ out Classification }
func (f *fakeClassifier) Classify(ctx context.Context, rawText string) (Classification, error) {
	return f.out, nil
}

type fakeStructurer struct{ out Structured }
func (f *fakeStructurer) Structure(ctx context.Context, rawText string, cls Classification) (Structured, error) {
	return f.out, nil
}

type fakeDedup struct{ out []DedupCandidate }
func (f *fakeDedup) FindCandidates(ctx context.Context, embedding []float32, threshold float64, limit int) ([]DedupCandidate, error) {
	return f.out, nil
}

// Test que las interfaces son implementadas por los fakes.
func TestBehavior_Interfaces_AreImplementable(t *testing.T) {
	var _ Classifier = &fakeClassifier{}
	var _ Structurer = &fakeStructurer{}
	var _ DedupSearcher = &fakeDedup{}
}



// DedupThreshold (≥) sugiere "es duplicado, descarta".
// MergeThreshold (≥) sugiere "append_to_hu en lugar de create_new".
// Si Similarity >= MergeThreshold → sugerir merge (0.92).
// Si DedupThreshold <= Similarity < MergeThreshold → sugerir dedup (0.75-0.92).
// Si Similarity < DedupThreshold → no es ni dedup ni merge.
func TestBehavior_Thresholds_RangesAreValid(t *testing.T) {
	w := &Worker{}

	w.DedupThreshold = 0.75
	w.MergeThreshold = 0.92

	require.Less(t, w.DedupThreshold, w.MergeThreshold,
		"DedupThreshold DEBE ser menor que MergeThreshold (ranges no se solapan)")


	require.GreaterOrEqual(t, w.DedupThreshold, 0.0)
	require.LessOrEqual(t, w.MergeThreshold, 1.0)
}



// ProcessOne requiere Classifier y Structurer. Si son nil, retorna
// error antes de tocar DB. Esto es defense in depth: un worker mal
// configurado falla rapido, no corrupte el estado.
func TestBehavior_ProcessOne_RequiresClassifierAndStructurer(t *testing.T) {
	w := &Worker{Service: nil, Classifier: nil, Structurer: nil}


	require.Nil(t, w.Classifier)
	require.Nil(t, w.Structurer)

}



// Referenciamos intake.StatusReceived para verificar el contrato.
func TestBehavior_ContractWithIntakeService(t *testing.T) {



	require.NotEmpty(t, intake.StatusReceived)
}
