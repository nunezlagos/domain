package mcpserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/observability"
)

// errInvalidFingerprint se devuelve cuando el param fingerprint no es hex valido.
var errInvalidFingerprint = errors.New("invalid fingerprint: must be hex-encoded sha256")

// errorReportingHandlers expone los tools de gestion de known_errors y el
// reset de dedup de error_events (issue-53.9). Lee/escribe via deps.Pool
// directo (las tablas no tienen org-scoping, igual que workflows).
type errorReportingHandlers struct {
	pool *pgxpool.Pool
}

// NewErrorReportingHandlers construye el handler con el pool. Si es nil,
// los tools devuelven error explicito al invocarse.
func NewErrorReportingHandlers(pool *pgxpool.Pool) *errorReportingHandlers {
	return &errorReportingHandlers{pool: pool}
}

// validHealActions limita auto_heal_action a las acciones soportadas.
var validHealActions = map[string]bool{
	observability.HealRetry:         true,
	observability.HealClearCache:    true,
	observability.HealRestartWorker: true,
	observability.HealNone:          true,
}

func toolKnownErrorSet() mcp.Tool {
	return mcp.NewTool("domain_known_error_set",
		mcp.WithDescription("Registra o actualiza un known_error por fingerprint (hex visible en error_events) con su remediacion y accion de auto-heal."),
		mcp.WithString("fingerprint",
			mcp.Description("Fingerprint sha256 en hex (columna fingerprint de error_events)"),
			mcp.Required(),
		),
		mcp.WithString("name", mcp.Description("Nombre corto del error conocido"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Descripcion opcional")),
		mcp.WithString("remediation", mcp.Description("Remediacion documentada (texto libre)")),
		mcp.WithBoolean("recoverable", mcp.Description("Si el error es recuperable (default true)")),
		mcp.WithString("auto_heal_action", mcp.Description("retry|clear_cache|restart_worker|none (default none)")),
		mcp.WithObject("action_params", mcp.Description("Parametros opcionales de la accion (jsonb)")),
	)
}

func toolErrorReset() mcp.Tool {
	return mcp.NewTool("domain_error_reset",
		mcp.WithDescription("Borra el error_event de un fingerprint para reiniciar el dedup_count (el proximo evento crea una fila nueva)."),
		mcp.WithString("fingerprint",
			mcp.Description("Fingerprint sha256 en hex a resetear"),
			mcp.Required(),
		),
	)
}

func (h *errorReportingHandlers) handleKnownErrorSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.pool == nil {
		return mcp.NewToolResultError("error reporting store not configured"), nil
	}
	fp, err := decodeFingerprint(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	recoverable := true
	if v, ok := args["recoverable"].(bool); ok {
		recoverable = v
	}
	action := req.GetString("auto_heal_action", observability.HealNone)
	if !validHealActions[action] {
		return mcp.NewToolResultError("invalid auto_heal_action: " + action), nil
	}
	var params []byte
	if m, ok := args["action_params"].(map[string]any); ok && len(m) > 0 {
		params, _ = json.Marshal(m)
	}
	_, err = h.pool.Exec(ctx, `
		INSERT INTO known_errors
			(fingerprint, name, description, remediation, recoverable, auto_heal_action, action_params)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7)
		ON CONFLICT (fingerprint) DO UPDATE
		SET name = EXCLUDED.name, description = EXCLUDED.description,
		    remediation = EXCLUDED.remediation, recoverable = EXCLUDED.recoverable,
		    auto_heal_action = EXCLUDED.auto_heal_action, action_params = EXCLUDED.action_params
	`, fp, name, req.GetString("description", ""), req.GetString("remediation", ""), recoverable, action, params)
	if err != nil {
		return mcp.NewToolResultError("set known_error: " + err.Error()), nil
	}
	return toolResultJSON(map[string]any{"fingerprint": hex.EncodeToString(fp), "name": name, "set": true})
}

func (h *errorReportingHandlers) handleErrorReset(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.pool == nil {
		return mcp.NewToolResultError("error reporting store not configured"), nil
	}
	fp, err := decodeFingerprint(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	tag, err := h.pool.Exec(ctx, `DELETE FROM error_events WHERE fingerprint = $1`, fp)
	if err != nil {
		return mcp.NewToolResultError("reset: " + err.Error()), nil
	}
	return toolResultJSON(map[string]any{"fingerprint": hex.EncodeToString(fp), "deleted": tag.RowsAffected()})
}

// decodeFingerprint lee el param "fingerprint" (hex) y lo decodifica a bytes.
func decodeFingerprint(req mcp.CallToolRequest) ([]byte, error) {
	fpHex, err := req.RequireString("fingerprint")
	if err != nil {
		return nil, err
	}
	fp, derr := hex.DecodeString(fpHex)
	if derr != nil || len(fp) == 0 {
		return nil, errInvalidFingerprint
	}
	return fp, nil
}

func registerErrorReportingTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := NewErrorReportingHandlers(deps.Pool)
	return []mcpgo.ServerTool{
		{Tool: toolKnownErrorSet(), Handler: wrap.Wrap("domain_known_error_set", h.handleKnownErrorSet)},
		{Tool: toolErrorReset(), Handler: wrap.Wrap("domain_error_reset", h.handleErrorReset)},
	}
}
