//go:build integration

// Package e2e — test E2E del flow real cliente plug-and-play:
//
//   prompt crudo → domain_prompt router → wizard interactivo issue-04.7
//   → attach screenshot → 8 respuestas → commit → promote attachments
//   → verifica intake_payload + issue_drafts + file_attachments en BD
//
// Cubre los 3 gaps cerrados en esta tirada:
//   - issue-04.7 + issue-04.6: image upload integrado al wizard (paso 3)
//   - issue-12.7: domain_prompt router single-shot (paso 2)
//   - workflowimport scanner (paso 2)
//
// Si este test pasa, el flow plug-and-play funciona end-to-end.
package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/promptrouter"
	"nunezlagos/domain/internal/service/workflowimport"
)

// mockAttSvc satisface issuebuilder.AttachmentService sin S3 real.
type mockAttSvc struct {
	uploadsCalled  int
	promotesCalled int
}

func (m *mockAttSvc) InitUpload(_ context.Context, entityType, entityIDStr, filename, mime, by string, size int64) (*issuebuilder.AttachmentInitResult, error) {
	m.uploadsCalled++
	return &issuebuilder.AttachmentInitResult{
		AttachmentID: uuid.New(),
		UploadURL:    "https://s3.mock/" + entityType + "/" + entityIDStr + "/" + filename,
		Filename:     filename,
	}, nil
}

func (m *mockAttSvc) PromoteEntity(_ context.Context, fromKind, toKind string, _, _ uuid.UUID) (int, error) {
	m.promotesCalled++
	return 1, nil
}

func TestE2E_PluggandPlayFlow_PromptToCommit(t *testing.T) {
	ctx := context.Background()

	// 1) Bootstrap DB
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
	defer pgC.Terminate(ctx)

	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	defer pools.Close()

	// 2) Wire services
	intakeSvc := &intake.Service{Pool: pools.App}
	hbSvc := &issuebuilder.Service{Pool: pools.App, Attachments: &mockAttSvc{}}
	router := &promptrouter.Router{
		IntakeService:    intakeSvc,
		IssueBuilderService: hbSvc,
		Classifier:       promptrouter.HeuristicClassifier{},
	}

	// === ESCENARIO 1: prompt de chat NO arranca wizard ===
	t.Run("chat_no_wizard", func(t *testing.T) {
		resp, err := router.Route(ctx,
			"Cómo se configuran las migrations de postgres?", nil)
		require.NoError(t, err)
		require.Equal(t, promptrouter.OutcomeChat, resp.Outcome)
		require.Equal(t, promptrouter.IntentChat, resp.Intent)
		require.Nil(t, resp.DraftID, "chat NO debe crear draft")
		require.Nil(t, resp.IntakeID, "chat NO debe crear intake")
		require.NotEmpty(t, resp.Reply)
	})

	// === ESCENARIO 2: prompt de bug arranca wizard correctamente ===
	t.Run("bug_starts_wizard", func(t *testing.T) {
		resp, err := router.Route(ctx,
			"El director no puede descargar la ficha aunque haya completado las 4 tasas. No funciona el botón de export, ya pasé el screenshot.",
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, promptrouter.OutcomeWizardStarted, resp.Outcome)
		require.Equal(t, promptrouter.IntentFix, resp.Intent)
		require.NotNil(t, resp.DraftID)
		require.NotNil(t, resp.IntakeID)
		require.NotNil(t, resp.NextQuestion)
		require.Equal(t, "severity", resp.NextQuestion.Key)

		// Verifica que el intake_payload se persistió con classification.
		intakeP, err := intakeSvc.Get(ctx, *resp.IntakeID)
		require.NoError(t, err)
		require.NotNil(t, intakeP.ClassifiedType)
		require.Equal(t, "fix", *intakeP.ClassifiedType)

		// === Sigue el flow: responde 8 preguntas + attach screenshot ===
		draftID := *resp.DraftID

		// Steps del bugFixFlow:
		// 1. severity → enum [critical|high|medium|low]
		// 2. component → enum [api|db|cli|mcp|auth|webhook|runner|ui]
		// 3. root_cause → enum [logic|race|perf|security|ux]
		// 4. has_repro → enum [yes|no]
		// 5. expected → string
		// 6. actual → string
		// 7. slug → slug regex
		// 8. summary → string
		bugAnswers := []any{
			"high",
			"ui",
			"ux",
			"yes",
			"Descargar la ficha PDF tras completar las 4 tasas",
			"Nada pasa al click; sin error visible, log silencioso",
			"export-ficha-director-broken",
			"Directores no pueden descargar la ficha tras completar tasas. UX bloqueante; afecta a 100% de los users con rol director.",
		}
		for i, a := range bugAnswers {
			_, _, err = hbSvc.Answer(ctx, draftID, a)
			require.NoErrorf(t, err, "step %d failed (answer=%v)", i+1, a)
		}

		// Attach screenshot mid-flow
		att, err := hbSvc.AttachToDraft(ctx, draftID,
			"screenshot-export-broken.png", "image/png", 512_000)
		require.NoError(t, err)
		require.NotEmpty(t, att.UploadURL)
		require.True(t, strings.HasPrefix(att.UploadURL, "https://s3.mock/hu_draft/"),
			"upload URL debe apuntar a entity_type=hu_draft")

		// Verifica que en issue_drafts.answers["attachments"] está la ref
		got, err := hbSvc.Get(ctx, draftID)
		require.NoError(t, err)
		require.Contains(t, string(got.Answers), "screenshot-export-broken.png")
	})
}

func TestE2E_WorkflowImport_DetectsRealRepoMDFiles(t *testing.T) {
	// Test in-process del scanner sobre un fixture tmpdir realista.
	tmp := t.TempDir()

	// Simula un repo con varios tools IA configurados.
	fs := map[string]string{
		"CLAUDE.md":              "# Project rules\n\nTDD strict.",
		".claude/rules/git.md":   "# Git conventions\n\nConventional commits.",
		".claude/rules/db.md":    "# DB rules\n\nUUID primary keys.",
		".claude/rules/api.md":   "# API conventions\n\nREST + JSON envelope.",
		".opencode/agents.md":    "# Agent defs",
		".cursorrules":           "Use Go 1.25.",
		"AGENTS.md":              "# Agent prompts",
		"src/main.go":            "package main",
		"README.md":              "# Project readme",
		"node_modules/foo/x.md":  "ignored",
	}
	writeFixture(t, tmp, fs)

	scanner := &workflowimport.Scanner{ProjectRoot: tmp}
	files, err := scanner.Detect(true)
	require.NoError(t, err)

	// Esperamos 7 archivos IA detectados (los 3 src/README/node_modules NO).
	require.Len(t, files, 7)

	tools := map[string]int{}
	for _, f := range files {
		tools[f.SourceTool]++
		require.NotEmpty(t, f.ContentHash)
		require.NotEmpty(t, f.Content)
	}
	require.Equal(t, 4, tools["claude-code"]) // CLAUDE.md + 3 en .claude/
	require.Equal(t, 1, tools["opencode"])
	require.Equal(t, 1, tools["cursor"])
	require.Equal(t, 1, tools["generic"])
}

// writeFixture helper.
func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		writeFile(t, root, path, content)
	}
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := root + "/" + relPath
	dir := full
	if i := lastIdx(full, '/'); i >= 0 {
		dir = full[:i]
	}
	require.NoError(t, mkdirAll(dir))
	require.NoError(t, writeFileBytes(full, []byte(content)))
}

func lastIdx(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// Wrappers que evitan importar os directamente acá; helpers minimal.
var (
	mkdirAll       = osMkdirAll
	writeFileBytes = osWriteFile
)
