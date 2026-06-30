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


	expectedTables := []string{

		"organizations", "users", "projects", "auth_api_keys",

		"observations", "prompts", "knowledge_docs", "knowledge_chunks",

		"sdd_requirements", "issues", "issue_gherkin_scenarios",
		"sdd_proposals", "sdd_designs", "issue_tasks", "tdd_verification_results", "tdd_sabotage_records",
		"issue_code_references", "file_attachments",

		"issue_drafts", "issue_draft_steps_log",
		"issue_intake_payloads", "intake_attachments",
		"external_providers", "external_sync_state", "external_sync_events",

		"imported_workflow_files",

		"agents", "agent_runs", "agent_run_logs", "agent_templates",
		"skills", "skill_versions",
		"flows", "flow_runs", "flow_run_steps", "flow_versions",
		"flow_signals", "flow_run_step_snapshots",

		"crons", "webhooks", "outbound_webhook_subscriptions", "outbound_webhook_deliveries",
		"event_log",

		"audit_log", "activity_log", "usage_counters", "usage_alerts",
		"notification_deliveries",


		"custom_roles", "auth_invitations", "auth_secrets",

		"platform_policies", "platform_policy_versions",
		"project_templates", "project_links", "project_merges",

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




	reg := seeds.NewRegistry()
	reg.Register(&seeds.PlatformPoliciesSeeder{})
	results, err := reg.RunAll(ctx, pools.Auth, seeds.EnvDev)
	require.NoError(t, err)
	require.GreaterOrEqual(t, results["platform_policies"].Created, 5)


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
