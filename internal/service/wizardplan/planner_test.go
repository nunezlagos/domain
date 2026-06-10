package wizardplan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	wp "nunezlagos/domain/internal/service/wizardplan"
)

func TestPlanner_NoPending_ReturnsNoMoreQuestions(t *testing.T) {
	env := wp.NewEnvelope("test prompt", "feature")
	for k := range env.Slots {
		env.Touch(k, "filled", "user", 1.0, "")
	}
	planner := &wp.Planner{}
	_, err := planner.NextQuestion(context.Background(), env)
	require.ErrorIs(t, err, wp.NoMoreQuestionsErr)
}

func TestPlanner_PrioritizesSeverityForBugFix(t *testing.T) {
	env := wp.NewEnvelope("el botón export no funciona", "fix")
	planner := &wp.Planner{}
	q, err := planner.NextQuestion(context.Background(), env)
	require.NoError(t, err)
	// Severity tiene prioridad 1 después de intent (que ya está provided por NewEnvelope+Touch).
	// Como intent NO está touched aún, va primero. Forzamos intent ya conocido:
	env.Touch(wp.SlotIntent, "fix", "intent_classifier", 0.9, "")
	q, err = planner.NextQuestion(context.Background(), env)
	require.NoError(t, err)
	require.Equal(t, wp.SlotSeverity, q.SlotKey)
}

func TestPlanner_TemplatePromptFallback(t *testing.T) {
	env := wp.NewEnvelope("idea cualquiera", "feature")
	env.Touch(wp.SlotIntent, "feature", "intent_classifier", 0.95, "")

	planner := &wp.Planner{} // sin LLM formulator → template fallback
	q, err := planner.NextQuestion(context.Background(), env)
	require.NoError(t, err)
	require.NotEmpty(t, q.Prompt)
	require.NotEqual(t, wp.SlotIntent, q.SlotKey, "intent ya touched")
}

func TestPlanner_LLMFormulator(t *testing.T) {
	env := wp.NewEnvelope("el director no descarga ficha", "fix")
	env.Touch(wp.SlotIntent, "fix", "intent_classifier", 0.92, "")
	env.HUMatches = &wp.HUDedupFinding{Candidates: []wp.HUDedupCandidate{
		{HUID: uuid.New(), Slug: "HU-04.6-s3-storage", Title: "S3 file attachments", Similarity: 0.72},
	}}
	env.Code = &wp.CodeGrepFinding{Hits: []wp.CodeHit{
		{Path: "internal/service/observation/service.go", Line: 421, Symbol: "Export", Category: "service"},
	}}

	formulator := &mockFormulator{response: "¿Cuán crítico es este bug? Encontré HU-04.6 (sim 0.72) y un service de Export."}
	planner := &wp.Planner{QuestionFormulator: formulator}

	q, err := planner.NextQuestion(context.Background(), env)
	require.NoError(t, err)
	require.Equal(t, wp.SlotSeverity, q.SlotKey)
	require.Contains(t, q.Prompt, "crítico")
}

func TestPlanner_LLMErrorFallsBackToTemplate(t *testing.T) {
	env := wp.NewEnvelope("hola", "feature")
	env.Touch(wp.SlotIntent, "feature", "intent_classifier", 0.95, "")
	formulator := &mockFormulator{err: errors.New("rate limit")}
	planner := &wp.Planner{QuestionFormulator: formulator}

	q, err := planner.NextQuestion(context.Background(), env)
	require.NoError(t, err)
	require.NotEmpty(t, q.Prompt, "fallback template debe devolver algo")
}

func TestPlanner_RecordAnswerMarksProvided(t *testing.T) {
	env := wp.NewEnvelope("x", "feature")
	planner := &wp.Planner{}
	planner.RecordAnswer(env, wp.SlotGoal, "exportar runs a CSV")
	require.Equal(t, wp.SlotProvided, env.Slots[wp.SlotGoal].Status)
}

func TestPlanner_HighConfidenceInferredIsConsideredKnown(t *testing.T) {
	env := wp.NewEnvelope("x", "feature")
	env.Touch(wp.SlotREQParent, "REQ-04", "hu_dedup", 0.85, "")
	_ = (&wp.Planner{}) // mantiene importado el package
	pending := env.PendingSlots(0.75)
	for _, p := range pending {
		require.NotEqual(t, wp.SlotREQParent, p, "REQParent con conf 0.85 NO debe estar pending con threshold 0.75")
	}
}

// Sabotaje: confidence inferida POR DEBAJO del threshold se considera pendiente.
func TestSabotage_LowConfidenceInferredStillPending(t *testing.T) {
	env := wp.NewEnvelope("x", "feature")
	env.Touch(wp.SlotREQParent, "REQ-XX", "hu_dedup", 0.40, "")
	pending := env.PendingSlots(0.75)
	found := false
	for _, p := range pending {
		if p == wp.SlotREQParent {
			found = true
		}
	}
	require.True(t, found, "conf 0.40 < threshold 0.75 → debe estar pending")
}

// mockFormulator
type mockFormulator struct {
	response string
	err      error
}

func (m *mockFormulator) FormulateQuestion(_ context.Context, _ wp.FormulateInput) (string, error) {
	return m.response, m.err
}
