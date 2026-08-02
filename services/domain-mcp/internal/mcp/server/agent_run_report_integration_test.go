//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-147: agent_runs solo registraba lo que ejecuta el server
// (internal/runner/agent). Cuando el agente corre en el CLIENTE —que es a donde
// apunta el diseño de DOMAINSERV-133— la ejecución no quedaba en ninguna parte:
// la telemetría de costo se vaciaba sola sin que nada avisara.
//
// domain_agent_run_report es el viaje de vuelta, mismo patrón que
// domain_orchestrate_phase_result y domain_mem_used: el cliente ejecuta y
// reporta, el server valida y persiste.

func TestAgentRunReport_RunDelCliente_QuedaRegistradoConTokensCostoModeloYProject(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	agentID := crearAgente(t, ctx, f, "reporter-local")

	out := callTool(t, f.srv, "domain_agent_run_report", map[string]any{
		"project_slug":  f.projectSlug,
		"agent_slug":    "reporter-local",
		"status":        "completed",
		"model":         "claude-opus-5",
		"tokens_input":  float64(1200),
		"tokens_output": float64(340),
		"cost_usd":      0.0182,
		"output":        "refactor aplicado",
	})
	var resp struct {
		RunID string `json:"run_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	runID, err := uuid.Parse(resp.RunID)
	require.NoError(t, err)

	var (
		gotAgent, gotProject  uuid.UUID
		status, source, model string
		tokensIn, tokensOut   int64
		cost                  float64
		finishedAt            *time.Time
	)
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT agent_id, project_id, status, source, model,
		        tokens_input, tokens_output, cost_usd::float8, finished_at
		 FROM agent_runs WHERE id = $1`, runID,
	).Scan(&gotAgent, &gotProject, &status, &source, &model,
		&tokensIn, &tokensOut, &cost, &finishedAt))

	require.Equal(t, agentID, gotAgent)
	require.Equal(t, f.projectID, gotProject,
		"el run reportado no quedó scopeado al project: la telemetría no se puede atribuir")
	require.Equal(t, "completed", status)
	require.Equal(t, "client", source,
		"sin source=client no hay forma de distinguir lo que ejecutó el cliente de lo que ejecutó el server")
	require.Equal(t, "claude-opus-5", model, "sin modelo el registro no sirve para costo")
	require.EqualValues(t, 1200, tokensIn)
	require.EqualValues(t, 340, tokensOut)
	require.InDelta(t, 0.0182, cost, 1e-9)
	require.NotNil(t, finishedAt,
		"un run sin finished_at queda colgado: es justamente lo que el ticket prohíbe")

	// El cron orphan-runs-audit cuenta como bypass del enforcement del orquestador
	// todo run con flow_run_id NULL cuyo metadata.standalone no sea 'true'. Un run
	// del cliente es standalone DECLARADO, no uno que se escapó: sin esta marca la
	// métrica domain_agent_runs_orphan_total crece con cada reporte legítimo.
	var standalone *string
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT metadata->>'standalone' FROM agent_runs WHERE id = $1`, runID).Scan(&standalone))
	require.NotNil(t, standalone, "sin metadata.standalone el cron orphan-runs-audit cuenta el reporte como bypass")
	require.Equal(t, "true", *standalone)
}

// El reporte es TERMINAL por construcción: no existe forma de abrir un run desde
// el cliente, así que no puede quedar uno colgado en 'running' esperando un
// reporte que nunca llega. Los demás casos son el scope explícito por project +
// los campos sin los que el registro no sirve (ticket, punto 2).
func TestAgentRunReport_ReporteInvalido_RechazaSinPersistir(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	crearAgente(t, ctx, f, "reporter-local")
	base := func() map[string]any {
		return map[string]any{
			"project_slug":  f.projectSlug,
			"agent_slug":    "reporter-local",
			"status":        "completed",
			"model":         "claude-opus-5",
			"tokens_input":  float64(10),
			"tokens_output": float64(20),
		}
	}
	casos := []struct {
		nombre string
		mutar  func(map[string]any)
	}{
		{"status no terminal quedaría colgado", func(a map[string]any) { a["status"] = "running" }},
		{"status desconocido", func(a map[string]any) { a["status"] = "lo-que-sea" }},
		{"project inexistente no se auto-crea", func(a map[string]any) { a["project_slug"] = "no-existe-este-project" }},
		{"agent inexistente", func(a map[string]any) { a["agent_slug"] = "agente-fantasma" }},
		{"sin modelo no sirve para costo", func(a map[string]any) { a["model"] = "" }},
		{"tokens negativos", func(a map[string]any) { a["tokens_input"] = float64(-1) }},
		{"costo negativo", func(a map[string]any) { a["cost_usd"] = -0.5 }},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			args := base()
			c.mutar(args)
			_, isErr := callToolRaw(t, f.srv, "domain_agent_run_report", args)
			require.True(t, isErr, "el reporte inválido fue aceptado: %v", args)
		})
	}

	var filas int64
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_runs`).Scan(&filas))
	require.Zero(t, filas, "un reporte rechazado escribió telemetría igual")
}

func crearAgente(t *testing.T, ctx context.Context, f *mcpFixture, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, provider, model)
		 VALUES ($1, $1, 'anthropic', 'claude-opus-5') RETURNING id`, slug,
	).Scan(&id))
	return id
}
