// agent_run_report.go — DOMAINSERV-147.
//
// domain_agent_run_report: el cliente reporta una ejecución agéntica que corrió
// EN EL CLIENTE. Mismo patrón que domain_orchestrate_phase_result y
// domain_mem_used — el cliente ejecuta, el server valida y persiste.
//
// Se separa de domain_agent_run a propósito: esa EJECUTA. Mezclar "ejecutá esto"
// con "registrá que ejecuté esto" deja una tool cuyo efecto depende de qué
// argumentos vinieron, y el caso desatendido (cron, webhooks) tiene que seguir
// funcionando sin tocar nada.
//
// Solo acepta estados TERMINALES: el cliente no puede abrir un run, así que
// ninguno queda colgado en 'running' esperando un reporte que nunca llega.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

var estadosTerminalesDeRun = map[string]bool{
	"completed": true, "failed": true, "cancelled": true,
}

func toolAgentRunReport() mcp.Tool {
	return mcp.NewTool("domain_agent_run_report",
		mcp.WithDescription("Registra en agent_runs una ejecucion agentica que corrio EN EL CLIENTE. "+
			"NO ejecuta nada: para ejecutar en el server esta domain_agent_run. Llamar UNA vez, al terminar, "+
			"con el consumo real. Solo acepta estados terminales (completed|failed|cancelled): un run no se "+
			"abre desde el cliente, asi que no puede quedar colgado. Sin model ni tokens el registro no sirve "+
			"para costo, por eso son requeridos; si no tenes el dato real NO inventes numeros, no reportar es "+
			"mejor que envenenar la telemetria."),
		mcp.WithString("project_slug", mcp.Description("Slug del project al que se le imputa el costo. Debe existir: no se auto-crea"), mcp.Required()),
		mcp.WithString("agent_slug", mcp.Description("Slug del agent que se ejecuto (domain_agent_list)"), mcp.Required()),
		mcp.WithString("status", mcp.Description("completed | failed | cancelled"), mcp.Required()),
		mcp.WithString("model", mcp.Description("Modelo realmente usado, ej. claude-opus-5. Puede diferir del de agents.model"), mcp.Required()),
		mcp.WithNumber("tokens_input", mcp.Description("Tokens de entrada consumidos"), mcp.Required()),
		mcp.WithNumber("tokens_output", mcp.Description("Tokens de salida consumidos"), mcp.Required()),
		mcp.WithNumber("cost_usd", mcp.Description("Costo en USD si lo conoces (default 0)")),
		mcp.WithString("output", mcp.Description("Resultado final, texto")),
		mcp.WithString("error", mcp.Description("Motivo del fallo si status != completed")),
	)
}

type reporteDeRun struct {
	agentID   uuid.UUID
	projectID uuid.UUID
	userID    *uuid.UUID
	status    string
	model     string
	tokensIn  int64
	tokensOut int64
	costUSD   float64
	output    string
	errorMsg  string
}

func (d *Deps) handleAgentRunReport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil || d.Projects == nil || d.Pool == nil {
		return mcp.NewToolResultError("agent run report no configurado (Agents/Projects/Pool)"), nil
	}
	rep, msg := d.parseReporteDeRun(ctx, req.GetArguments())
	if msg != "" {
		return mcp.NewToolResultError(msg), nil
	}
	runID, err := insertarRunReportado(ctx, d, *rep)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("no se pudo registrar el run: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"run_id": runID.String(), "status": rep.status, "source": "client",
	})
}

// parseReporteDeRun valida el reporte completo antes de escribir: un reporte
// inválido no deja telemetría a medias. Devuelve el motivo como string porque el
// contrato MCP es un error de tool, no un error de transporte.
func (d *Deps) parseReporteDeRun(ctx context.Context, args map[string]any) (*reporteDeRun, string) {
	status, _ := args["status"].(string)
	if !estadosTerminalesDeRun[status] {
		return nil, "status debe ser completed | failed | cancelled: un run no terminal no se reporta, se ejecuta"
	}
	rep := reporteDeRun{
		status:    status,
		model:     strOf(args["model"]),
		tokensIn:  int64(numOf(args["tokens_input"])),
		tokensOut: int64(numOf(args["tokens_output"])),
		costUSD:   numOf(args["cost_usd"]),
		output:    strOf(args["output"]),
		errorMsg:  strOf(args["error"]),
	}
	if rep.model == "" {
		return nil, "model requerido: sin modelo el registro no sirve para costo"
	}
	if rep.tokensIn < 0 || rep.tokensOut < 0 || rep.costUSD < 0 {
		return nil, "tokens_input, tokens_output y cost_usd no pueden ser negativos"
	}

	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, strOf(args["project_slug"]))
	if err != nil {
		return nil, fmt.Sprintf("project '%s' no existe: la telemetria de costo no se imputa a un project inventado", strOf(args["project_slug"]))
	}
	ag, err := d.Agents.GetBySlug(ctx, orgID, strOf(args["agent_slug"]))
	if err != nil {
		return nil, fmt.Sprintf("agent '%s' no existe", strOf(args["agent_slug"]))
	}
	rep.projectID, rep.agentID = proj.ID, ag.ID
	if uid, err := uuid.Parse(d.Principal.UserID); err == nil {
		rep.userID = &uid
	}
	return &rep, ""
}

// numOf hace un type-assert seguro a float64: los numeros de JSON llegan asi.
// A diferencia de intOf, preserva los decimales de cost_usd.
func numOf(v any) float64 {
	f, _ := v.(float64)
	return f
}

// insertarRunReportado escribe la fila ya terminal. metadata.standalone=true es
// lo que evita que el cron orphan-runs-audit lo cuente como bypass del
// enforcement del orquestador: un run del cliente sin flow_run_id ES standalone
// declarado, no un run que se escapó.
func insertarRunReportado(ctx context.Context, d *Deps, rep reporteDeRun) (uuid.UUID, error) {
	outputs, _ := json.Marshal(map[string]any{"text": rep.output})
	var errPtr *string
	if rep.errorMsg != "" {
		errPtr = &rep.errorMsg
	}
	var runID uuid.UUID
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs
		   (agent_id, user_id, project_id, source, status, model, inputs, outputs, error,
		    tokens_input, tokens_output, cost_usd, metadata, started_at, finished_at)
		 VALUES ($1, $2, $3, 'client', $4, $5, '{}'::jsonb, $6, $7, $8, $9, $10,
		         '{"standalone":true,"reason":"client_report"}'::jsonb, NOW(), NOW())
		 RETURNING id`,
		rep.agentID, rep.userID, rep.projectID, rep.status, rep.model, outputs, errPtr,
		rep.tokensIn, rep.tokensOut, rep.costUSD,
	).Scan(&runID)
	return runID, err
}
