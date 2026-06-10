//go:build integration

package promptrouter_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	orgsvc "nunezlagos/domain/internal/service/org"
	"nunezlagos/domain/internal/service/promptrouter"
)

func setupRouterDB(t *testing.T) (*db.Pools, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	return pools, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func buildFullPhasesRegistry() *phases.Registry {
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDExploreHandler())
	reg.MustRegister(phases.NewSDDSpecHandler())
	reg.MustRegister(phases.NewSDDProposeHandler())
	reg.MustRegister(phases.NewSDDDesignHandler())
	reg.MustRegister(phases.NewSDDTasksHandler())
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	reg.MustRegister(phases.NewSDDJudgeHandler())
	reg.MustRegister(phases.NewSDDArchiveHandler())
	reg.MustRegister(phases.NewSDDOnboardHandler())
	return reg
}

// Cuando Router tiene Orchestrator inyectado, un prompt feat/fix/refactor
// arranca el orquestador SDD en lugar del wizard legacy.
func TestRouter_WithOrchestrator_FeaturePromptStartsFullOrchestrator(t *testing.T) {
	pools, cleanup := setupRouterDB(t)
	defer cleanup()
	ctx := context.Background()

	rec := &audit.PGRecorder{Pool: pools.Auth}
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, err := orgS.Create(ctx, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)

	intakeSvc := &intake.Service{Pool: pools.App, Audit: rec}
	hubuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: rec, DraftTTLHrs: 24}
	orchSvc := orchestrator.New(pools.App, rec, buildFullPhasesRegistry(), "dev")
	router := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: hubuilderSvc,
		Classifier:          promptrouter.HeuristicClassifier{},
		Orchestrator:        orchSvc,
	}

	userID := owner.UserID
	res, err := router.Route(ctx, "agregar nueva feature de export CSV", &userID, &org.ID)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeOrchestratorStarted, res.Outcome)
	require.Equal(t, promptrouter.IntentFeature, res.Intent)
	require.NotNil(t, res.FlowRunID)
	require.NotNil(t, res.OrchestratorRunID)
	require.NotEmpty(t, res.SnapshotPrompt)
	require.Equal(t, "full", res.Mode, "feature → Full mode")
	require.NotNil(t, res.IntakeID, "intake_payload persistido para audit")
}

// Intent fix dispara Express (fast path 2 fases).
func TestRouter_WithOrchestrator_FixPromptStartsExpress(t *testing.T) {
	pools, cleanup := setupRouterDB(t)
	defer cleanup()
	ctx := context.Background()

	rec := &audit.PGRecorder{Pool: pools.Auth}
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, err := orgS.Create(ctx, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)

	intakeSvc := &intake.Service{Pool: pools.App, Audit: rec}
	hubuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: rec, DraftTTLHrs: 24}
	orchSvc := orchestrator.New(pools.App, rec, buildFullPhasesRegistry(), "dev")
	router := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: hubuilderSvc,
		Classifier:          promptrouter.HeuristicClassifier{},
		Orchestrator:        orchSvc,
	}

	userID := owner.UserID
	res, err := router.Route(ctx, "bug: el botón export no funciona", &userID, &org.ID)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeOrchestratorStarted, res.Outcome)
	require.Equal(t, promptrouter.IntentFix, res.Intent)
	require.Equal(t, "express", res.Mode, "fix → Express mode")
}

// Intent chat NO arranca orquestador (incluso si está configurado).
func TestRouter_WithOrchestrator_ChatBypassesOrchestrator(t *testing.T) {
	pools, cleanup := setupRouterDB(t)
	defer cleanup()
	ctx := context.Background()

	rec := &audit.PGRecorder{Pool: pools.Auth}
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, err := orgS.Create(ctx, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)

	intakeSvc := &intake.Service{Pool: pools.App, Audit: rec}
	hubuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: rec, DraftTTLHrs: 24}
	orchSvc := orchestrator.New(pools.App, rec, buildFullPhasesRegistry(), "dev")
	router := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: hubuilderSvc,
		Classifier:          promptrouter.HeuristicClassifier{},
		Orchestrator:        orchSvc,
	}

	userID := owner.UserID
	res, err := router.Route(ctx, "cómo se configura X?", &userID, &org.ID)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeChat, res.Outcome)
	require.Nil(t, res.FlowRunID, "chat NO debe arrancar orquestador")
}

// Sin orgID y con Orchestrator configurado → error explícito.
func TestRouter_WithOrchestrator_RequiresOrgID(t *testing.T) {
	pools, cleanup := setupRouterDB(t)
	defer cleanup()
	ctx := context.Background()

	rec := &audit.PGRecorder{Pool: pools.Auth}
	intakeSvc := &intake.Service{Pool: pools.App, Audit: rec}
	hubuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: rec, DraftTTLHrs: 24}
	orchSvc := orchestrator.New(pools.App, rec, buildFullPhasesRegistry(), "dev")
	router := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: hubuilderSvc,
		Classifier:          promptrouter.HeuristicClassifier{},
		Orchestrator:        orchSvc,
	}

	userID := uuid.New()
	_, err := router.Route(ctx, "agregar feature", &userID, nil)
	require.ErrorIs(t, err, promptrouter.ErrOrgIDRequiredForOrchestrator)
}
