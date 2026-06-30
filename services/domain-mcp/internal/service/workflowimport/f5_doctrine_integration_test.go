//go:build integration




package workflowimport_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/workflowimport"
)

const agentsMDOriginal = `# Domain Test Repo

This is the original AGENTS.md that should be backed up to domain.
It contains project-specific instructions.
`

const claudeMDOriginal = `# Claude Project

Original CLAUDE.md content.
`

const doctrineStub = `# Project uses Domain — instructions live in DB.

This is a stub. The original content was backed up to Domain and can be
restored with: domain workflow restore AGENTS.md

For project context and rules, query domain via the MCP tools.
`

func setup(t *testing.T) (*pgxpool.Pool, *uuid.UUID, *uuid.UUID, func()) {
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
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	orgID, err := createOrg(ctx, pool)
	require.NoError(t, err)
	projectID, err := createProject(ctx, pool, orgID)
	require.NoError(t, err)

	return pool, &orgID, &projectID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func createOrg(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Test Org', 'org') RETURNING id
	`).Scan(&id)
	return id, err
}

func createProject(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO projects (organization_id, name, slug) VALUES ($1, 'P', 'p') RETURNING id
	`, orgID).Scan(&id)
	return id, err
}

func TestWorkflowImport_AGENTS_md_BackedUp(t *testing.T) {
	pool, orgID, projectID, cleanup := setup(t)
	defer cleanup()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMDOriginal), 0o644))

	svc := &workflowimport.Service{Pool: pool}
	report, err := svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    false,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, report.BackedUp, 1, "AGENTS.md debe respaldarse")
	require.Equal(t, 0, report.Replaced, "sin WriteStub no debe reemplazar")

	var original string
	var status string
	err = pool.QueryRow(context.Background(), `
		SELECT original_content, status FROM project_imported_workflow_files
		WHERE project_id = $1 AND rel_path = 'AGENTS.md'
	`, projectID).Scan(&original, &status)
	require.NoError(t, err)
	require.Equal(t, agentsMDOriginal, original, "contenido completo respaldado")
	require.Equal(t, "backed_up", status)

	onDisk, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, agentsMDOriginal, string(onDisk), "sin WriteStub el archivo en disco NO se toca")
}

func TestWorkflowImport_AGENTS_md_StubReplaces(t *testing.T) {
	pool, orgID, projectID, cleanup := setup(t)
	defer cleanup()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMDOriginal), 0o644))

	svc := &workflowimport.Service{Pool: pool}
	_, err := svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    true,
	})
	require.NoError(t, err)

	onDisk, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, doctrineStub, string(onDisk), "stub debe reemplazar al original")

	var status string
	err = pool.QueryRow(context.Background(), `
		SELECT status FROM project_imported_workflow_files
		WHERE project_id = $1 AND rel_path = 'AGENTS.md'
	`, projectID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "replaced", status)
}

func TestWorkflowImport_CLAUDE_md_AlsoHandled(t *testing.T) {
	pool, orgID, projectID, cleanup := setup(t)
	defer cleanup()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMDOriginal), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(claudeMDOriginal), 0o644))

	svc := &workflowimport.Service{Pool: pool}
	report, err := svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    true,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, report.BackedUp, 2, "ambos AGENTS.md y CLAUDE.md")

	var agentStatus, claudeStatus string
	pool.QueryRow(context.Background(),
		`SELECT status FROM project_imported_workflow_files WHERE project_id=$1 AND rel_path='AGENTS.md'`,
		projectID).Scan(&agentStatus)
	pool.QueryRow(context.Background(),
		`SELECT status FROM project_imported_workflow_files WHERE project_id=$1 AND rel_path='CLAUDE.md'`,
		projectID).Scan(&claudeStatus)
	require.Equal(t, "replaced", agentStatus)
	require.Equal(t, "replaced", claudeStatus)
}

func TestWorkflowImport_RestoreFromDB(t *testing.T) {
	pool, orgID, projectID, cleanup := setup(t)
	defer cleanup()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMDOriginal), 0o644))

	svc := &workflowimport.Service{Pool: pool}
	_, err := svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    true,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Restore(context.Background(), projectID, "AGENTS.md", dir))

	onDisk, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, agentsMDOriginal, string(onDisk), "restore debe reescribir el original")

	var status string
	pool.QueryRow(context.Background(), `
		SELECT status FROM project_imported_workflow_files
		WHERE project_id = $1 AND rel_path = 'AGENTS.md'
	`, projectID).Scan(&status)
	require.Equal(t, "restored", status)
}

func TestWorkflowImport_Idempotent_NoDuplicate(t *testing.T) {
	pool, orgID, projectID, cleanup := setup(t)
	defer cleanup()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMDOriginal), 0o644))

	svc := &workflowimport.Service{Pool: pool}
	_, err := svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    true,
	})
	require.NoError(t, err)

	_, err = svc.Import(context.Background(), workflowimport.ImportInput{
		OrgID:        orgID,
		ProjectID:    projectID,
		ProjectRoot:  dir,
		StubTemplate: doctrineStub,
		WriteStub:    true,
	})
	require.NoError(t, err)

	var count int
	pool.QueryRow(context.Background(), `
		SELECT count(*) FROM project_imported_workflow_files
		WHERE project_id = $1 AND rel_path = 'AGENTS.md'
	`, projectID).Scan(&count)
	require.Equal(t, 1, count, "segunda import del mismo path no debe duplicar")
}
