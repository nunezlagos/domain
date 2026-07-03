package mcpserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/observability"
)

// errInvalidFingerprint se devuelve cuando el param fingerprint no es hex valido.
var errInvalidFingerprint = errors.New("invalid fingerprint: must be hex-encoded sha256")

// errorReportingHandlers expone los tools de gestion de known_errors y el
// reset de dedup de error_events (issue-53.9). Lee/escribe via deps.Pool
// directo (las tablas no tienen org-scoping, igual que workflows).
type errorReportingHandlers struct {
	pool      *pgxpool.Pool
	principal *apikey.Principal
}

// NewErrorReportingHandlers construye el handler con el pool y el principal de la
// sesion (para el audit trail, REQ-56 issue-56.2). Si el pool es nil, los tools
// devuelven error explicito al invocarse. El principal puede ser nil (sesion sin
// autenticar): en ese caso el audit queda con actor_id NULL.
func NewErrorReportingHandlers(pool *pgxpool.Pool, principal *apikey.Principal) *errorReportingHandlers {
	return &errorReportingHandlers{pool: pool, principal: principal}
}

// actorID devuelve el UUID del principal de la sesion, o uuid.Nil si no hay
// principal o el UserID no parsea. Se usa para poblar actor_id en el audit.
func (h *errorReportingHandlers) actorID() uuid.UUID {
	if h.principal == nil {
		return uuid.Nil
	}
	id, err := uuid.Parse(h.principal.UserID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// logDecision escribe una entrada append-only en error_decision_log. Best-effort:
// si falla, NO revierte la decision principal (ya aplicada) — solo se pierde el
// rastro, que es preferible a fallar la operacion del operador.
func (h *errorReportingHandlers) logDecision(ctx context.Context, fp []byte, action, reason string, detail []byte) {
	actor := h.actorID()
	var actorArg any
	if actor != uuid.Nil {
		actorArg = actor
	}
	_, _ = h.pool.Exec(ctx, `
		INSERT INTO error_decision_log (fingerprint, action, actor_id, reason, detail)
		VALUES ($1,$2,$3,NULLIF($4,''),$5)
	`, fp, action, actorArg, reason, detail)
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
		mcp.WithString("reason", mcp.Description("Razon de la decision (por que se clasifica asi). Queda en el audit trail (error_decision_log).")),
	)
}

func toolErrorReset() mcp.Tool {
	return mcp.NewTool("domain_error_reset",
		mcp.WithDescription("Soft-delete del error_event de un fingerprint para reiniciar el dedup_count (el proximo evento crea una fila nueva). Reversible: marca deleted_at/by/reason, no borra la fila."),
		mcp.WithString("fingerprint",
			mcp.Description("Fingerprint sha256 en hex a resetear"),
			mcp.Required(),
		),
		mcp.WithString("reason", mcp.Description("Razon del reset (por que se descarta el historial). Queda en el audit trail (error_decision_log).")),
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
	// REQ-56 issue-56.2: dejar rastro de la decision (quien/cuando/por que + snapshot).
	detail, _ := json.Marshal(map[string]any{
		"name": name, "recoverable": recoverable, "auto_heal_action": action,
	})
	h.logDecision(ctx, fp, "known_error_set", req.GetString("reason", ""), detail)
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
	reason := req.GetString("reason", "")
	var actorArg any
	if a := h.actorID(); a != uuid.Nil {
		actorArg = a
	}
	// REQ-56 issue-56.2: soft-delete en vez de DELETE duro. Solo afecta filas vivas
	// (deleted_at IS NULL) para que un reset repetido no pise la autoria original.
	tag, err := h.pool.Exec(ctx, `
		UPDATE error_events
		SET deleted_at = now(), deleted_by = $2, deletion_reason = NULLIF($3,'')
		WHERE fingerprint = $1 AND deleted_at IS NULL
	`, fp, actorArg, reason)
	if err != nil {
		return mcp.NewToolResultError("reset: " + err.Error()), nil
	}
	h.logDecision(ctx, fp, "error_reset", reason, nil)
	return toolResultJSON(map[string]any{"fingerprint": hex.EncodeToString(fp), "soft_deleted": tag.RowsAffected()})
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
	h := NewErrorReportingHandlers(deps.Pool, deps.Principal)
	return []mcpgo.ServerTool{
		{Tool: toolKnownErrorSet(), Handler: wrap.Wrap("domain_known_error_set", h.handleKnownErrorSet)},
		{Tool: toolErrorReset(), Handler: wrap.Wrap("domain_error_reset", h.handleErrorReset)},
	}
}
