//go:build integration

// Auditoría completa del schema + datos sembrados post-RunAll del seeder.
// Verifica que todas las tablas críticas existen + columnas + counts.
package e2e_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

func TestSchemaAudit_AllExpectedTablesExist(t *testing.T) {
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
	defer pgC.Terminate(ctx)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	defer pools.Close()

	// Lista canónica de tablas críticas para el flow plug-and-play.
	expectedTables := []string{
		// Core
		"organizations", "users", "projects", "auth_api_keys",
		// Memory (REQ-42.3: sessions dropeada — feature legacy)
		"observations", "prompts", "knowledge_docs", "knowledge_chunks",
		// SDD
		"sdd_requirements", "issues", "issue_gherkin_scenarios",
		"sdd_proposals", "sdd_designs", "issue_tasks", "tdd_verification_results", "tdd_sabotage_records",
		"issue_code_references", "file_attachments",
		// Wizard + Intake + ExtSync
		"issue_drafts", "issue_draft_steps_log",
		"issue_intake_payloads", "intake_attachments",
		"external_providers", "external_sync_state", "external_sync_events",
		// Workflow override
		"imported_workflow_files",
		// Agents + Skills + Flows
		"agents", "agent_runs", "agent_run_logs", "agent_templates",
		"skills", "skill_versions",
		"flows", "flow_runs", "flow_run_steps", "flow_versions",
		"flow_signals", "flow_run_step_snapshots",
		// Crons + Webhooks
		"crons", "webhooks", "outbound_webhook_subscriptions", "outbound_webhook_deliveries",
		"event_log",
		// Audit + Usage (REQ-42.2: cost_logs dropeada con el dominio billing/costos)
		"audit_log", "activity_log", "usage_counters", "usage_alerts",
		"notification_deliveries",
		// Auth + RBAC (auth_rate_limits es in-memory issue-02.5;
		// role_resource_limits es ALTER ROLE migration 000029, no tabla)
		"auth_otp_codes", "custom_roles", "auth_invitations", "auth_secrets",
		// Policies + Templates (REQ-42.2: plans dropeada con billing/costos)
		"platform_policies", "platform_policy_versions",
		"project_templates", "project_links", "project_merges",
		// Domain misc (REQ-42.3: runtime_configs/model_registry/idempotency_keys dropeadas)
		"seed_versions", "mcp_servers", "mcp_server_tools",
		"selfhosted_runners", "selfhosted_tasks",
		"domain_query_stats_history",
		"llm_semantic_cache",
	}

	missing := []string{}
	for _, table := range expectedTables {
		var exists bool
		err := pools.App.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table,
		).Scan(&exists)
		require.NoError(t, err)
		if !exists {
			missing = append(missing, table)
		}
	}

	if len(missing) > 0 {
		t.Errorf("tablas faltantes en schema:\n  %v", missing)
	} else {
		t.Logf("✓ %d tablas críticas presentes", len(expectedTables))
	}
}

func TestSchemaAudit_SeedersPopulateCatalogs(t *testing.T) {
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
	defer pgC.Terminate(ctx)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	defer pools.Close()

	// REQ-42.2: PlansSeeder se eliminó junto con el dominio billing/costos.
	// REQ-42.3: ModelRegistrySeeder se eliminó (model_registry dropeada; el
	// pricing vive en código, internal/llm/registry).
	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.GreaterOrEqual(t, results["platform_policies"].Created, 5)

	// Verifica counts en BD efectivos.
	checks := []struct {
		table   string
		minRows int
	}{
		{"platform_policies", 8},
	}
	for _, c := range checks {
		var n int
		require.NoError(t, pools.App.QueryRow(ctx,
			"SELECT COUNT(*) FROM "+c.table).Scan(&n))
		require.GreaterOrEqual(t, n, c.minRows,
			"%s debe tener al menos %d filas tras seed, tiene %d", c.table, c.minRows, n)
	}
}

func TestSchemaAudit_CriticalForeignKeysPresent(t *testing.T) {
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
	defer pgC.Terminate(ctx)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	defer pools.Close()

	// FKs críticas del flow plug-and-play.
	criticalFKs := []struct {
		table       string
		column      string
		referencedT string
	}{
		{"issues", "req_id", "sdd_requirements"},
		{"sdd_proposals", "issue_id", "issues"},
		{"sdd_designs", "issue_id", "issues"},
		{"issue_tasks", "issue_id", "issues"},
		{"file_attachments", "entity_id", ""}, // polymorphic, no FK
		{"issue_drafts", "organization_id", "organizations"},
		{"issue_intake_payloads", "committed_issue_id", "issues"},
		{"external_sync_state", "provider_id", "external_providers"},
		{"projects", "organization_id", "organizations"},
		{"observations", "project_id", "projects"},
		{"agent_runs", "organization_id", "organizations"},
		{"flow_runs", "flow_id", "flows"},
		{"flow_run_steps", "flow_run_id", "flow_runs"},
	}

	for _, fk := range criticalFKs {
		if fk.referencedT == "" {
			continue
		}
		var found bool
		err := pools.App.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_constraint con
				JOIN pg_class src  ON src.oid = con.conrelid
				JOIN pg_class dst  ON dst.oid = con.confrelid
				JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY(con.conkey)
				WHERE con.contype = 'f'
				  AND src.relname = $1
				  AND a.attname  = $2
				  AND dst.relname = $3
			)`, fk.table, fk.column, fk.referencedT,
		).Scan(&found)
		require.NoError(t, err)
		require.Truef(t, found, "FK %s.%s → %s missing", fk.table, fk.column, fk.referencedT)
	}
}
