package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// TestRecordPhaseResult_Full_MissingRequiredSave_StepRunningNotFailed recupera
// la cobertura adversarial sab-003 (DOMAINSERV-52) en el lugar correcto del
// contrato actual (REQ-56): un required save VIVO faltante deja el step
// RUNNING/reintentable con missing_required_saves poblado — NUNCA failed.
//
// Se ejercita en modo Full con sdd-judge (sabotage_record Required=true), que
// es un required save vivo hoy — a diferencia de Express, donde ninguna fase
// tiene required saves y el enforcement no se puede ejercitar.
func TestRecordPhaseResult_Full_MissingRequiredSave_StepRunningNotFailed(t *testing.T) {
	stepID := uuid.New()
	flowRunID := uuid.New()
	orgID := uuid.New()

	repo := &multiConcernRepo{
		flowRun: &FlowRunRow{ID: flowRunID, OrganizationID: orgID, Status: "running"},
		step: &FlowRunStepRow{
			ID:        stepID,
			FlowRunID: flowRunID,
			StepKey:   "sdd-judge",
			Status:    "running",
			Inputs: map[string]any{
				"mode": "full",
				"suggested_saves": []any{
					map[string]any{
						"type":     "sabotage_record",
						"required": true,
						"hint":     "guardar 1 sabotage_record por test de sabotaje ejecutado",
					},
				},
			},
		},
	}
	// registry vacío: Lookup(sdd-judge) falla y el bloque de validación de shape
	// se saltea, aislando el contrato de required_saves.
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = repo

	// reporta la fase SIN el memory_ref sabotage_record requerido.
	res, err := svc.RecordPhaseResult(context.Background(), PhaseResultInput{
		FlowRunStepID: stepID,
		Output:        map[string]any{"sabotage_records": []any{}},
	})
	require.NoError(t, err)

	require.Len(t, res.MissingRequiredSaves, 1, "debe reportar el required save faltante")
	require.Equal(t, "sabotage_record", res.MissingRequiredSaves[0].Type)
	require.NotEqual(t, "completed", res.StepStatus, "el step NO debe cerrarse sin el required save")
	require.Equal(t, uuid.Nil, repo.completedID, "MarkStepCompleted no debe llamarse")
	// corazón de sab-003 / REQ-56: reintentable, NO failed.
	require.Equal(t, uuid.Nil, repo.failedID, "MarkStepFailed NUNCA debe invocarse por un required save faltante")
}
