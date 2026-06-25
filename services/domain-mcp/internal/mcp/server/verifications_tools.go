// REQ-50 — verify checkpoints post-cambios.
//
// Flujo:
//  1. Tras un cambio de código no trivial, el LLM llama domain_verify_start
//     con kind + lista de items (build/test/lint/etc). El server crea
//     un checkpoint con status=running.
//  2. El LLM ejecuta cada item con sus tools nativas (Bash, Read) y
//     reporta cada resultado con domain_verify_update_item.
//  3. Al terminar, domain_verify_complete cierra el checkpoint con
//     status final (passed/failed/partial).
//  4. domain_verify_pending lista checkpoints abiertos del proyecto
//     — útil al re-abrir sesión, ver qué quedó sin verificar.
//
// El server NO ejecuta nada. Solo persiste resultados estructurados
// para audit y para que un próximo LLM pueda ver "el último cambio
// dejó tests fallando, no avanzar sin arreglar primero".
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
	projsvc "nunezlagos/domain/internal/service/project"
)

type verificationsProjectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type verificationsHandlers struct {
	projects  verificationsProjectGetter
	pool      *pgxpool.Pool
	principal *apikey.Principal
}

func (h *verificationsHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerVerificationsTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &verificationsHandlers{
		projects:  deps.Projects,
		pool:      deps.Pool,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolVerifyStart(), Handler: wrap.Wrap("domain_verify_start", rls(h.handleVerifyStart))},
		{Tool: toolVerifyUpdateItem(), Handler: wrap.Wrap("domain_verify_update_item", rls(h.handleVerifyUpdateItem))},
		{Tool: toolVerifyComplete(), Handler: wrap.Wrap("domain_verify_complete", rls(h.handleVerifyComplete))},
		{Tool: toolVerifyPending(), Handler: wrap.Wrap("domain_verify_pending", rls(h.handleVerifyPending))},
	}
}

func toolVerifyStart() mcp.Tool {
	return mcp.NewTool("domain_verify_start",
		mcp.WithDescription("Abre un checkpoint de verificación post-cambio. Llamar DESPUÉS de un edit no trivial (no para typos), antes de declarar 'listo'. items[] = lista de checks individuales que vas a correr (build, test, lint, smoke, typecheck, migration). Status del item arranca en 'pending' y vos lo updateás con domain_verify_update_item."),
		mcp.WithString("project_slug", mcp.Description("Proyecto en el que estás trabajando"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("build | test | lint | smoke | typecheck | migration | custom"), mcp.Required()),
		mcp.WithString("context", mcp.Description("Qué cambio gatilló esta verificación (1 línea, ej: 'agregué endpoint POST /api/v1/clients').")),
		mcp.WithArray("items", mcp.Description("Array de items {label, command?}. label es obligatorio, command es informativo. Ej: [{label: 'go test ./internal/...', command: 'go test ./...'}, {label: 'go vet', command: 'go vet ./...'}]"), mcp.Required()),
	)
}

func toolVerifyUpdateItem() mcp.Tool {
	return mcp.NewTool("domain_verify_update_item",
		mcp.WithDescription("Reporta el resultado de UN item de un verify checkpoint. Llamar tras correr el comando con tu tool nativa Bash. status: pass | fail | skipped."),
		mcp.WithString("verification_id", mcp.Description("UUID del checkpoint"), mcp.Required()),
		mcp.WithString("label", mcp.Description("Label exacto del item (el que pasaste en verify_start)"), mcp.Required()),
		mcp.WithString("status", mcp.Description("pass | fail | skipped"), mcp.Required()),
		mcp.WithString("output", mcp.Description("Output relevante (último ~500 chars, no todo el log)")),
		mcp.WithNumber("duration_ms", mcp.Description("Tiempo de ejecución en ms (opcional)")),
	)
}

func toolVerifyComplete() mcp.Tool {
	return mcp.NewTool("domain_verify_complete",
		mcp.WithDescription("Cierra el verify checkpoint. Server calcula status final automáticamente: si todos los items son 'pass' → passed; si algún item 'fail' → failed; si mezcla pass + skipped → partial."),
		mcp.WithString("verification_id", mcp.Description("UUID del checkpoint"), mcp.Required()),
	)
}

func toolVerifyPending() mcp.Tool {
	return mcp.NewTool("domain_verify_pending",
		mcp.WithDescription("Lista checkpoints pendientes o fallados de un proyecto. Llamar al inicio de sesión para ver si quedaron tests fallando del último cambio — no avanzar con feature nuevo sin atender esto."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a consultar"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Default 10")),
	)
}

func (h *verificationsHandlers) handleVerifyStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	kind := strings.ToLower(strings.TrimSpace(asString(args["kind"])))
	if projSlug == "" || kind == "" {
		return mcp.NewToolResultError("project_slug y kind requeridos"), nil
	}
	rawItems, _ := args["items"].([]any)
	if len(rawItems) == 0 {
		return mcp.NewToolResultError("items debe ser un array no vacío"), nil
	}

	// Normalizar items: cada uno debe tener label, status arranca pending.
	items := make([]map[string]any, 0, len(rawItems))
	for _, ri := range rawItems {
		m, ok := ri.(map[string]any)
		if !ok {
			continue
		}
		label, _ := m["label"].(string)
		if label == "" {
			continue
		}
		item := map[string]any{
			"label":   label,
			"status":  "pending",
			"command": m["command"],
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return mcp.NewToolResultError("items debe contener al menos uno con 'label'"), nil
	}
	itemsJSON, _ := json.Marshal(items)
	contextStr, _ := args["context"].(string)

	// REQ-42.3: columna session_id dropeada de verifications (FK a sessions).

	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	var id uuid.UUID
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO tdd_verifications
		   (project_id, user_id,
		    kind, items, status, context)
		 VALUES ($1,$2,$3,$4,'running',NULLIF($5,''))
		 RETURNING id`,
		proj.ID, userID, kind, itemsJSON, contextStr,
	).Scan(&id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verify start failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":     id.String(),
		"kind":   kind,
		"items":  items,
		"status": "running",
	})
}

func (h *verificationsHandlers) handleVerifyUpdateItem(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["verification_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("verification_id inválido"), nil
	}
	label, _ := args["label"].(string)
	status := strings.ToLower(strings.TrimSpace(asString(args["status"])))
	if label == "" || (status != "pass" && status != "fail" && status != "skipped") {
		return mcp.NewToolResultError("label y status (pass|fail|skipped) requeridos"), nil
	}
	output, _ := args["output"].(string)
	if len(output) > 2000 {
		output = output[:2000] + "...[truncated]"
	}
	durationMs := 0
	if v, ok := args["duration_ms"].(float64); ok {
		durationMs = int(v)
	}

	// Leer items actuales, actualizar el matching label, persistir.
	var itemsRaw []byte
	if err := h.q(ctx).QueryRow(ctx,
		`SELECT items FROM tdd_verifications WHERE id = $1`,
		id,
	).Scan(&itemsRaw); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verification not found: %v", err)), nil
	}
	var items []map[string]any
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return mcp.NewToolResultError("items corruptos"), nil
	}
	found := false
	for i := range items {
		if items[i]["label"] == label {
			items[i]["status"] = status
			if output != "" {
				items[i]["output"] = output
			}
			if durationMs > 0 {
				items[i]["duration_ms"] = durationMs
			}
			found = true
			break
		}
	}
	if !found {
		return mcp.NewToolResultError("label no encontrado en items del checkpoint"), nil
	}
	newRaw, _ := json.Marshal(items)
	if _, err := h.q(ctx).Exec(ctx,
		`UPDATE tdd_verifications SET items = $2 WHERE id = $1`,
		id, newRaw,
	); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"verification_id": id.String(),
		"label":           label,
		"status":          status,
	})
}

func (h *verificationsHandlers) handleVerifyComplete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["verification_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("verification_id inválido"), nil
	}

	var itemsRaw []byte
	if err := h.q(ctx).QueryRow(ctx,
		`SELECT items FROM tdd_verifications WHERE id = $1`,
		id,
	).Scan(&itemsRaw); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verification not found: %v", err)), nil
	}
	var items []map[string]any
	_ = json.Unmarshal(itemsRaw, &items)

	// Calcular status final.
	pass, fail, skipped, pending := 0, 0, 0, 0
	for _, it := range items {
		switch it["status"] {
		case "pass":
			pass++
		case "fail":
			fail++
		case "skipped":
			skipped++
		default:
			pending++
		}
	}
	var finalStatus string
	switch {
	case fail > 0:
		finalStatus = "failed"
	case pending > 0:
		finalStatus = "partial" // hay items sin reportar — el LLM cerró sin completar
	case pass > 0 && skipped == 0:
		finalStatus = "passed"
	case skipped > 0:
		finalStatus = "partial"
	default:
		finalStatus = "passed"
	}

	if _, err := h.q(ctx).Exec(ctx,
		`UPDATE tdd_verifications SET status = $2, completed_at = NOW()
		   WHERE id = $1`,
		id, finalStatus,
	); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("complete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"verification_id": id.String(),
		"status":          finalStatus,
		"counts": map[string]any{
			"pass": pass, "fail": fail, "skipped": skipped, "pending": pending,
		},
	})
}

func (h *verificationsHandlers) handleVerifyPending(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	if projSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 && v <= 50 {
		limit = int(v)
	}

	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	rows, err := h.q(ctx).Query(ctx,
		`SELECT id::text, kind, status, COALESCE(context,''),
		        to_char(started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM tdd_verifications
		   WHERE project_id = $1
		     AND status IN ('pending','running','failed','partial')
		   ORDER BY started_at DESC LIMIT $2`,
		proj.ID, limit,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("pending list failed: %v", err)), nil
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, status, contextStr, ts string
		if err := rows.Scan(&id, &kind, &status, &contextStr, &ts); err == nil {
			out = append(out, map[string]any{
				"id": id, "kind": kind, "status": status,
				"context": contextStr, "started_at": ts,
			})
		}
	}
	return toolResultJSON(map[string]any{
		"verifications": out,
		"total":         len(out),
	})
}

// silenciar context si no se usa
var _ context.Context
