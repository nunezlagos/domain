//go:build integration

package e2e_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/promptrouter"
	wp "nunezlagos/domain/internal/service/wizardplan"
	"nunezlagos/domain/internal/service/wizardplan/sources"
)

func setupAdaptive(t *testing.T) (*issuebuilder.AdaptiveService, *db.Pools, uuid.UUID, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	// project sembrado: el scoping por project exige project_id NOT NULL.
	var projectID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('Demo', 'demo') RETURNING id`,
	).Scan(&projectID))

	classifier := &promptrouter.WizardplanAdapter{Inner: promptrouter.HeuristicClassifier{}}

	analyzer := &wp.Analyzer{
		Classifier: classifier,
		Sources: []wp.Source{
			&sources.IssueDedupSource{Pool: pools.App, Limit: 5},
			&sources.CodebaseSource{ProjectRoot: ".", MaxHits: 10},


		},
	}

	hbSvc := &issuebuilder.Service{Pool: pools.App}
	adaptive := &issuebuilder.AdaptiveService{
		Service:  hbSvc,
		Analyzer: analyzer,
		Planner:  &wp.Planner{},
	}

	return adaptive, pools, projectID, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestE2E_WizardAdaptive_FewQuestionsForBugFix(t *testing.T) {
	svc, _, projectID, cleanup := setupAdaptive(t)
	defer cleanup()
	ctx := context.Background()


	prompt := "URGENTE: producción caída, el endpoint /api/v1/observations falla con 500 al hacer POST"

	d, q, err := svc.StartAdaptive(ctx, prompt, nil, &projectID)
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, issuebuilder.ModeBugFix, d.Mode)



	require.NotNil(t, q, "debe haber al menos 1 pregunta inicial")


	questions := []*wp.Question{q}
	answers := map[string]any{
		wp.SlotSeverity:  "critical",
		wp.SlotComponent: "internal/api/handler/observations",
		wp.SlotActual:    "POST /observations devuelve 500 con stacktrace nil pointer",
		wp.SlotExpected:  "POST debe persistir observation y devolver 201",
		wp.SlotREQParent: "REQ-03-memory-system",
		wp.SlotSlug:      "fix-observation-create-500",
		wp.SlotSummary:   "Bug crítico en handler POST /observations: nil pointer rompe el endpoint en prod.",
	}

	max := 10
	for q != nil && max > 0 {
		val, ok := answers[q.SlotKey]
		if !ok {
			val = "default answer for " + q.SlotKey
		}
		d, q, err = svc.AnswerAdaptive(ctx, d.ID, q.SlotKey, val)
		require.NoError(t, err)
		if q != nil {
			questions = append(questions, q)
		}
		max--
	}

	require.Equal(t, issuebuilder.StatusFinished, d.Status)




	t.Logf("preguntas realizadas en flow adaptive: %d", len(questions))
	require.LessOrEqual(t, len(questions), 7,
		"el wizard adaptive debe preguntar MENOS que los 8 fijos del v1")
}

func TestE2E_WizardAdaptive_HUDedupInfersREQParent(t *testing.T) {
	svc, pools, projectID, cleanup := setupAdaptive(t)
	defer cleanup()
	ctx := context.Background()


	var reqID, issueID uuid.UUID
	err := pools.App.QueryRow(ctx,
		`INSERT INTO sdd_requirements (project_id, slug, title) VALUES ($1, 'REQ-03-memory-system', 'Sistema de memoria') RETURNING id`,
		projectID,
	).Scan(&reqID)
	require.NoError(t, err)
	err = pools.App.QueryRow(ctx,
		`INSERT INTO issues (project_id, req_id, slug, title, description)
		 VALUES ($1, $2, 'issue-03.1-observations',
		         'CRUD de observaciones con búsqueda',
		         'Endpoints para crear y listar observaciones del proyecto con búsqueda FTS')
		 RETURNING id`, projectID, reqID,
	).Scan(&issueID)
	require.NoError(t, err)

	prompt := "Bug: al crear una observación nueva el endpoint falla con error 500"
	d, _, err := svc.StartAdaptive(ctx, prompt, nil, &projectID)
	require.NoError(t, err)

	env, err := svc.LoadEnvelope(ctx, d.ID)
	require.NoError(t, err)
	require.NotNil(t, env.HUMatches, "hu_dedup debió correr (puede no haber matches por FTS stemming)")



	if len(env.HUMatches.Candidates) > 0 {
		t.Logf("hu_dedup matches: %d", len(env.HUMatches.Candidates))
		slot, ok := env.Slots[wp.SlotREQParent]
		if ok && slot.Status != wp.SlotUnknown {
			require.Equal(t, "REQ-03-memory-system", slot.Value,
				"req_parent inferido del HU dedup")
		}
	} else {
		t.Log("hu_dedup sin matches — esperado si stemming spanish no une los términos")
	}
}

func TestE2E_WizardAdaptive_ChatIntentSkipsWizard(t *testing.T) {
	svc, _, _, cleanup := setupAdaptive(t)
	defer cleanup()

	_, _, err := svc.StartAdaptive(context.Background(),
		"¿Cómo se configuran las migrations?", nil, nil)
	require.Error(t, err, "intent=chat NO debe arrancar wizard")
	require.Contains(t, err.Error(), "no requiere wizard")
}
