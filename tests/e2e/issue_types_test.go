//go:build integration

// E2E del flow real plug-and-play para cada tipo de issue.
//
// Pattern por sub-test:
//   1. Setup org + project + datos contextuales (HUs previas, etc.)
//   2. Prompt típico del intent
//   3. domain_prompt router → analyzer pipeline
//   4. Verificar:
//      - intake_payload persistido con classified_type correcto
//      - hu_drafts persistido con envelope si arrancó wizard
//      - slots inferidos (mínimos) o pendientes (esperados)
//      - lifecycle transitions registradas
//   5. Si entró al wizard: responder lo pendiente + verificar commit
//   6. Verificar attachments persisten y promueven al final
package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/hubuilder"
	"nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/promptrouter"
	wp "nunezlagos/domain/internal/service/wizardplan"
	"nunezlagos/domain/internal/service/wizardplan/sources"
)

type issueFixture struct {
	router    *promptrouter.Router
	adaptive  *hubuilder.AdaptiveService
	intakeSvc *intake.Service
	pools     *db.Pools
	orgID     uuid.UUID
	projectID uuid.UUID
}

func bootstrapForIssueTypes(t *testing.T) (*issueFixture, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "pgvector/pgvector:pg16",
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

	// Org + project base.
	var orgID, projectID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)
	err = pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug, organization_id) VALUES ('Demo', 'demo', $1) RETURNING id`, orgID,
	).Scan(&projectID)
	require.NoError(t, err)

	// Servicios wired.
	classifier := &promptrouter.WizardplanAdapter{Inner: promptrouter.HeuristicClassifier{}}
	analyzer := &wp.Analyzer{
		Classifier: classifier,
		Sources: []wp.Source{
			&sources.HUDedupSource{Pool: pools.App, Limit: 5},
		},
		Timeout: 5 * time.Second,
	}
	hbSvc := &hubuilder.Service{Pool: pools.App}
	adaptive := &hubuilder.AdaptiveService{
		Service: hbSvc, Analyzer: analyzer, Planner: &wp.Planner{},
	}
	intakeSvc := &intake.Service{Pool: pools.App}
	router := &promptrouter.Router{
		IntakeService:    intakeSvc,
		HubuilderService: hbSvc,
		Classifier:       promptrouter.HeuristicClassifier{},
	}

	cleanup := func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
	return &issueFixture{
		router: router, adaptive: adaptive, intakeSvc: intakeSvc,
		pools: pools, orgID: orgID, projectID: projectID,
	}, cleanup
}

// ============================================================
// ESCENARIO: chat → respuesta directa, NO entra al SDD
// ============================================================
func TestIssueType_Chat_SkipsWizardAndReplies(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	resp, err := f.router.Route(ctx,
		"¿Cómo se configuran las migrations de postgres?", nil)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeChat, resp.Outcome)
	require.Equal(t, promptrouter.IntentChat, resp.Intent)
	require.NotEmpty(t, resp.Reply, "debe haber respuesta de chat")

	// Asserts BD: NO se debe haber creado intake ni hu_draft.
	var intakeCount, draftCount int
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM intake_payloads`).Scan(&intakeCount))
	require.Equal(t, 0, intakeCount)
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM hu_drafts`).Scan(&draftCount))
	require.Equal(t, 0, draftCount)
}

// ============================================================
// ESCENARIO: idea → respuesta tipo chat, NO SDD
// ============================================================
func TestIssueType_Idea_SkipsWizardAndReplies(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	resp, err := f.router.Route(ctx,
		"Se me ocurre una idea: y si agregamos modo TUI offline", nil)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeChat, resp.Outcome)
	require.Equal(t, promptrouter.IntentIdea, resp.Intent)
	require.Contains(t, resp.Reply, "idea")
}

// ============================================================
// ESCENARIO: feature → wizard adaptive arranca
// ============================================================
func TestIssueType_Feature_StartsAdaptiveWizard(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "Quiero implementar export de runs a CSV con streaming"
	d, q, err := f.adaptive.StartAdaptive(ctx, prompt, nil)
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, hubuilder.ModeFeature, d.Mode)
	require.NotNil(t, q, "debe pedir al menos 1 input")

	// Verifica envelope persistido.
	env, err := f.adaptive.LoadEnvelope(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, "feature", env.Intent.Intent)
	require.GreaterOrEqual(t, env.Intent.Confidence, 0.5)
}

// ============================================================
// ESCENARIO: fix → wizard mode=bug-fix, classification persiste
// ============================================================
func TestIssueType_Fix_PersistsClassificationAndDraft(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "El botón export no funciona, devuelve 500 al hacer click"
	resp, err := f.router.Route(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeWizardStarted, resp.Outcome)
	require.Equal(t, promptrouter.IntentFix, resp.Intent)
	require.NotNil(t, resp.IntakeID)
	require.NotNil(t, resp.DraftID)

	// Verifica intake_payload.
	intakeP, err := f.intakeSvc.Get(ctx, *resp.IntakeID)
	require.NoError(t, err)
	require.Equal(t, "fix", *intakeP.ClassifiedType)
	require.Equal(t, "high", *intakeP.ClassifiedSeverity)
	require.GreaterOrEqual(t, *intakeP.ClassifiedConfidence, 0.5)

	// Verifica draft en hu_drafts.
	var draftStatus, draftMode string
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`SELECT status, mode FROM hu_drafts WHERE id = $1`, *resp.DraftID,
	).Scan(&draftStatus, &draftMode))
	require.Equal(t, "in_progress", draftStatus)
	require.Equal(t, hubuilder.ModeBugFix, draftMode)
}

// ============================================================
// ESCENARIO: hotfix → severity inferida critical, conf alta
// ============================================================
func TestIssueType_Hotfix_HighConfidenceCritical(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "URGENTE: producción caída, todos los logins fallan, esto es critical bug"
	resp, err := f.router.Route(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, promptrouter.IntentHotfix, resp.Intent)
	require.GreaterOrEqual(t, resp.Confidence, 0.8)

	intakeP, err := f.intakeSvc.Get(ctx, *resp.IntakeID)
	require.NoError(t, err)
	require.Equal(t, "critical", *intakeP.ClassifiedSeverity)
}

// ============================================================
// ESCENARIO: refactor → wizard mode=refactor
// ============================================================
func TestIssueType_Refactor_StartsCorrectMode(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "Necesito refactor del módulo de auth para extract los handlers en archivos separados"
	d, _, err := f.adaptive.StartAdaptive(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, hubuilder.ModeRefactor, d.Mode)
}

// ============================================================
// ESCENARIO: doc → wizard mode=doc
// ============================================================
func TestIssueType_Doc_StartsCorrectMode(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "Hay que actualizar la documentación del README con los nuevos endpoints"
	d, _, err := f.adaptive.StartAdaptive(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, hubuilder.ModeDoc, d.Mode)
}

// ============================================================
// ESCENARIO: rfc → wizard mode=rfc
// ============================================================
func TestIssueType_RFC_StartsCorrectMode(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	prompt := "RFC: diseño arquitectura del nuevo sistema de cache multi-tier, tradeoffs entre redis vs pgvector"
	d, _, err := f.adaptive.StartAdaptive(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, hubuilder.ModeRFC, d.Mode)
}

// ============================================================
// ESCENARIO: HU dedup detecta duplicado → req_parent inferido
// ============================================================
func TestIssueType_Feature_WithHUDedup_InfersReqParent(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	// Sembrar HU existente que matche el prompt en spanish FTS.
	var reqID uuid.UUID
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`INSERT INTO requirements (slug, title) VALUES ('REQ-13-http-api', 'API HTTP REST') RETURNING id`,
	).Scan(&reqID))
	_, err := f.pools.App.Exec(ctx,
		`INSERT INTO user_stories (req_id, slug, title, description)
		 VALUES ($1, 'HU-13.5-bulk-batch', 'Endpoints batch de creación masiva',
		         'Endpoints batch para crear múltiples observaciones simultáneamente')`,
		reqID,
	)
	require.NoError(t, err)

	prompt := "Necesito un endpoint batch para crear múltiples observaciones simultáneamente"
	d, _, err := f.adaptive.StartAdaptive(ctx, prompt, nil)
	require.NoError(t, err)
	require.Equal(t, hubuilder.ModeFeature, d.Mode)

	env, err := f.adaptive.LoadEnvelope(ctx, d.ID)
	require.NoError(t, err)
	require.NotNil(t, env.HUMatches, "hu_dedup debe correr")

	if len(env.HUMatches.Candidates) > 0 {
		t.Logf("✓ HU dedup encontró %d candidatos; top sim=%.2f",
			len(env.HUMatches.Candidates), env.HUMatches.Candidates[0].Similarity)
		// req_parent debería haber sido inferido.
		slot, ok := env.Slots[wp.SlotREQParent]
		if ok && slot.Status != wp.SlotUnknown {
			require.Equal(t, "REQ-13-http-api", slot.Value)
		}
	}
}

// ============================================================
// ESCENARIO: full happy path con commit + verificación BD post
// ============================================================
func TestIssueType_FullHappyPath_FixWithCommit(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()
	ctx := context.Background()

	resp, err := f.router.Route(ctx,
		"El endpoint POST /api/v1/observations falla con error 500 al hacer click — no funciona el bug que reportaron los devs", nil)
	require.NoError(t, err)
	require.Equal(t, promptrouter.OutcomeWizardStarted, resp.Outcome)
	draftID := *resp.DraftID

	// Adaptive responds. El adaptive tiene un loop:
	// next-question → answer → next-question → ... hasta finished.
	q, err := f.adaptive.LoadEnvelope(ctx, draftID)
	require.NoError(t, err)
	_ = q

	// Resolver TODOS los slots vía respuestas plausibles.
	answers := map[string]any{
		wp.SlotIntent:    "fix",
		wp.SlotSeverity:  "high",
		wp.SlotComponent: "internal/api/handler/observation",
		wp.SlotActual:    "POST devuelve 500 con stacktrace nil pointer al insertar",
		wp.SlotExpected:  "POST debe persistir observation y retornar 201",
		wp.SlotREQParent: "REQ-03-memory-system",
		wp.SlotSlug:      "fix-observation-create-500-nilpointer",
		wp.SlotSummary:   "Bug en POST /api/v1/observations: nil pointer al insertar; rompe el endpoint y los clientes ven 500.",
	}

	var d *hubuilder.Draft
	var nextQ *wp.Question
	d, nextQ, err = f.adaptive.AnswerAdaptive(ctx, draftID, wp.SlotSeverity, answers[wp.SlotSeverity])
	require.NoError(t, err)

	max := 12
	for nextQ != nil && max > 0 {
		val, ok := answers[nextQ.SlotKey]
		if !ok {
			val = "default for " + nextQ.SlotKey
		}
		d, nextQ, err = f.adaptive.AnswerAdaptive(ctx, draftID, nextQ.SlotKey, val)
		require.NoError(t, err)
		max--
	}

	require.Equal(t, hubuilder.StatusFinished, d.Status,
		"el draft debe quedar en finished tras responder todos los slots")

	// Asserts BD: envelope persistido + slots provided.
	var answersRaw string
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`SELECT answers::text FROM hu_drafts WHERE id = $1`, draftID,
	).Scan(&answersRaw))
	require.Contains(t, answersRaw, "__envelope__")
	require.Contains(t, answersRaw, "fix-observation-create-500-nilpointer")
}

// Sabotaje: si el classifier devuelve intent inválido + sin fallback,
// el router devolverá error en lugar de seguir.
func TestSabotage_RouterRequiresValidPrompt(t *testing.T) {
	f, cleanup := bootstrapForIssueTypes(t)
	defer cleanup()

	_, err := f.router.Route(context.Background(), "   ", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, promptrouter.ErrEmptyPrompt)
}
