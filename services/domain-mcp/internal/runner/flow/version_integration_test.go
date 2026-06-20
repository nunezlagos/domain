//go:build integration

package flowrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/service/flow"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

func createSkillAndFlow(t *testing.T, f *fix, slug string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if _, err := f.skills.GetBySlug(ctx, f.orgID, "ver-skill"); err != nil {
		_, err := f.skills.Create(ctx, skillsvc.CreateInput{
			OrganizationID: f.orgID, Slug: "ver-skill", Name: "Ver Skill",
			SkillType: skillsvc.TypePrompt, Content: "ok",
			InputSchema: map[string]any{"type": "object"},
			ActorID:     f.userID,
		})
		require.NoError(t, err)
	}
	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: slug, Name: slug,
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun,
				Config: map[string]any{"skill_slug": "ver-skill", "args": map[string]any{}}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)
	return fl.ID
}

// TestFlow_Run_PinsVersion — fv-008: el run guarda flow_version_id y el pin
// es idempotente por hash (dos runs del mismo spec → misma versión).
func TestFlow_Run_PinsVersion(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	flowID := createSkillAndFlow(t, f, "pin-flow")

	res1, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: flowID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res1.Status)

	var v1 *uuid.UUID
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT flow_version_id FROM flow_runs WHERE id = $1`, res1.RunID).Scan(&v1))
	require.NotNil(t, v1, "el run debe quedar pinneado a una versión")

	res2, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: flowID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	var v2 *uuid.UUID
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT flow_version_id FROM flow_runs WHERE id = $1`, res2.RunID).Scan(&v2))
	require.NotNil(t, v2)
	require.Equal(t, *v1, *v2, "mismo spec → misma versión (idempotente por hash)")

	var count int
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM flow_versions WHERE flow_id = $1`, flowID).Scan(&count))
	require.Equal(t, 1, count)
}

// TestFlow_Run_SpecificVersion_NotPublishedRejected — invocar una versión
// draft es rechazado (solo published es invokable).
func TestFlow_Run_SpecificVersion_NotPublishedRejected(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	flowID := createSkillAndFlow(t, f, "draft-flow")

	// auto-pin crea v1 (draft por default)
	_, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: flowID, TriggeredBy: &f.userID})
	require.NoError(t, err)

	_, err = f.runner.Run(ctx, flowrunner.RunInput{
		FlowID: flowID, TriggeredBy: &f.userID, FlowVersion: 1,
	})
	require.Error(t, err, "versión draft no debe ser invokable")
}

// TestVersioning_ArchiveDeprecated — fv-009: borra deprecated >retention sin
// runs; conserva las referenciadas por runs.
func TestVersioning_ArchiveDeprecated(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	flowID := createSkillAndFlow(t, f, "arch-flow")
	vs := &flow.VersioningStore{Pool: f.runner.Pool}

	// v1 referenciada por un run real
	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: flowID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	_ = res

	// v2 sin runs
	v2, err := vs.NewVersion(ctx, flowID, []byte(`{"version":1,"steps":[]}`), "hash-v2", "", nil)
	require.NoError(t, err)

	// Ambas deprecated hace 100 días
	_, err = f.runner.Pool.Exec(ctx, `
		UPDATE flow_versions SET status = 'deprecated',
		  deprecated_at = NOW() - INTERVAL '100 days', is_default = false
		WHERE flow_id = $1`, flowID)
	require.NoError(t, err)

	n, err := vs.ArchiveDeprecated(ctx, 90*24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "solo v2 (sin runs) debe archivarse")

	var remaining int
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM flow_versions WHERE flow_id = $1`, flowID).Scan(&remaining))
	require.Equal(t, 1, remaining, "v1 referenciada por run debe sobrevivir")

	_, err = vs.GetVersion(ctx, flowID, v2.Version)
	require.ErrorIs(t, err, flow.ErrFlowVersionNotFound)
}
